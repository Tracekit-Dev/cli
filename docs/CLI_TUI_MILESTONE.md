# CLI TUI Milestone - Interactive Terminal Experience

## Completed
- [x] `tracekit dashboard` - Live terminal dashboard with health, services, alerts, anomalies, error hotspots
- [x] `--api-key` flag for standalone use without project init
- [x] `/v1/alerts/dashboard` API endpoint returning full dashboard data

## Planned

### Phase 1: Interactive Trace Browser
**`tracekit traces`**
- List recent traces with filtering (service, errors, duration, time range)
- Arrow keys to navigate, Enter to expand trace detail
- Span waterfall rendered in terminal with duration bars
- Filter bar at top (type to filter by service, toggle errors-only)
- `tracekit traces --service payment-api --errors --last 1h`

### Phase 2: Live Trace Tail
**`tracekit logs`**
- Real-time stream of incoming traces as they arrive
- Color-coded by status (green OK, red error)
- Shows service, operation, duration, error status
- Filter in real-time by service or error status
- SSE connection to server for live updates
- `tracekit logs --service payment-api`
- `tracekit logs --errors-only`

### Phase 3: Interactive Init Wizard
**Upgrade `tracekit init`**
- Replace stdin prompts with Bubbletea multi-step wizard
- Framework selection with arrow keys and visual highlight
- Animated progress during SDK installation
- Inline validation with real-time feedback
- Summary screen before confirming setup

### Phase 4: Alert Management
**`tracekit alerts`**
- `tracekit alerts list` - Table of alert rules with status (enabled/disabled/firing)
- `tracekit alerts create` - Interactive wizard to create alert rules
- `tracekit alerts delete <id>` - Delete with confirmation
- `tracekit alerts silence <id> --duration 1h` - Temporarily silence
- `tracekit alerts history` - Recent alert firings

### Phase 5: Service Explorer
**`tracekit services`**
- List all services with health indicators
- Arrow keys to select, Enter to see detail view
- Detail view: P50/P95/P99, error rate, throughput, recent errors
- `tracekit services` (interactive list)
- `tracekit services show payment-api` (direct detail)

### Phase 6: AI Ask
**`tracekit ask "why is checkout slow?"`**
- Natural language query from terminal
- Streams response from copilot API
- Shows trace references inline
- Markdown rendered in terminal
- `tracekit ask "what errors happened today?"`
- `tracekit ask "compare latency this week vs last week"`

### Phase 7: Incident Management
**`tracekit incidents`**
- View active incidents from triage inbox
- Acknowledge/investigate/resolve from terminal
- Filter by severity, team, type
- `tracekit incidents list --severity critical`
- `tracekit incidents ack <id>`
- `tracekit incidents resolve <id> --note "fixed the query"`

## API Endpoints Needed

| Command | Endpoint | Status |
|---------|----------|--------|
| dashboard | `/v1/alerts/dashboard` | Done |
| traces | `/v1/traces` (paginated, filtered) | Needs work |
| logs | `/v1/traces/stream` (SSE) | New |
| alerts | `/v1/alerts/rules` (CRUD) | Partially exists |
| services | `/v1/services` | Needs work |
| ask | `/v1/copilot/ask` | New |
| incidents | `/v1/triage` (CRUD) | New |

## Tech Stack
- **Bubbletea** - TUI framework (already added)
- **Lipgloss** - Styling (already in use)
- **Bubbles** - Components (already indirect dep)
- **Cobra** - CLI framework (already in use)
