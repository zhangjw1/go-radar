# Web3 Radar Frontend

This is the Arco Design Pro Vue frontend scaffold for the Go radar backend.

## Development

Run the Go backend on `127.0.0.1:8080`, then start the frontend:

```powershell
cd D:\Users\jiawei.zhang\project\go-radar\web
pnpm install
pnpm dev
```

The Arco Pro scaffold keeps its local mock endpoints under `/api/*`, including
the demo login page. Go Radar backend endpoints use `/radar-api/*` in the
frontend and are proxied to `http://127.0.0.1:8080/api/*`.

Open the scaffold entry page at:

```text
http://127.0.0.1:5173/#/dashboard/workplace
```

## Page Structure

The web app keeps the full Arco Design Pro Vue scaffold as the primary admin
shell, including dashboard, visualization, list, form, profile, result,
exception, user center, external Arco Design, and FAQ menu entries.

Go Radar-specific pages are mounted under `/radar/*` as an extension area:

- `/radar/dashboard`: radar overview
- `/radar/signals`: signal list
- `/radar/s1`, `/radar/s2`, `/radar/s3`, `/radar/s5`, `/radar/s7`: scanner views
- `/radar/jobs`: scanner runs
- `/radar/pushes`: Telegram push records
- `/radar/watchlist`: watchlist
- `/radar/settings`: runtime settings
