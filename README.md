# Aircast CLI

A command-line tool that allows traditional MAVLink ground control stations (QGroundControl, Mission Planner, MAVProxy) to connect to the Aircast WebSocket MAVLink proxy via TCP or UDP.

## Overview

The Aircast API uses WebSocket for MAVLink communication, but most ground control software expects TCP or UDP connections. This bridge solves that problem by:

1. Connecting to the Aircast API WebSocket endpoint
2. Listening for TCP/UDP connections from MAVLink clients
3. Forwarding MAVLink messages bidirectionally

```
[QGroundControl] ←→ TCP ←→ [aircast-cli] ←→ WebSocket ←→ [Aircast API] ←→ [Device]
```

## Installation

### Build from source

```bash
cd aircast-cli
go build -o aircast-cli ./cmd/bridge
```

### Install globally

```bash
go install github.com/pavliha/aircast/aircast-cli/cmd/bridge@latest
```

## Usage

### First Time - Dead Simple!

Just run with your device ID - authentication happens automatically:

```bash
aircast-cli --device 35f0f949-c3ca-479e-9b9f-f3f168c50244
```

You'll see:
```
Authentication required...

╔═══════════════════════════════════════════════════════════════╗
║                   Aircast Authentication                      ║
╚═══════════════════════════════════════════════════════════════╝

To authenticate aircast-cli, please visit:

  https://dev.aircast.one/activate

And enter this code:

  ABC-DEF

Code expires in 15 minutes.

Waiting for authorization...
```

Visit the URL, enter the code, and you're done! The token is saved to `~/.aircast/token.json` for future use.

### Subsequent Runs

Same command - uses saved token automatically:

```bash
aircast-cli --device 35f0f949-c3ca-479e-9b9f-f3f168c50244
```

### Custom TCP/UDP Ports

```bash
aircast-cli \
  --device 35f0f949-c3ca-479e-9b9f-f3f168c50244 \
  --tcp 127.0.0.1:14551 \
  --udp 127.0.0.1:14552
```

### Local Development

```bash
aircast-cli \
  --device 35f0f949-c3ca-479e-9b9f-f3f168c50244 \
  --api http://localhost:3333
```

### Command Line Options

- `--device <id>` - Device ID to connect to (required)
- `--api <url>` - API base URL (default: https://api.dev.aircast.one)
- `--tcp <address>` - TCP listen address (default: 127.0.0.1:14550)
- `--udp <address>` - UDP listen address (optional)
- `--login` - Force re-authentication (clear stored token)
- `--logout` - Clear stored authentication token
- `--log-level <level>` - Log level: trace, debug, info, warn, error (default: info)
- `--version` - Show version information

### Managing Authentication

```bash
# Force re-authentication
aircast-cli --device YOUR_DEVICE_ID --login

# Logout (clear token)
aircast-cli --logout

# Token location
~/.aircast/token.json
```

## Authentication

Authentication uses **OAuth2 Device Code Flow** (RFC 8628) - the same flow used by:
```bash
# Login to get token
curl -X POST https://api.dev.aircast.one/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"your@email.com","password":"yourpassword"}'

# Response will contain the token
{"token":"eyJhbGc..."}
```

## Connecting Ground Control Software

### QGroundControl

1. Start the bridge:
   ```bash
   aircast-cli --device YOUR_DEVICE_ID
   ```

2. In QGroundControl:
   - Go to **Application Settings** → **Comm Links**
   - Add new connection
   - Type: **TCP**
   - Server Address: **127.0.0.1**
   - Port: **14550**
   - Click **OK** and **Connect**

### Mission Planner

1. Start the bridge:
   ```bash
   aircast-cli --device YOUR_DEVICE_ID
   ```

2. In Mission Planner:
   - Top right dropdown: Select **TCP**
   - Click **Connect**
   - Enter **127.0.0.1:14550**
   - Click **OK**

### MAVProxy

```bash
# Start the bridge
aircast-cli --device YOUR_DEVICE_ID

# In another terminal, start MAVProxy
mavproxy.py --master=tcp:127.0.0.1:14550
```

## Features

- ✅ TCP and UDP support
- ✅ Multiple simultaneous TCP clients
- ✅ Multiple simultaneous UDP clients
- ✅ Automatic WebSocket reconnection
- ✅ Binary MAVLink message forwarding
- ✅ Authentication with Bearer token
- ✅ Configurable logging

## Architecture

The bridge operates as a bidirectional proxy:

**From Ground Station → Device:**
1. Ground station sends MAVLink message to TCP/UDP
2. Bridge reads from TCP/UDP socket
3. Bridge forwards message to WebSocket (binary)
4. Aircast API routes message to device

**From Device → Ground Station:**
1. Device sends MAVLink message through Aircast API
2. Aircast API sends message via WebSocket (binary)
3. Bridge reads from WebSocket
4. Bridge forwards to all connected TCP/UDP clients

## Troubleshooting

### Cannot connect to WebSocket

```
Failed to connect to WebSocket: websocket: bad handshake
```

**Solution**: Check your authentication token is valid and not expired.

### TCP port already in use

```
Failed to start TCP listener: listen tcp 127.0.0.1:14550: bind: address already in use
```

**Solution**: Another application is using port 14550. Either:
- Stop the other application
- Use a different port: `--tcp 127.0.0.1:14551`

### No data received in ground station

**Check**:
1. Bridge is connected: Look for "WebSocket connected" in logs
2. Ground station is connected: Look for "TCP client connected" in logs
3. MAVLink proxy is running on the API side
4. Device is sending MAVLink data

**Enable debug logging**:
```bash
mavlink-bridge --device YOUR_DEVICE_ID --token YOUR_TOKEN --log-level debug
```

## Development

### Running tests

```bash
go test ./...
```

### Building for different platforms

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o mavlink-bridge-linux ./cmd/bridge

# Windows
GOOS=windows GOARCH=amd64 go build -o mavlink-bridge.exe ./cmd/bridge

# macOS (ARM)
GOOS=darwin GOARCH=arm64 go build -o mavlink-bridge-macos-arm ./cmd/bridge
```

## License

Same as the main Aircast project.
