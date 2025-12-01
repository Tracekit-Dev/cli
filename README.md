# TraceKit CLI

Zero-friction APM setup via command line.

## Current Status

✅ **Working:** `tracekit init` command
🚧 **Not implemented yet:** login, status, test, upgrade, health, webhook commands

## Quick Start

```bash
# Test with local development server (port 8081)
./bin/tracekit init --dev

# Test with production API
./bin/tracekit init
```

## What `tracekit init` Does

1. 🔍 Detects your framework (gemvc, laravel, express, django, etc.)
2. 📧 Prompts for your email
3. 📦 Asks for service name (auto-sanitized: spaces → dashes, lowercase)
4. 🚀 Calls `/v1/integrate/register` API
5. 📨 Sends 6-digit verification code to your email
6. 🔑 Prompts you to enter the code
7. ✨ Calls `/v1/integrate/verify` API
8. 🎉 Creates:
   - User account
   - Organization (with referral_source set to framework)
   - API key
   - Hacker (free) subscription (200k traces/month)
   - Integration record
   - Partner referral (if framework is a registered partner like gemvc)
9. 💾 Saves API key to `.env` file
10. 📊 Shows dashboard URL and next steps

## Flags

- `--dev` - Use local API (http://localhost:8081)
- `--email <email>` - Pre-fill email address
- `--api-url <url>` - Custom API URL

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
