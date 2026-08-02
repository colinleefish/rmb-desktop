# Domain strategy

> Decision record. Publishing soon.

## Two domains, two products

| Domain | Product | Role |
|--------|---------|------|
| **colinleefish.com** (`rmb.colinleefish.com`) | Hosted **rmb** (client–server) | API server, hooks, recall, distillation pipeline, Postgres backend |
| **re-mem-ber.me** | **rmb-desktop** (local-first) | Desktop GUI, marketing site, downloads, menubar tray app |

`colinleefish.com` stays dedicated to the hosted client–server stack. The new domain is the public face of the desktop product — not a second API host.

## re-mem-ber.me

Purchased 2026-08-02. Plays on *remember* with *mem* (memory) in the middle. `.me` TLD fits a personal memory product.

### Near-term (publish)

- **Apex** (`https://re-mem-ber.me`) — product landing page, docs links, download CTA for rmb-desktop
- Bundle IDs / launch agents use reverse-DNS `me.remember.*` (hyphens dropped — standard practice)

### Deferred

| Subdomain | Planned use |
|-----------|-------------|
| `docs.re-mem-ber.me` | Desktop product documentation |
| `sync.re-mem-ber.me` | Paid cross-device sync (rmb-desktop) |

No `api.re-mem-ber.me` — API remains on `rmb.colinleefish.com`.

## colinleefish.com (hosted rmb)

Current production: `https://rmb.colinleefish.com`

- `RMB_URL` for hooks and recall CLI
- Dual-submit hooks send sessions here **and** to local `rmbd` (`127.0.0.1:19019`)
- No planned migration off this domain for the hosted service

## DNS / deploy (re-mem-ber.me, when ready)

- Point `A` / `AAAA` at hosting (can be static host or CDN; separate from rmb API server)
- Caddy (or similar) for automatic TLS
- Static site or lightweight framework for landing + download links
