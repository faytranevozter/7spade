# Coding Standards

This project is a monorepo: a Go HTTP API (`services/api`), a Go WebSocket server (`services/ws`), and a React + TypeScript frontend (`web/`).

---

## Go (services/api, services/ws)

### Style
- Follow standard Go formatting (`gofmt` / `goimports`).
- Use `camelCase` for unexported identifiers, `PascalCase` for exported ones.
- Error strings are lowercase and do not end with punctuation.
- Prefer `errors.New` / `fmt.Errorf` with `%w` for wrapping; return typed errors for sentinel cases.
- Use `log.Printf` / `log.Fatal` for logging — no third-party logger unless added in a future slice.
- Configuration comes exclusively from environment variables (via `os.Getenv`). No config files.
- All HTTP handlers live in `services/<service>/` as separate files per domain (e.g. `auth.go`, `rooms.go`).
- The Game Engine (`services/ws/engine/` or equivalent) must be a **pure package** — zero I/O, zero global state.

### Error handling
- Every error must be handled explicitly; never use `_` to discard errors in production code.
- HTTP handlers write a JSON error body with a `message` field and an appropriate status code.
- WebSocket errors are returned only to the sender as `{"type":"error","message":"..."}`.

### Testing
- Unit tests use the standard `testing` package; table-driven tests preferred.
- Test files live alongside the code they test (`foo_test.go`).
- Integration tests that require Redis use testcontainers-go.
- Every exported function in the Game Engine package must have at least one test.
- Use a fixed seed for any test involving `Deal` to keep results deterministic.

### Architecture
- Services communicate only via Redis and JWTs — never direct HTTP calls to each other.
- The WebSocket server never writes to PostgreSQL directly; persistence is delegated to the API via the shared data model.
- Keep I/O (Redis, Postgres, HTTP) at the edges; core logic in pure functions.

---

## TypeScript / React (web/)

### Style
- Use `camelCase` for variables and functions, `PascalCase` for components and types.
- Prefer named exports over default exports (exception: page-level route components).
- Use TypeScript strict mode; avoid `any` — use `unknown` and narrow explicitly.
- Prefer `const` over `let`; never use `var`.
- Use template literals over string concatenation.

### Components
- One component per file; filename matches the component name.
- Keep components small and focused; extract custom hooks for stateful logic.
- Use `React.FC` sparingly — prefer plain function declarations with explicit return types.
- Avoid nested ternary operators; prefer early returns or `if/else` blocks.

### State & side-effects
- Manage WebSocket state in a dedicated context/hook (`useGameSocket` or equivalent).
- JWT is stored in `localStorage`; never log or expose it.
- Use `useEffect` cleanup functions to close WebSocket connections on unmount.

### Testing
- Component tests use Vitest + Testing Library.
- Test behaviour, not implementation details (no snapshot tests unless justified).
- Every interactive component (card click, modal) must have at least one test.

---

## Commits
- Commit messages start with `RALPH:` prefix.
- Include: task completed + issue reference, key decisions, files changed, and any blockers.
- Keep commits focused; one logical change per commit.

## Branches
- Feature branches follow the pattern `sandcastle/issue-{id}-{slug}`.
- Branch off `main`; rebase or merge `main` before opening a PR.

## Pull Requests
- Every change goes through a PR — no direct pushes to `main`.
- PR title mirrors the issue title.
- PR description must reference the issue (`Closes #<id>`), summarise what changed, and note any decisions made.
- Review findings are posted as PR review comments (not just inline commit comments).
- Merge only after review is complete and tests pass.
