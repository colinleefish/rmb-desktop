# webui

React SPA embedded into `rmbd` (served under `/ui/`, built only via `make webui-build` — never
hand-edit `internal/http/static/web/`).

Package manager: **pnpm** (`packageManager` in `package.json`). Enable once: `corepack enable`.

## Dev loop

```bash
cd webui
pnpm install --frozen-lockfile   # first time / CI parity (make setup)
pnpm run dev:mock                # mock mode: full app on in-memory data, no daemon needed
pnpm run dev                     # real mode: vite proxies /api + /healthz to the running rmbd
```

- **`pnpm run dev:mock`** (`VITE_MOCK=true`) serves every `/api` call from the in-memory mock
  router (`src/lib/mock/`): ~36 sessions with turns/atoms/scenes, 60 memories, 12 skills,
  corrections, config, pipeline health. Deterministic (seeded) across reloads. Mutations
  (config PUT, corrections add/retract, onboarding reset) work in-memory. HMR picks up
  component edits live.
- **`pnpm run dev`** proxies to the daemon (default `127.0.0.1:19019`, override with
  `RMB_API_TARGET=http://… pnpm run dev`).
- The corner **badge** shows the current mode (`MOCK DATA` / `LIVE DATA`) and toggles between
  them (localStorage `rmb.mock`) without restarting the server. In `dev:mock` the mode is
  env-forced and the badge is inert.
- Legacy single-feature mocks remain available: `dev:mock-setup`, `dev:mock-pipeline`,
  `dev:onboarding`.

## Verification

- `pnpm exec tsc -b && pnpm exec oxlint src` — static gate (F10 L1).
- `pnpm exec esbuild src/lib/mock/selftest.ts --bundle --format=esm --outfile=.mock-selftest.mjs && node .mock-selftest.mjs`
  — mock-router contract selftest (F10 L2). The router must stay shape-compatible with
  `src/lib/types.ts`.
- `make webui-build` — production bundle (also embedded-check gate).
