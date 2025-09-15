# unitrack

A Bubble Tea TUI to track time per Linear issue, rounding to the next quarter hour and posting as a comment via Linear's GraphQL API. Features real-time issue title display for better context and tracking accuracy.

## Features

- **Issue Title Display**: Automatically fetches and displays Linear issue titles next to the input field as you type
- **Smart Input**: Enter just the issue number (e.g. `1234`) or full ID (e.g. `UE-1234`) - prefix is handled automatically
- **Time Tracking**: Start/pause/resume timers with automatic quarter-hour rounding
- **Limited Timers**: Set time limits that automatically stop and submit when reached - perfect for timeboxing
- **Auto-save & Recovery**: Crash protection with automatic timer state persistence
- **History Navigation**: Quick access to previously tracked issues via arrow keys
- **Theme Support**: Dark and light themes for optimal terminal readability
- **In-memory Caching**: Fast issue title lookup for previously accessed issues during the session

## Install

1. **Recommended:** Install via Go:
   ```shell
   go install github.com/apfohl-uninow/unitrack@latest
   ```
   The binary will be available as `unitrack` in your `$GOPATH/bin` (usually `~/go/bin`).

2. **Manual install:**
   - Clone this repo
   - Run `go mod tidy`
   - Build: `go build .`
   - Binary will be available as `./unitrack`

## Setup

### Linear API Key Configuration

1. **Create a Linear API Key**:
   - Go to Linear Settings → API
   - Create a new API key with **both** required permissions:
     - **`Read`** - Required to fetch issue titles and information
     - **`Create comments`** - Required to post time tracking comments
   - Copy the generated API key

2. **Configure unitrack**:
   - Ensure Go (>=1.20) is installed, and `$GOPATH/bin` (`~/go/bin` by default) is in your system `$PATH`
   - Create config file at `~/.config/unitrack/unitrack.json`:
   ```json
   {
     "api_key": "YOUR_LINEAR_API_KEY",
     "prefix": "UE",
     "timer_expire_days": 5,
     "theme": "dark"
   }
   ```
   - `api_key`: Your Linear API key with `Read` and `Create comments` permissions
   - `prefix`: The project key in issue IDs (e.g. "UE" for UE-1234)
   - `timer_expire_days` (optional): Days before saved timers expire (default: 5)
   - `theme` (optional): Color scheme - `"dark"` (default) or `"light"`

⚠️ **Important**: Your Linear API key must have **both `Read` and `Create comments` permissions** for unitrack to work properly. The `Read` permission enables issue title fetching, while `Create comments` permission allows posting time tracking comments.

## Usage

- Run with: `unitrack`
- **Issue Title Display**: As you type an issue ID, unitrack automatically fetches and displays the issue title next to the input field for better context
- The issue input placeholder uses your configured prefix (e.g. `UE-1234`)
- Enter **either** the full issue ID (e.g. `UE-1234`) **or** just the number (e.g. `1234`). If only the number is entered, the prefix from the config is used automatically
- **Smart Caching**: Issue titles are cached in memory during the session for faster subsequent lookups
- Press `Enter` to start the timer for the issue
- The timer runs and shows elapsed time (hh:mm:ss)
- Press `p` to pause, `r` to resume the timer
- **Quick Time Adjustment**: While the timer is running:
  - Press `+` to add 15 minutes to the timer
  - Press `-` to subtract 15 minutes from the timer (only if timer has at least 15 minutes)
  - For limited timers, `+` only works if there are more than 15 minutes remaining
- Press `c` to cancel (you'll get a y/n confirmation)
- Press `s` to stop, round to nearest quarter hour, and post as a comment to Linear
- Previous full issue IDs are saved in history; cycle them with `Up`/`Down` arrows
- Quit with `q` or `ctrl+c`
- All logs/output are in `$HOME/.config/unitrack/unitrack.log`

### Limited Timer

unitrack supports limited timers that automatically stop and submit time when a specified duration is reached:

- Press `l` (instead of `Enter`) to set up a limited timer
- Enter the desired time limit in minutes (e.g., `15`, `30`, `60`)
- Press `Enter` to start the limited timer
- The timer shows a progress bar indicating how much time remains
- When the time limit is reached, the timer automatically:
  - Stops the timer
  - Rounds to the nearest quarter hour
  - Posts the time as a comment to Linear
  - Shows a system notification (if supported)
- You can still pause (`p`), resume (`r`), cancel (`c`), or manually submit (`s`) before the limit is reached

**Use cases**: Perfect for timeboxing work sessions, Pomodoro technique, or ensuring you don't exceed allocated time for specific tasks.

### Auto-save & Recovery

- The timer state (including limited timers) is automatically saved every minute to `~/.config/unitrack/saved_timer_<issue_id>.json`
- If you start tracking an issue that has a saved timer, you'll be prompted to either:
  - Continue from the saved time (press `y`) - this preserves the original timer type and limit
  - Start fresh and discard the saved time (press `n`)
- Saved timers are automatically deleted when:
  - You submit the time to Linear
  - You cancel a timer
  - The saved timer is older than the configured expiration (default: 5 days)
- This feature helps recover from crashes or accidental closures, preserving both regular and limited timer states

### Theme Configuration

unitrack supports both light and dark color themes to provide optimal readability in different terminal environments:

- **Dark theme** (default): Uses muted, lighter colors optimized for dark terminal backgrounds
- **Light theme**: Uses darker, higher-contrast colors optimized for light terminal backgrounds

Configure the theme in your `~/.config/unitrack/unitrack.json`:

```json
{
  "api_key": "YOUR_LINEAR_API_KEY",
  "prefix": "UE",
  "theme": "light"
}
```

If no theme is specified, unitrack defaults to the dark theme.

## Troubleshooting

### Issue Titles Not Displaying

If issue titles are not appearing next to the input field:

1. **Check API Key Permissions**: Ensure your Linear API key has **both** required permissions:
   - `Read` - Required for fetching issue titles
   - `Create comments` - Required for posting comments
   
2. **Verify API Key**: Check that your API key in `~/.config/unitrack/unitrack.json` is correct

3. **Check Logs**: Review `~/.config/unitrack/unitrack.log` for any API errors

4. **Network Connectivity**: Ensure you can reach Linear's API at `https://api.linear.app/graphql`

Common error: `"Invalid scope: 'read' required"` means your API key needs the `Read` permission added.

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
