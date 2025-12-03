# TraceKit CLI

**Zero-friction APM setup for modern applications.** Create an account, get an API key, and start monitoring your application in under 60 seconds.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![GitHub release](https://img.shields.io/github/v/release/Tracekit-Dev/cli)](https://github.com/Tracekit-Dev/cli/releases)

---

## 🚀 ONE COMMAND DOES EVERYTHING

```bash
tracekit init
```

That's it! This single command will:
- ✅ Detect your framework automatically
- ✅ Create your free account (email verification)
- ✅ Generate and save API key to `.env`
- ✅ Send test trace to verify setup
- ✅ Optionally install SDK
- ✅ Optionally configure health checks

**No manual configuration. No complex setup. Just one command.**

---

## 📦 Installation

### Quick Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/Tracekit-Dev/cli/main/install.sh | sh
```

### Alternative Methods

**macOS (Apple Silicon)**
```bash
curl -fsSL https://github.com/Tracekit-Dev/cli/releases/latest/download/tracekit-darwin-arm64 -o tracekit
chmod +x tracekit
sudo mv tracekit /usr/local/bin/
```

**macOS (Intel)**
```bash
curl -fsSL https://github.com/Tracekit-Dev/cli/releases/latest/download/tracekit-darwin-amd64 -o tracekit
chmod +x tracekit
sudo mv tracekit /usr/local/bin/
```

**Linux (x64)**
```bash
curl -fsSL https://github.com/Tracekit-Dev/cli/releases/latest/download/tracekit-linux-amd64 -o tracekit
chmod +x tracekit
sudo mv tracekit /usr/local/bin/
```

**Linux (ARM64)**
```bash
curl -fsSL https://github.com/Tracekit-Dev/cli/releases/latest/download/tracekit-linux-arm64 -o tracekit
chmod +x tracekit
sudo mv tracekit /usr/local/bin/
```

**Windows (x64)**
```powershell
# Download from GitHub releases
# https://github.com/Tracekit-Dev/cli/releases/latest/download/tracekit-windows-amd64.exe
```

**Build from Source**
```bash
git clone https://github.com/Tracekit-Dev/cli.git
cd cli
go build -o tracekit .
```

### Verify Installation

```bash
tracekit --version
```

---

## ⚡ Quick Start

### One-Command Setup

Simply run `tracekit init` in your project directory:

```bash
cd your-project
tracekit init
```

**Complete Interactive Flow:**

```
┌─────────────────────────────────────────────────┐
│  TraceKit - Distributed Tracing & Monitoring    │
└─────────────────────────────────────────────────┘

🔍 Framework Detection
✓ Detected: gemvc (php)

📧 Account Creation
Enter your email: dev@example.com
✓ Verification code sent to dev@example.com

🔑 Email Verification
Enter 6-digit code: 123456
✓ Account created!
✓ API key saved to .env

🧪 Sending Test Trace
✓ Test trace sent successfully!

📊 Integration Status
  Service:        my-app
  Organization:   Quantum Labs
  Plan:           Hacker (Free - 200k traces/month)

📦 SDK Installation (Optional)
Install OpenTelemetry PHP now? (Y/n): Y
✓ SDK installed successfully!

🏥 Health Check Setup (Optional)
Configure health check now? (Y/n): Y
✓ Push-based health check configured!

═══════════════════════════════════════════════════

🎉 Setup Complete!

Dashboard:  https://app.tracekit.dev
API Key:    ctxio_a1b2...xyz6 (saved to .env)
Service:    my-app
Plan:       Hacker (Free - 200k traces/month)
```

---

## 📋 Features

✅ **Account Management**
- Create accounts via email verification
- Login to existing accounts
- Generate new API keys

✅ **Framework Detection** (10+ frameworks)
- PHP: GemVC, Laravel, Symfony
- Node.js: Express, NestJS, Next.js
- Python: Django, Flask, FastAPI
- Go: Gin, Echo, Fiber
- Ruby: Rails, Sinatra

✅ **SDK Installation**
- Automatic OpenTelemetry SDK installation
- Support for composer, npm, pip, go get
- Smart package manager detection

✅ **Health Monitoring**
- Push-based heartbeats
- Pull-based endpoint checks
- Automatic alert configuration
- Email notifications

✅ **Subscription Management**
- In-CLI plan upgrades
- Stripe Checkout integration
- Real-time confirmation
- Usage tracking

✅ **What You Get (Free Hacker Plan)**
- 200,000 traces/month forever
- Email alerts
- Health monitoring
- Dashboard access
- Full distributed tracing

---

## 🛠️ Commands

### `tracekit init`

Initialize TraceKit monitoring for your project.

```bash
# Interactive mode (recommended)
tracekit init

# With options
tracekit init --email=dev@example.com --service=my-app

# Development mode (localhost API)
tracekit init --dev

# JSON output (for automation)
tracekit init --json
```

**Options:**
- `--email` - Your email address
- `--service` - Service name (default: current directory name)
- `--source` - Partner/framework code (e.g., `gemvc`)
- `--dev` - Use development server (localhost:8081)
- `--json` - Output JSON for programmatic usage

---

### `tracekit login`

Login to existing TraceKit account and generate a new API key.

```bash
tracekit login
```

**Use cases:**
- Adding TraceKit to a new project
- Regenerating API key
- Setting up on a different machine

---

### `tracekit status`

Check integration status and usage.

```bash
tracekit status
```

**Output:**
```
┌───────────────────────────────────────────┐
│  TraceKit Status                          │
└───────────────────────────────────────────┘

  Status:         Active ✓
  Service:        my-app
  Organization:   Quantum Labs
  Plan:           Hacker (Free)

  Traces:         1,234 / 200,000 this month
  Health Checks:  1 active
  Alerts:         0 active

  Dashboard: https://app.tracekit.dev
```

---

### `tracekit test`

Send a test trace to verify your integration.

```bash
tracekit test
```

---

### `tracekit health setup`

Configure health check monitoring.

```bash
# Interactive mode
tracekit health setup

# Push-based heartbeat
tracekit health setup --type=push --interval=60

# Pull-based endpoint check
tracekit health setup --type=pull --url=https://api.example.com/health
```

**Options:**
- `--type` - Check type: `push` (heartbeat) or `pull` (endpoint)
- `--interval` - Heartbeat interval in seconds (default: 60)
- `--threshold` - Consecutive failures before alert (default: 3)
- `--url` - Endpoint URL (for pull-based checks)

---

### `tracekit health list`

List all configured health checks.

```bash
tracekit health list
```

---

### `tracekit upgrade`

Upgrade your subscription plan from the CLI.

```bash
# Production upgrade
tracekit upgrade

# Development mode (localhost)
tracekit upgrade --dev
```

**Flow:**
1. CLI generates secure one-time upgrade token (15-min expiry)
2. Opens browser to upgrade page with token authentication
3. User selects plan and completes Stripe checkout
4. Success page triggers callback to CLI localhost server
5. CLI receives instant confirmation or polls for status
6. Displays updated plan and trace limits

**Features:**
- Secure token authentication
- Clickable URL if browser doesn't auto-open
- Smart timeout: 2-min callback wait + 5-min polling
- Instant feedback via browser callback

---

## 🏥 Health Check Monitoring

### Push-Based (Heartbeat)

Send heartbeats every 60 seconds to indicate service health.

**Using API:**
```bash
curl -X POST https://api.tracekit.dev/v1/health/heartbeat \
  -H "X-API-Key: $TRACEKIT_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "service_name": "my-app",
    "status": "healthy"
  }'
