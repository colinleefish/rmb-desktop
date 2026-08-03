# rmb webui (Vite + React)

Local dashboard served by `rmbd` at `http://127.0.0.1:19019/ui/`.

## Dev

Terminal 1 — daemon:

```bash
make build && make run-rmbd
```

Terminal 2 — webui with API proxy:

```bash
make webui-dev
```

Open http://localhost:5173/ui/ (proxies `/api` to rmbd).

## Production embed

```bash
make webui-build    # builds webui/dist → internal/http/static/web/
make build          # embeds static assets into rmbd binary
```

Or `make build-all` for both steps.
