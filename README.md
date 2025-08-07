# unitrack

A Bubble Tea TUI to track time per Linear issue, rounding to the next quarter hour and posting as a comment via Linear's GraphQL API.

## Setup

1. Install Go (>=1.20), and ensure `$GOPATH/bin` (`~/go/bin` by default) is in your system `$PATH`.
2. Clone this repo
3. Run: `go mod tidy`
4. Install: `go install .` (the binary will be available as `unitrack` in your `$GOPATH/bin`)
5. Create `~/.config/unitrack/unitrack.json`:
   ```json
   { "api_key": "YOUR_LINEAR_API_KEY", "prefix": "UE" }
   ```
   - `prefix` configures the project key in issue IDs (e.g. "UE-1234").

## Usage

- Run with: `unitrack`
- The issue input placeholder uses your configured prefix (e.g. `UE-1234`).
- Enter **either** the full issue ID (e.g. `UE-1234`) **or** just the number (e.g. `1234`). If only the number is entered, the prefix from the config is used automatically.
- Press `Enter` to start the timer for the issue.
- The timer runs and shows elapsed (hh:mm:ss).
- Press `p` to pause, `r` to resume the timer.
- Press `c` to cancel (you'll get a y/n confirmation).
- Press `s` to stop, round to nearest quarter hour, and post as a comment to Linear.
- Previous full issue IDs are saved in history; cycle them with `Up`/`Down` arrows.
- Quit with `q` or `ctrl+c`.
- All logs/output are in `$HOME/.config/unitrack/unitrack.log`.

### Notes
- Customize the prefix for issue IDs in the config (e.g. "UI" for UI-1234).
- History, config, and logs are created/loaded automatically per session.