```

**Using Cron:**
```bash
* * * * * /usr/local/bin/tracekit health heartbeat
```

### Pull-Based (Endpoint Check)

TraceKit periodically pings your health endpoint.

```bash
tracekit health setup \
  --type=pull \
  --url=https://api.example.com/health \
  --interval=60
```

**Your endpoint should return:**
```json
{
  "status": "healthy",
  "timestamp": "2025-12-03T12:00:00Z"
}
```

---

## 🔧 Configuration

### `.env` File

TraceKit automatically creates/updates your `.env` file:

```bash
# TraceKit Configuration
TRACEKIT_API_KEY=ctxio_abc123def456...
TRACEKIT_ENDPOINT=https://api.tracekit.dev/v1/traces
TRACEKIT_SERVICE_NAME=my-app
TRACEKIT_ENABLED=true
TRACEKIT_CODE_MONITORING_ENABLED=true
```

### Supported Frameworks

| Framework | Language | Detection Method |
|-----------|----------|------------------|
| GemVC | PHP | `composer.json` contains `gemvc/library` |
| Laravel | PHP | `composer.json` contains `laravel/framework` |
| Symfony | PHP | `composer.json` contains `symfony/symfony` |
| Express | Node.js | `package.json` contains `"express"` |
| NestJS | Node.js | `package.json` contains `"@nestjs/core"` |
| Next.js | Node.js | `package.json` contains `"next"` |
| Django | Python | `requirements.txt` contains `Django` |
| Flask | Python | `requirements.txt` contains `Flask` |
| FastAPI | Python | `requirements.txt` contains `fastapi` |
| Gin | Go | `go.mod` contains `github.com/gin-gonic/gin` |
| Echo | Go | `go.mod` contains `github.com/labstack/echo` |
| Fiber | Go | `go.mod` contains `github.com/gofiber/fiber` |
| Rails | Ruby | `Gemfile` contains `gem 'rails'` |

---

## 🔐 Security

### API Key Storage

- Stored in `.env` (automatically added to `.gitignore`)
- Never logged or printed except initial generation
- Masked in `tracekit status` output

### HTTPS Only

- All API calls use HTTPS
- Certificate validation enforced
- No fallback to HTTP

---

## 🤝 Partner Integration

Frameworks can bundle TraceKit CLI for automatic monitoring setup.

### Example: Framework Installation Script

```php
<?php
// Post-install script
namespace YourFramework\Console;

