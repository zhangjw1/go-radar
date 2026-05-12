# Go Radar

Standalone Go version of Web3 Online Radar.

The whole `go-radar/` directory is designed to be movable. It can run inside the
original Python repository, or be copied to another directory and started there
without depending on Python files.

## Run

From this directory:

```powershell
Copy-Item .env.example .env
$env:GOTELEMETRY = "off"
npm install
go run ./cmd/radar
```

`S5` uses `gmgn-cli`. For this standalone project, install it locally into this
directory so the service does not depend on any other repository:

```powershell
npm install
$env:GOTELEMETRY = "off"
go run ./cmd/radar
```

The service listens on `:8080` by default. Override it with `GO_RADAR_PORT` in
`.env` or your shell.

```powershell
$env:GO_RADAR_PORT = "8081"
go run ./cmd/radar
```

The Go scheduler is disabled by default. Enable it only when you want the Go
service to run scanners and send pushes.

```powershell
$env:GO_RADAR_ENABLE_SCHEDULER = "true"
go run ./cmd/radar
```

## Configuration

- `.env.example` contains the standalone configuration template.
- `.env` is optional. If it does not exist, Go Radar starts with safe defaults.
- `GO_RADAR_ENV_FILE=none` forces Go Radar to ignore parent `.env` files.
- `DATABASE_URL=sqlite:///./radar.db` stores data in this directory.
- `GO_RADAR_AUTO_MIGRATE=true` creates the SQLite tables automatically.
- `TG_BOT_TOKEN` and `TG_CHAT_ID` are optional. If empty, Telegram sending is disabled.
- `GMGN_API_KEY` is optional for public fallback calls, but recommended for `gmgn-cli` and authenticated GMGN access. Direct GMGN HTTP fallback sends it as the `X-APIKEY` header when configured.
- `gmgn-cli` is optional, but recommended. For a fully standalone setup, run `npm install` in this directory so `node_modules/.bin/gmgn-cli` is available locally.

## Endpoints

- `GET /health`
- `GET /api/signals?limit=5`
- `GET /api/jobs`
- `GET /api/watchlist`
- `POST /api/watchlist`
- `GET /api/settings`
- `POST /api/settings`
- `POST /api/telegram/test`
- `GET /api/tokens/:chain/:address`
- `GET /dashboard`
- `GET /signals`
- `GET /pushes`
- `GET /radar/:source`
- `GET /token/:chain/:address`
- `GET /watchlist`
- `POST /watchlist`
- `GET /jobs`
- `GET /settings`
- `POST /settings`
- `POST /telegram/test`

## Notes

- This module can run against its own `.env` and `radar.db`.
- It auto-creates its own SQLite schema by default.
- The scheduler is available behind `GO_RADAR_ENABLE_SCHEDULER=true`.
- `s1`, `s2`, `s3`, `s5`, and `s7` have first Go scanner implementations.
- When the scheduler is enabled, migrated scanners can write snapshots/signals, create cross-source resonance signals, apply the push policy, send Telegram messages, and mark pushed signals.
- It can write runtime settings/watchlist records and can send a Telegram test message when explicitly requested.
- When running beside the old Python service, use different ports. When moved out, no Python files are required.
- Page and backend parity are being migrated incrementally.
