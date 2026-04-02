# TraceKit CLI

**Zero-friction APM for modern applications.** A full observability TUI in your terminal -- live dashboards, trace browsing, alert management, AI copilot, and more.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![GitHub release](https://img.shields.io/github/v/release/Tracekit-Dev/cli)](https://github.com/Tracekit-Dev/cli/releases)

---

## Installation

### Homebrew (macOS & Linux)

```bash
brew install Tracekit-Dev/tap/tracekit
```

### Quick Install Script

```bash
curl -fsSL https://raw.githubusercontent.com/Tracekit-Dev/cli/main/install.sh | sh
```

### Manual Download

**macOS (Apple Silicon)**
```bash
curl -fsSL https://github.com/Tracekit-Dev/cli/releases/latest/download/tracekit-darwin-arm64 -o tracekit
chmod +x tracekit && sudo mv tracekit /usr/local/bin/
```

**macOS (Intel)**
```bash
curl -fsSL https://github.com/Tracekit-Dev/cli/releases/latest/download/tracekit-darwin-amd64 -o tracekit
chmod +x tracekit && sudo mv tracekit /usr/local/bin/
```

**Linux (x64)**
```bash
curl -fsSL https://github.com/Tracekit-Dev/cli/releases/latest/download/tracekit-linux-amd64 -o tracekit
chmod +x tracekit && sudo mv tracekit /usr/local/bin/
```

**Linux (ARM64)**
```bash
curl -fsSL https://github.com/Tracekit-Dev/cli/releases/latest/download/tracekit-linux-arm64 -o tracekit
chmod +x tracekit && sudo mv tracekit /usr/local/bin/
```

**Windows (x64)** -- download from [GitHub Releases](https://github.com/Tracekit-Dev/cli/releases/latest/download/tracekit-windows-amd64.exe)

**Build from Source**
```bash
git clone https://github.com/Tracekit-Dev/cli.git
cd cli && go build -o tracekit .
```

### Verify Installation

```bash
tracekit --version
```

---

## Quick Start

```bash
# Initialize a new project (creates account, detects framework, saves config)
tracekit init

# Or login to an existing account
tracekit login

# Open the live dashboard
tracekit dashboard
```

Use `Ctrl+N` in any TUI screen to switch between views without quitting.

---

## Commands

### Account & Setup

#### `tracekit init`

Interactive setup wizard. Detects your framework, creates your account, generates an API key, and optionally installs the SDK.

```bash
tracekit init
tracekit init --email dev@example.com
```

#### `tracekit login`

Login to an existing account. Saves credentials to `~/.tracekitconfig`.

```bash
tracekit login
tracekit login --email dev@example.com
tracekit login --api-url http://localhost:8081 --tag local
```

| Flag | Description |
|------|-------------|
| `--email` | Pre-fill email address |
| `--api-url` | Target server URL (default: https://app.tracekit.dev) |
| `--tag` | Tag this profile with a short name (e.g. `prod`, `local`) |

#### `tracekit status`

Show current configuration, framework detection, and integration status.

```bash
tracekit status
tracekit status --env local    # check a specific profile by tag
```

#### `tracekit test`

Send a test trace to verify your integration is working.

```bash
tracekit test
```

---

### Live TUI Views

All TUI views support `Ctrl+N` to switch between views, and `q` or `Ctrl+C` to quit.

#### `tracekit dashboard`

Live-updating dashboard with health score, service overview, error hotspots, anomaly count, and performance sparklines. Auto-refreshes every 30 seconds.

```bash
tracekit dashboard
```

| Key | Action |
|-----|--------|
| `1` / `2` / `3` | Switch time window: 1h / 6h / 24h |
| `r` | Refresh now |
| `Ctrl+N` | Switch view |

#### `tracekit traces`

Interactive trace browser with filtering, search, and detail view.

```bash
tracekit traces
```

| Key | Action |
|-----|--------|
| `j/k` or arrows | Navigate trace list |
| `enter` | Open trace detail (spans, attributes) |
| `/` | Filter by service |
| `e` | Toggle errors-only |
| `d` | Set minimum duration filter |
| `t` | Cycle time range (1h / 6h / 24h / all) |
| `r` | Refresh |

#### `tracekit logs`

Stream live traces in real-time via Server-Sent Events.

```bash
tracekit logs
tracekit logs --service my-api
tracekit logs --errors
tracekit logs --service my-api --errors
```

| Key | Action |
|-----|--------|
| `j/k` or arrows | Scroll through log history |
| `g` / `G` | Jump to top / bottom |

#### `tracekit services`

Browse service health, metrics, and error breakdowns.

```bash
tracekit services
```

| Key | Action |
|-----|--------|
| `j/k` or arrows | Navigate service list |
| `enter` | Open service detail |
| `tab` | Switch between metrics and errors tabs |
| `esc` | Back to service list |
| `r` | Refresh |

#### `tracekit alerts`

Manage alert rules -- create, toggle, delete, and view firing history.

```bash
tracekit alerts
```

| Key | Action |
|-----|--------|
| `j/k` or arrows | Navigate alert list |
| `n` | Create new alert rule |
| `d` | Delete selected rule |
| `t` | Toggle enabled/disabled |
| `h` | View alert history |
| `r` | Refresh |

#### `tracekit incidents`

Triage incidents from the unified inbox with status transitions.

```bash
tracekit incidents
```

| Key | Action |
|-----|--------|
| `j/k` or arrows | Navigate incident list |
| `a` | Acknowledge |
| `i` | Investigate |
| `r` | Resolve (prompts for notes) |
| `z` | Snooze (select duration) |
| `s` / `t` / `w` | Filter by severity / type / team |
| `esc` | Clear filter |

#### `tracekit ask`

AI copilot that analyzes your traces, services, and metrics.

```bash
tracekit ask "why is latency high?"
tracekit ask                          # interactive chat mode
```

---

### Profile Management

Manage credentials for multiple servers (production, staging, local dev).

#### `tracekit profile`

List all saved profiles.

```bash
tracekit profile
```

```
  Saved Profiles

  > https://app.tracekit.dev  [prod]  (active)
      API Key:  ctxio_7a9958291...b747
      Service:  context.io

    http://localhost:8081  [local]
      API Key:  ctxio_542eb9c37...a672
      Service:  cli
```

#### `tracekit profile use <url-or-tag>`

Switch the active profile by URL or tag.

```bash
tracekit profile use prod
tracekit profile use http://localhost:8081
```

#### `tracekit profile tag <url-or-tag> <name>`

Tag a profile with a short name for quick switching.

```bash
tracekit profile tag https://app.tracekit.dev prod
tracekit profile tag http://localhost:8081 local
```

#### `tracekit profile remove <url-or-tag>`

Delete a saved profile.

```bash
tracekit profile remove local
```

#### Using profiles with commands

Use `--env` on any command to target a specific profile by tag or URL:

```bash
tracekit dashboard --env local
tracekit traces --env prod
tracekit status --env http://localhost:8081
```

---

### Releases & Deploys

Track releases and deployments for error attribution.

```bash
# Create a release
tracekit releases new v1.2.3

# Create, deploy, and finalize in one step
tracekit releases deploy --env production

# List releases
tracekit releases list

# Finalize a release
tracekit releases finalize v1.2.3
```

### Source Maps

Upload source maps for readable JavaScript stack traces.

```bash
# Upload source maps from build output
tracekit sourcemaps upload ./dist
tracekit sourcemaps upload ./dist --release v1.2.3

# Delete source maps for a release
tracekit sourcemaps delete --release v1.2.3
```

### Webhooks

Receive real-time notifications for health check failures, alerts, and errors.

```bash
tracekit webhook create
tracekit webhook list
tracekit webhook delete <webhook-id>
```

### Health Checks

Configure push-based heartbeats or pull-based endpoint monitoring.

```bash
# Interactive setup
tracekit health setup

# List configured checks
tracekit health list
```

### Subscription

```bash
# Upgrade your plan (opens browser for Stripe checkout)
tracekit upgrade
```

### CLI Updates

```bash
# Update the CLI binary to latest version
tracekit update
tracekit update --check    # check without installing
```

---

## Configuration

### Multi-Profile Config (`~/.tracekitconfig`)

Credentials are stored as JSON with URL-keyed profiles:

```json
{
  "active": "https://app.tracekit.dev",
  "profiles": {
    "https://app.tracekit.dev": {
      "api_key": "ctxio_abc123...",
      "user_id": "...",
      "endpoint": "https://app.tracekit.dev",
      "service_name": "my-app",
      "tag": "prod"
    },
    "http://localhost:8081": {
      "api_key": "ctxio_def456...",
      "user_id": "...",
      "endpoint": "http://localhost:8081",
      "service_name": "my-app",
      "tag": "local"
    }
  }
}
```

Legacy `.env` format is auto-migrated on first read.

### Local `.env` File

For SDK configuration in your project directory:

```bash
TRACEKIT_API_KEY=ctxio_abc123...
TRACEKIT_ENDPOINT=https://app.tracekit.dev
TRACEKIT_SERVICE_NAME=my-app
TRACEKIT_ENABLED=true
```

### Global Flag

| Flag | Description |
|------|-------------|
| `--env` | Profile tag, URL, or file path to load credentials from |

---

## Supported Frameworks

| Framework | Language | Detection |
|-----------|----------|-----------|
| GemVC | PHP | `composer.json` |
| Laravel | PHP | `composer.json` |
| Symfony | PHP | `composer.json` |
| Express | Node.js | `package.json` |
| NestJS | Node.js | `package.json` |
| Next.js | Node.js | `package.json` |
| Django | Python | `requirements.txt` |
| Flask | Python | `requirements.txt` |
| FastAPI | Python | `requirements.txt` |
| Gin | Go | `go.mod` |
| Echo | Go | `go.mod` |
| Fiber | Go | `go.mod` |
| Rails | Ruby | `Gemfile` |

---

## Troubleshooting

**"tracekit: command not found"**
```bash
export PATH="/usr/local/bin:$PATH"
# Or reinstall
curl -fsSL https://raw.githubusercontent.com/Tracekit-Dev/cli/main/install.sh | sh
```

**"unauthorized: API key is invalid or expired"**
The CLI will automatically prompt you to re-login. Or manually:
```bash
tracekit login
```

**Update failed with permission error**
```bash
sudo tracekit update
```

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

**Get started:** `tracekit init`
