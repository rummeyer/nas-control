// Package main implements a lightweight HTTP server to control a Synology NAS.
// It provides endpoints to wake (WoL), shut down (via Synology API), and
// check the online state of the NAS. Access is restricted to private networks.
package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// NASConfig holds connection details for the target Synology NAS.
type NASConfig struct {
	URL  string `yaml:"url"`
	User string `yaml:"user"`
	Pass string `yaml:"pass"`
	MAC  string `yaml:"mac"`
}

// nasHost returns the hostname (without port) from the configured NAS URL.
func nasHost() string {
	u, err := url.Parse(config.NAS.URL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// nasPort returns the port from the configured NAS URL, or the default port
// for the scheme if none is specified.
func nasPort() string {
	u, err := url.Parse(config.NAS.URL)
	if err != nil {
		return ""
	}
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

// Config is the top-level application configuration.
type Config struct {
	ListenAddr string    `yaml:"listen_addr"`
	NAS        NASConfig `yaml:"nas"`
}

var config Config

// loadConfig populates the global config. It searches for config.yaml first
// in the current working directory, then next to the executable. If no file
// is found, built-in defaults are used.
func loadConfig() {
	config = Config{
		ListenAddr: "0.0.0.0:7654",
	}

	paths := []string{"config.yaml"}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "config.yaml"))
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(data, &config); err != nil {
			log.Fatalf("Failed to parse %s: %v", p, err)
		}
		log.Printf("Config loaded from %s", p)
		return
	}
	log.Println("No config.yaml found, using defaults")
}

// Response is the standard JSON envelope returned by all API endpoints.
type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// writeJSON serialises resp as JSON and writes it with the given HTTP status code.
func writeJSON(w http.ResponseWriter, statusCode int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

// handleOn sends a Wake-on-LAN magic packet to power on the NAS.
// The magic packet is broadcast via UDP port 9 and consists of 6 bytes 0xFF
// followed by the target MAC address repeated 16 times.
func handleOn(w http.ResponseWriter, r *http.Request) {
	mac, err := net.ParseMAC(config.NAS.MAC)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{"error", "Invalid MAC address: " + err.Error()})
		return
	}

	packet := make([]byte, 0, 102)
	for i := 0; i < 6; i++ {
		packet = append(packet, 0xFF)
	}
	for i := 0; i < 16; i++ {
		packet = append(packet, mac...)
	}

	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{
		IP:   net.IPv4bcast,
		Port: 9,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{"error", "UDP connection failed: " + err.Error()})
		return
	}
	defer conn.Close()

	if _, err = conn.Write(packet); err != nil {
		writeJSON(w, http.StatusInternalServerError, Response{"error", "Failed to send magic packet: " + err.Error()})
		return
	}

	log.Println("Wake-on-LAN packet sent to", config.NAS.MAC)
	writeJSON(w, http.StatusOK, Response{"ok", "Wake-on-LAN packet sent to " + config.NAS.MAC})
}

// handleOff performs a graceful shutdown of the NAS by authenticating against
// the Synology DSM API (SYNO.API.Auth) and then calling SYNO.Core.System shutdown.
func handleOff(w http.ResponseWriter, r *http.Request) {
	baseURL := strings.TrimRight(config.NAS.URL, "/")
	client := &http.Client{Timeout: 30 * time.Second}

	// Authenticate to obtain a session ID.
	loginData := url.Values{
		"api":     {"SYNO.API.Auth"},
		"method":  {"login"},
		"version": {"6"},
		"account": {config.NAS.User},
		"passwd":  {config.NAS.Pass},
		"format":  {"sid"},
	}

	loginResp, err := client.PostForm(baseURL+"/webapi/auth.cgi", loginData)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, Response{"error", "Login request failed: " + err.Error()})
		return
	}
	defer loginResp.Body.Close()

	var loginResult struct {
		Data struct {
			SID string `json:"sid"`
		} `json:"data"`
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginResult); err != nil {
		writeJSON(w, http.StatusBadGateway, Response{"error", "Failed to parse login response: " + err.Error()})
		return
	}
	if !loginResult.Success || loginResult.Data.SID == "" {
		writeJSON(w, http.StatusBadGateway, Response{"error", "Login failed, no SID received"})
		return
	}

	// Issue the shutdown command using the obtained session ID.
	shutdownData := url.Values{
		"api":     {"SYNO.Core.System"},
		"method":  {"shutdown"},
		"version": {"1"},
		"_sid":    {loginResult.Data.SID},
	}

	shutdownResp, err := client.PostForm(baseURL+"/webapi/entry.cgi", shutdownData)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, Response{"error", "Shutdown request failed: " + err.Error()})
		return
	}
	defer shutdownResp.Body.Close()

	var shutdownResult struct {
		Success bool `json:"success"`
	}
	json.NewDecoder(shutdownResp.Body).Decode(&shutdownResult)

	if shutdownResult.Success {
		log.Println("NAS shutdown triggered")
		writeJSON(w, http.StatusOK, Response{"ok", "NAS shutdown triggered"})
	} else {
		writeJSON(w, http.StatusBadGateway, Response{"error", "Shutdown command was not successful"})
	}
}

// handleState checks whether the NAS is reachable by pinging it.
func handleState(w http.ResponseWriter, r *http.Request) {
	online := ping(nasHost(), 2*time.Second)

	msg := "offline"
	if online {
		msg = "online"
	}

	log.Printf("NAS state check: %s", msg)
	writeJSON(w, http.StatusOK, Response{"ok", msg})
}

// handleIndex serves the web UI. It looks for index.html in the current
// working directory first, then next to the executable.
func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	paths := []string{"index.html"}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "index.html"))
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
		return
	}
	http.Error(w, "index.html not found", http.StatusInternalServerError)
}

// handleInfo returns basic NAS connection info as JSON.
func handleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"nas_ip": nasHost()})
}

// privateRanges contains the RFC 1918 / RFC 4193 CIDR blocks used to
// identify requests originating from private networks.
var privateRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"::1/128",
		"fd00::/8",
	} {
		_, network, _ := net.ParseCIDR(cidr)
		privateRanges = append(privateRanges, network)
	}
}

// isPrivateIP reports whether addr (host:port or bare IP) belongs to a
// private/loopback network range.
func isPrivateIP(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, network := range privateRanges {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// localOnly is HTTP middleware that rejects requests from non-private IPs.
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isPrivateIP(r.RemoteAddr) {
			log.Printf("Blocked request from %s", r.RemoteAddr)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	loadConfig()

	// Allow overriding the listen address via command line argument.
	if len(os.Args) > 1 {
		config.ListenAddr = os.Args[1]
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/info", handleInfo)
	mux.HandleFunc("/on", handleOn)
	mux.HandleFunc("/off", handleOff)
	mux.HandleFunc("/state", handleState)

	log.Printf("NAS Control Server starting on %s", config.ListenAddr)
	if err := http.ListenAndServe(config.ListenAddr, localOnly(mux)); err != nil {
		log.Fatal(err)
	}
}
