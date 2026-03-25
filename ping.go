package main

import (
	"net"
	"os"
	"time"
)

// ping sends a single ICMP echo request to addr and returns true if a reply
// is received within the given timeout. If raw ICMP sockets are unavailable
// (e.g. missing root/CAP_NET_RAW), it falls back to a TCP reachability check.
func ping(addr string, timeout time.Duration) bool {
	ip := net.ParseIP(addr)
	if ip == nil {
		ips, err := net.LookupIP(addr)
		if err != nil || len(ips) == 0 {
			return false
		}
		ip = ips[0]
	}

	// Select the correct ICMP protocol for IPv4 vs IPv6.
	network := "ip4:icmp"
	if ip.To4() == nil {
		network = "ip6:ipv6-icmp"
	}

	conn, err := net.DialTimeout(network, ip.String(), timeout)
	if err != nil {
		// Raw sockets not available — fall back to TCP connect probe.
		return pingTCP(addr, timeout)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))

	id := os.Getpid() & 0xffff
	msg := buildEchoRequest(id, 1)

	if _, err := conn.Write(msg); err != nil {
		return false
	}

	buf := make([]byte, 1500)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return false
		}
		if n >= 8 && isEchoReply(buf[:n], id) {
			return true
		}
	}
}

// buildEchoRequest constructs a minimal 8-byte ICMP Echo Request packet
// with the given identifier and sequence number.
func buildEchoRequest(id, seq int) []byte {
	msg := make([]byte, 8)
	msg[0] = 8 // Type: Echo Request
	msg[1] = 0 // Code
	// Bytes 2-3: checksum (filled below)
	msg[4] = byte(id >> 8)
	msg[5] = byte(id)
	msg[6] = byte(seq >> 8)
	msg[7] = byte(seq)

	cs := icmpChecksum(msg)
	msg[2] = byte(cs >> 8)
	msg[3] = byte(cs)

	return msg
}

// icmpChecksum computes the Internet Checksum (RFC 1071) over data.
func icmpChecksum(data []byte) uint16 {
	var sum uint32
	length := len(data)
	for i := 0; i < length-1; i += 2 {
		sum += uint32(data[i])<<8 | uint32(data[i+1])
	}
	if length%2 == 1 {
		sum += uint32(data[length-1]) << 8
	}
	sum = (sum >> 16) + (sum & 0xffff)
	sum += sum >> 16
	return ^uint16(sum)
}

// isEchoReply checks whether buf contains an ICMP Echo Reply (type 0) that
// matches the expected identifier. It handles both raw ICMP payloads and
// responses that include the 20-byte IPv4 header.
func isEchoReply(buf []byte, id int) bool {
	for _, offset := range []int{0, 20} {
		if len(buf) < offset+8 {
			continue
		}
		typ := buf[offset]
		replyID := int(buf[offset+4])<<8 | int(buf[offset+5])
		if typ == 0 && replyID == id {
			return true
		}
	}
	return false
}

// pingTCP is a fallback reachability check that attempts TCP connections to
// common ports. It is used when raw ICMP sockets are not available.
func pingTCP(addr string, timeout time.Duration) bool {
	ports := []string{"80", "443", nasPort()}
	for _, port := range ports {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(addr, port), timeout)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}
