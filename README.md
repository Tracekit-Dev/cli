# TraceKit CLI

Zero-friction APM setup via command line.

## Features

✅ **Implemented:**
- `tracekit init` - Create account and setup project
- `tracekit login` - Login to existing account
- `tracekit status` - Show configuration and integration status
- `tracekit test` - Send a test trace

🚧 **Not implemented yet:**
- SDK auto-installation (composer/npm/pip)
- Package distribution (npm, Homebrew)

## Quick Start

```bash
# Initialize a new project (first time)
./bin/tracekit init

# Login to existing account (existing user)
./bin/tracekit login

# Check your setup
./bin/tracekit status

# Send a test trace
./bin/tracekit test
```

## Commands

### `tracekit init`

Initialize TraceKit in a new project (first-time setup).

**What it does:**
1. 🔍 Detects your framework (gemvc, laravel, express, django, etc.)
2. 📧 Prompts for your email
3. 🚀 Calls `/v1/integrate/register` API
4. 📨 Sends 6-digit verification code to your email
5. 🔑 Prompts you to enter the code
6. ✨ Calls `/v1/integrate/verify` API
7. 🎉 Creates:
   - User account (if new)
   - Organization with fancy random name
   - API key
   - Hacker (free) subscription (200k traces/month, 100-year expiry)
   - Integration record
   - Partner referral (if framework is a registered partner)
8. 💾 Saves complete `.env` configuration
9. 📊 Shows dashboard URL and next steps

### `tracekit login`

Login to existing TraceKit account and generate a new API key.

**What it does:**
1. 📧 Prompts for your email
2. 📨 Sends 6-digit verification code
3. 🔑 Verifies code
4. ✨ Generates new API key for your existing organization
5. 💾 Saves to `.env` file

**Use cases:**
- Adding TraceKit to a new project
- Regenerating API key
- Setting up on a different machine

### `tracekit status`

Show current TraceKit configuration and integration status.

**What it shows:**
1. 📋 Configuration from `.env` file (API key, endpoint, service name, etc.)
2. 🔍 Framework detection results
3. 🔌 Integration status (service name, type, source, trace timestamps)

### `tracekit test`

Send a test trace to verify your integration is working.

**What it does:**
1. 📋 Reads configuration from `.env`
2. 🧪 Generates a test trace with events
3. 📤 Sends to TraceKit API
4. ✅ Confirms delivery

## Build

```bash
go build -o bin/tracekit .
```

## Framework Detection

**PHP:**
- GemVC (checks `composer.json` for `gemvc/library`)
- Laravel (checks for `laravel/framework`)
- Symfony (checks for `symfony/symfony`)

**Go:**
- Gin (checks `go.mod` for `github.com/gin-gonic/gin`)
- Echo (checks for `github.com/labstack/echo`)
- Fiber (checks for `github.com/gofiber/fiber`)

**Node.js:**
- Express (checks `package.json` for `"express"`)
- Next.js (checks for `"next"`)
- NestJS (checks for `"@nestjs/core"`)

**Python:**
- Django (checks `requirements.txt`)
- Flask
- FastAPI

**Ruby:**
- Rails (checks `Gemfile` for `rails`)
- Sinatra
