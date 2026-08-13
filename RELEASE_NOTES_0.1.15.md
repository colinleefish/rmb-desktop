## Tray / daemon lifecycle

- Quit now stops the backend for real: a shutdown flag blocks the health poller from restarting `rmbd` after tray exit.
- Port listeners are terminated with SIGTERM then SIGKILL; launchd `me.remember.rmbd` is unloaded on quit.
- On app launch, sidecars are refreshed and `rmbd` is always recycled so an old in-memory daemon cannot keep serving after you install a new build.

## Also includes

- Migrate existing agent recall blocks to the full `~/.rmb/bin/rmb` path.
