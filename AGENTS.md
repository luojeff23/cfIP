# Repository Guidelines

## Project Structure & Module Organization

- `main.go` starts the HTTP server and WebSocket handler.
- `scanner/` contains the Go scanning logic (`scanner.go`) for TCP ping and speed tests.
- `static/` holds the web UI assets (`index.html`, `style.css`, `app.js`).
- `go.mod` / `go.sum` define dependencies.
- `run.bat` is a Windows helper to run the app.
- `效果图.jpg` is a UI screenshot used in the README.

## Build, Test, and Development Commands

- `go run main.go` — run the server locally (serves UI at `http://localhost:13334`).
- `run.bat` — Windows shortcut for `go run main.go`.
- `go build -o cfping` — build a local binary.
- `go test ./...` — run tests (none currently in this repo, but use when adding tests).

## Coding Style & Naming Conventions

- Go code should remain `gofmt`-formatted (tabs, standard Go layout).
- Package names are short and lowercase (e.g., `scanner`).
- Exported struct fields use `CamelCase` with JSON tags in `snake_case`.
- Frontend files use simple, descriptive IDs/classes (e.g., `#results-body`, `.btn-primary`).

## Testing Guidelines

- No automated tests are present; consider adding `_test.go` files under `scanner/`.
- Prefer table-driven tests for scan edge cases (CIDR expansion, timeouts).
- Run `go test ./...` before submitting changes that affect logic.

## Commit & Pull Request Guidelines

- This checkout does not include Git history, so no existing commit convention can be inferred.
- Use short, imperative commit subjects (e.g., `Add speed test timeout`).
- PRs should include: a brief description, test command(s) run, and UI screenshots if `static/` changes.

## Configuration & Safety Notes

- Default server port is `13334`; change in `main.go` if needed.
- Speed tests use the URL in the UI (default is the Cloudflare download endpoint).
- Be mindful when scanning large CIDR ranges; the server caps scans to 10,000 IPs.
