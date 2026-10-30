# Chat with an AI CLI

`/chat` in the web is a conversation with a headless AI CLI (Antigravity, no tools) that runs in the
sandbox. It is built on the `conversation`, `transport/httpapi` and `@aicli/ui` packages of aicli.
This app provides what aicli leaves to the host (`server/internal/chat`):

| aicli asks the host for | Here |
|---|---|
| authentication | the existing bearer token (`AuthMiddleware`); `handler/chat_api.go` also requires the token's organization to be the one in the path |
| authorization | `conversation.OwnerOnly` on the principal `orgID:userID`, plus the **admin** role, because a conversation spends the organization's CLI credential |
| a `Store` | `internal/chat/store.go` on Postgres (migration `000033`), checked against aicli's `storetest` contract suite |
| a `Resolver` | `internal/chat/resolver.go`: one workspace and one CLI session directory per conversation under `CHAT_DATA_DIR`; the login is the organization's first active credential of the profile's provider (`cli:antigravity`), and a token the CLI refreshes is merged back into it |

## Turn it on

```sh
CHAT_ENABLED=true
SANDBOX_RUNTIME=docker
SANDBOX_IMAGE=aicli/base:latest      # an image that contains agy
```

Connect an Antigravity credential under AI Providers, then open **Chat** as an admin.

## Things worth knowing

- **Routing.** `/api/v1/organizations/{orgID}/chat/*`. The router's 30 s request timeout skips
  `…/chat/conversations/{id}/stream`, which is a long-lived Server-Sent Events response. The
  server's `WriteTimeout` does not cut it either (aicli lifts it for that response).
- **Origins.** Browsers must come from `CORS_ALLOWED_ORIGINS`. Requests without an `Origin` (scripts)
  are allowed because the API is authenticated by a bearer token, not a cookie.
- **No credential.** The turn still runs, the CLI starts logged out and the turn ends `auth_invalid`.
- **Dependency.** `@aicli/ui` is installed from `../../aicli/web` with
  `npm install --install-links file:../../aicli/web`. Plain `file:` makes a symlink, and the linked
  package then resolves its own React: hooks fail with "Invalid hook call". Re-run the command after
  `make web-build` in aicli to refresh the copy.
- **Go modules.** Building needs the unreleased aicli packages, so `go.work` (not committed) points
  `github.com/baby-hero-404/aicli` at `../aicli`. `go.mod` and `vendor/` can only be updated once aicli
  is tagged.
- **CLI terminals.** The login and credential-test terminals (`cli-auth`, `cli-test`) now run on aicli's
  `capture` and `capture/wsgorilla` instead of local copies. This also closes three gaps of the old
  code: the socket accepted any Origin, the credential walk followed symlinks the sandboxed process
  could plant, and a closed browser tab left the container running.