use Symfony\Component\Process\Process;

class SetupMonitoring
{
    public function handle()
    {
        echo "Setting up TraceKit monitoring...\n";

        $process = new Process([
            'tracekit',
            'init',
            '--email=' . $this->askEmail(),
            '--service=' . basename(getcwd()),
            '--source=yourframework'
        ]);

        $process->setTty(true);
        $process->run();

        if (!$process->isSuccessful()) {
            echo "Failed to setup TraceKit\n";
            return 1;
        }

        echo "✅ TraceKit monitoring activated!\n";
        return 0;
    }
}
```

---

## 📚 Documentation

- **Complete Integration Guide:** [docs/INTEGRATION_CLI.md](https://github.com/Tracekit-Dev/cli/blob/main/docs/INTEGRATION_CLI.md)
- **API Documentation:** [docs/INTEGRATION_API.md](https://github.com/Tracekit-Dev/cli/blob/main/docs/INTEGRATION_API.md)
- **Dashboard:** https://app.tracekit.dev
- **Support:** support@tracekit.dev

---

## 🐛 Troubleshooting

### "tracekit: command not found"

**Solution:**
```bash
# Add to PATH
export PATH="/usr/local/bin:$PATH"

# Or reinstall
curl -fsSL https://raw.githubusercontent.com/Tracekit-Dev/cli/main/install.sh | sh
```

### "Email verification failed"

**Solutions:**
- Check spam folder for verification email
- Request new code (restart `tracekit init`)
- Ensure code is entered within 15 minutes

### "Health check not receiving heartbeats"

**Solutions:**
```bash
# Test manual heartbeat
tracekit health heartbeat

# Check if API key is set
echo $TRACEKIT_API_KEY

# Verify configuration
tracekit status
```

### Debug Mode

```bash
# Enable debug output
export TRACEKIT_DEBUG=1
tracekit init
```

---

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

---

## 🤝 Contributing

Contributions welcome! Please feel free to submit a Pull Request.

---

## 🌟 Star History

If TraceKit CLI makes your life easier, consider giving us a star! ⭐

---

**Ready to get started?** Run `tracekit init` now! 🚀
