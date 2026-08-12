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

### Integration UI without touching real agent configs

While using Cursor (or any agent) on the same machine, mock the integration tab so Apply never writes to disk:

```bash
cd webui && npm run dev:mock-setup
```

Or in the browser console on any dev session: `localStorage.setItem('rmb.mockSetup', '1')` then reload.
Disable: `localStorage.removeItem('rmb.mockSetup')`.

Only Settings → Integration uses mock data; sessions, memories, and the rest still hit rmbd.

### Pipeline health dashboard (UI demo)

Prototype the distillation health page with a 100-session backlog scenario (no rmbd required):

```bash
cd webui && npm run dev:mock-pipeline
```

Open http://localhost:5173/ui/pipeline

Or in the browser console: `localStorage.setItem('rmb.mockPipeline', '1')` then reload.
Disable: `localStorage.removeItem('rmb.mockPipeline')`.

### Onboarding wizard (UI demo)

Prototype the first-run setup wizard. Connection test calls rmbd (start rmbd for real checks). Agent apply is simulated:

```bash
cd webui && npm run dev:onboarding
```

Open http://localhost:5173/ui/ — you are redirected to `/ui/onboarding` until you finish or skip.

Or in any dev session:

```bash
localStorage.setItem('rmb.mockOnboarding', '1')
localStorage.setItem('rmb.mockSetup', '1')
localStorage.removeItem('rmb.onboardingComplete')
```

Reload. Reset demo: `localStorage.removeItem('rmb.onboardingComplete')` then reload.

Direct link to wizard anytime: http://localhost:5173/ui/onboarding

## Production embed

```bash
make build-all      # webui-build + go build (recommended after clone)
```

Or step by step:

```bash
make webui-build    # builds webui/dist → internal/http/static/web/
make build          # embeds static assets into rmbd binary
```

`internal/http/static/web/` is **gitignored** (Vite output). `make build` and `make test` fail with a clear message if you skip `webui-build`.
