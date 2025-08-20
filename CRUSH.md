# CRUSH.md

Repository: `github.com/uninow/unitrack`
Language: Go (Bubble Tea TUI)

## Build/Test/Lint Commands
- Install: `go mod tidy`
- Build: `go build .`
- Build with version: `go build -ldflags "-X main.version=v1.1.0" .`
- Run: `./unitrack`
- Run tests: `go test ./...`
- Run single test: `go test -run <TestName> ./...`
- Format: `gofmt -w .`
- Lint (if installed): `golangci-lint run`

## Code Style & Conventions
- Use Go 1.20+
- Imports: Standard, then external, then internal; prefer `goimports` ordering.
- Naming: CamelCase exported, lowerCamelCase private, uppercase for acronyms (e.g. ID, API).
- Types: Built-in Go types; explicit for exported API/funcs.
- Error Handling: Always check errors; context-wrap where helpful. Never panic for normal flow.
- Formatting: Run `gofmt` before commit.
- Functions: Keep small and composable. Exported require doc comments.
- Tests: Suffix: `_test.go`, names start with `Test`, use Go test framework.
- Context: Pass `context.Context` as first arg if used.
- Secrets: Never commit secrets/tokens/config keys.

## App-Specific
- Input is a string issueID (e.g., UE-1234). Timer is in hh:mm:ss. Press `s` to post ceiled time to Linear.
- Logs all Linear requests and errors to `unitrack_error.log`.
- API key is loaded from `$HOME/.config/unitrack/unitrack.json`.
- Configuration, logs, and binary are all `.gitignore`'d.
- Bubble Tea, Bubbles, Resty libraries in use. No comments in code unless documenting exported declarations.
- `--version` flag shows program name and version (e.g., "unitrack v1.1.0" or "unitrack unknown" for local builds).

## Misc
- `.crush/`, `.config/`, `unitrack_error.log`, and all OS/binary artifacts are `.gitignore`'d.
