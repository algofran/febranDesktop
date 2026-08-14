# Workspace & Project Folder Map

Canonical folder layout for the Febran platform. All agents work within these folders.

## Folders

| Path | Role | Notes |
|------|------|-------|
| `D:\data\devapp\febran` | Frontend | Main website + tenant control panel |
| `D:\data\devapp\febranBackend` | Backend | Gateway backend reference |
| `D:\data\devapp\febranDesktop` | WA Gateway Backend | Dedicated Go + whatsmeow gateway service for the main `febran` project (this repo, primary workspace) |

## Conventions

- Web frontend work → `febran`.
- Backend / gateway reference → `febranBackend`.
- Desktop gateway node → `febranDesktop`.
- `D:\data\devapp\remarket\hybrid` is a separate project (re:market); out of scope for Febran unless explicitly assigned.
