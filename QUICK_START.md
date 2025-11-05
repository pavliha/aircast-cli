# Aircast CLI - Quick Start

The simplest way to connect QGroundControl, Mission Planner, or MAVProxy to your Aircast device.

## Install

```bash
cd aircast-cli
make build
```

## First Run

```bash
./aircast-cli --device YOUR_DEVICE_ID
```

That's it! You'll be prompted to authenticate once. After that, just run the same command.

## What Happens

1. **First time**: Opens browser for OAuth2 login, saves token to `~/.aircast/token.json`
2. **Subsequent runs**: Uses saved token automatically
3. **Token expires**: Prompts to re-authenticate

## Connect Ground Station

The CLI creates a TCP server on `127.0.0.1:14550` by default.

**QGroundControl:**
- Settings → Comm Links → Add
- Type: TCP, Host: 127.0.0.1, Port: 14550

**Mission Planner:**
- Top dropdown: TCP → Connect → 127.0.0.1:14550

**MAVProxy:**
```bash
mavproxy.py --master=tcp:127.0.0.1:14550
```

## Options

```bash
# Custom port
./aircast-cli --device YOUR_ID --tcp 127.0.0.1:14551

# Local development
./aircast-cli --device YOUR_ID --api http://localhost:3333

# Force re-login
./aircast-cli --device YOUR_ID --login

# Logout
./aircast-cli --logout
```

## Troubleshooting

**"Authentication failed"**
- Check your internet connection
- Make sure you're completing the OAuth flow in the browser

**"Failed to connect to WebSocket"**
- Verify device ID is correct
- Check API URL (use `--api` for local dev)
- Ensure you have access to the device

**"Port already in use"**
- Another app is using port 14550
- Use `--tcp 127.0.0.1:14551` to use a different port

## Architecture

```
[QGC/Mission Planner] ←→ TCP ←→ [aircast-cli] ←→ WebSocket ←→ [Aircast API] ←→ [Device]
                      14550                      OAuth2 Auth
```

## Security

- Token stored at `~/.aircast/token.json` with 0600 permissions (user read/write only)
- Uses OAuth2 Device Code Flow (RFC 8628)
- No passwords or API keys in CLI
- Tokens expire after 24 hours
