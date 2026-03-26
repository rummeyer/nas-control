# NAS Control

A lightweight HTTP server to remotely control a Synology NAS from your local network. Runs as a background daemon and provides a minimal web UI plus a JSON API for wake, shutdown, and status checks.

## Features

- **Wake-on-LAN** — send a magic packet to power on the NAS
- **Shutdown** — graceful shutdown via the Synology DSM API
- **Status check** — ICMP ping (with TCP fallback) to detect online/offline state
- **Web UI** — minimal single-page dashboard
- **Daemon mode** — runs in the background with start/stop commands
- **Local-only access** — requests from public IPs are rejected

## Quick Start

```bash
go build -o nas-control .
./nas-control start
```

This starts the server as a background daemon on `0.0.0.0:7654` (default). Open `http://<host>:7654` in your browser.

To stop the server:

```bash
./nas-control stop
```

Log output is written to `nas-control.log` next to the binary.

### Usage

```
nas-control [start|stop]
```

| Command | Description |
|---------|-------------|
| `start` | Start the server as a background daemon (default if no argument given) |
| `stop`  | Stop a running daemon |

## Configuration

Create a `config.yaml` in the working directory or next to the binary:

```yaml
listen_addr: "0.0.0.0:7654"
nas:
  url: "http://192.168.1.100:5000"
  user: "nas-control"
  pass: "your-password"
  mac: "00:11:32:CA:B0:95"
```

| Field | Description |
|-------|-------------|
| `listen_addr` | Address and port the server listens on |
| `nas.url` | Base URL of the Synology DSM web interface |
| `nas.user` | DSM user for API access (see below) |
| `nas.pass` | Password for the DSM user |
| `nas.mac` | MAC address of the NAS (for Wake-on-LAN) |

If no config file is found, built-in defaults are used.

## DSM User Setup

The shutdown feature requires a dedicated DSM user. **Do not use your main admin account** — create a separate user without 2FA so the API login works.

1. Open **DSM > Control Panel > User & Group**
2. Click **Create** and pick a username (e.g. `nas-control`)
3. Set a strong password
4. Add the user to the **administrators** group (required for shutdown permissions)
5. On the **Permissions** tab deny access to all shared folders — this user only needs API access
6. Complete the wizard and make sure **2-Factor Authentication is not enabled** for this user

Use this user's credentials in your `config.yaml`.

## API

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

## Running Tests

```bash
go test -v ./...
```

The ICMP ping test requires raw socket privileges (root or `CAP_NET_RAW`) and is automatically skipped when running unprivileged.

## Tested With

- Synology DSM 7.2
