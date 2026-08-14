# febranDesktop — WA Gateway Backend

Dedicated Go service running WhatsApp sessions (whatsmeow) for the main [febran](https://github.com/algofran/febran) project.

This repo is the **gateway transport layer**: the `febran` API delegates WhatsApp pairing, send, and receive to this service, which holds one long-lived whatsmeow socket per number (sessions stored as SQLite per number).

See [plan/wa-gateway-backend.md](plan/wa-gateway-backend.md) for the architecture and implementation phases.

## Stack

- Go + [whatsmeow](https://github.com/tulir/whatsmeow) (multi-device)
- SQLite (`modernc.org/sqlite`, pure Go, no cgo) per-number sessions

## Related projects

| Path | Role |
|------|------|
| `D:\data\devapp\febran` | Main full-stack app (Nuxt + Go Fiber API + Postgres) |
| `D:\data\devapp\febranBackend` | Legacy backend reference |
| `D:\data\devapp\febranDesktop` | This repo — WA gateway backend |
