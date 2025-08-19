# unitrack

A Bubble Tea TUI to track time per Linear issue, rounding to the next quarter hour and posting as a comment via Linear's GraphQL API.

## Setup

1. Install Go (>=1.20), and ensure `$GOPATH/bin` (`~/go/bin` by default) is in your system `$PATH`.
2. Clone this repo
3. Run: `go mod tidy`
4. Install: `go install github.com/apfohl-uninow/unitrack@latest` (binary available as `unitrack` in your `$GOPATH/bin`)
5. Create `~/.config/unitrack/unitrack.json`:
   ```json
   {
     "api_key": "YOUR_LINEAR_API_KEY",
     "prefix": "UE",
     "timer_expire_days": 5
   }
   ```
   - `prefix` configures the project key in issue IDs (e.g. "UE-1234").
   - `timer_expire_days` (optional) sets how many days before saved timers expire (default: 5).

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

### Auto-save & Recovery

- The timer state is automatically saved every minute to `~/.config/unitrack/saved_timer_<issue_id>.json`
- If you start tracking an issue that has a saved timer, you'll be prompted to either:
  - Continue from the saved time (press `y`)
  - Start fresh and discard the saved time (press `n`)
- Saved timers are automatically deleted when:
  - You submit the time to Linear
  - You cancel a timer
  - The saved timer is older than the configured expiration (default: 5 days)
- This feature helps recover from crashes or accidental closures

### Notes
- Customize the prefix for issue IDs in the config (e.g. "UI" for UI-1234).
- History, config, and logs are created/loaded automatically per session.

## CI / Releases

- **Build/test on PRs and push to main is automatic via GitHub Actions.**
- **Tagged versions like `v0.1.0` trigger a macOS build and release binary upload to repo assets.**
- **Install stable releases with:**

  ```shell
  go install github.com/apfohl-uninow/unitrack@latest
  ```

- **Or install by specific tag:**

  ```shell
  go install github.com/apfohl-uninow/unitrack@v0.1.0
  ```
