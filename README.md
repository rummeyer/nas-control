# NAS Control

A lightweight HTTP server to remotely control a Synology NAS from your local network.

## Features

- **Wake-on-LAN** — send a magic packet to power on the NAS
- **Shutdown** — graceful shutdown via the Synology DSM API
- **Status check** — ICMP ping (with TCP fallback) to detect online/offline state
- **Web UI** — minimal single-page dashboard
- **Local-only access** — requests from public IPs are rejected

## Quick Start

```bash
go build -o nas-control .
./nas-control
```

The server starts on `0.0.0.0:7654` by default. Open `http://localhost:7654` in your browser.

### Override listen address

```bash
./nas-control 0.0.0.0:8080
```

## DSM User Setup

The shutdown feature requires a dedicated DSM user. **Do not use your main admin account** — create a separate user without 2FA so the API login works.

1. Open **DSM > Control Panel > User & Group**
2. Click **Create** and pick a username (e.g. `nas-control`)
3. Set a strong password
4. Add the user to the **administrators** group (required for shutdown permissions)
5. On the **Permissions** tab deny access to all shared folders — this user only needs API access
6. Complete the wizard and make sure **2-Factor Authentication is not enabled** for this user

Use this user's credentials in your `config.yaml`.

## Configuration

Create a `config.yaml` in the working directory or next to the binary:

```yaml
listen_addr: "0.0.0.0:7654"
nas:
  url: "http://192.168.1.100:5000"
  user: "admin"
  pass: "your-password"
  mac: "00:11:32:CA:B0:95"
```

If no config file is found, built-in defaults are used.

## API Endpoints

| Method | Path     | Description                    |
|--------|----------|--------------------------------|
| GET    | `/`      | Web UI                         |
| GET    | `/info`  | NAS connection info (JSON)     |
| GET    | `/state` | Online/offline status (JSON)   |
| POST   | `/on`    | Send Wake-on-LAN magic packet  |
| POST   | `/off`   | Shut down via Synology DSM API |

All endpoints return JSON:

```json
{"status": "ok", "message": "Wake-on-LAN packet sent to 00:11:32:CA:B0:95"}
```

## Tested With

- Synology DSM 7.2 (latest)

## Running Tests

```bash
go test -v ./...
```

Note: The ICMP ping test requires raw socket privileges (root or `CAP_NET_RAW`). It is automatically skipped when running unprivileged.

## Cross-Compiling

Build for a Raspberry Pi or other Linux ARM device:

```bash
GOOS=linux GOARCH=arm64 go build -o nas-control .
```
