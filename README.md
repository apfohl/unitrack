# unitrack

A simple Bubble Tea TUI to track time spent per Linear issue, rounding to the next quarter hour and posting as a comment via Linear's GraphQL API.

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

1. Start the app:
   ```sh
   ./unitrack
   ```
2. Enter the Linear issue ID (e.g. `UE-1234`). Press `Enter` to start the timer.
3. Timer shows elapsed (hh:mm:ss).
4. Press `s` to stop, round the timer to the nearest quarter hour, and post as a comment to the issue.
5. Enter another issue ID to repeat.

- All API responses and errors are logged to `unitrack_error.log` in the repo directory.
- Quit anytime with `q` or `ctrl+c`.
