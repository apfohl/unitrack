# unitrack

A Bubble Tea TUI to track time per Linear issue, rounding to the next quarter hour and posting as a comment via Linear's GraphQL API.

## Setup

1. Install Go (>=1.20)
2. Clone this repo
3. Run: `go mod tidy`
4. Build: `go build .`
5. Create `~/.config/unitrack/unitrack.json` with:
   ```json
   { "api_key": "YOUR_LINEAR_API_KEY" }
   ```

## Usage

- Start the app: `./unitrack`
- Enter the Linear issue ID (e.g. `UE-1234`). Press `Enter` to start the timer.
- Timer runs and shows elapsed (hh:mm:ss).
- Press `p` to pause timer. Press `r` to resume.
- Press `c` to cancel the current timer and return to input.
- When timer is running, press `s` to stop, round to nearest quarter hour, and post the time as a comment to the issue.
- In input mode, use the `Up` and `Down` arrows to cycle through previous issue IDs (history is stored in `$HOME/.config/unitrack/history` and loaded on launch).
- Quit anytime with `q` or `ctrl+c`.
- All API responses and errors are logged to `$HOME/.config/unitrack/unitrack_error.log`.

### Notes
- Only unique, non-empty issue IDs are persisted to history.
- The config, error log, and history directory is created if not present.
- Any issue ID used will be available in next session's history cycle.
