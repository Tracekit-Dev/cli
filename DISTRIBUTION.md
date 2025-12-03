# TraceKit CLI Distribution Guide

This guide explains how to distribute the TraceKit CLI via Homebrew, APT, and other package managers.

---

## 📦 Current Distribution Methods

✅ **GitHub Releases** - Automated via GitHub Actions
✅ **Install Script** - `curl -fsSL https://raw.githubusercontent.com/Tracekit-Dev/cli/main/install.sh | sh`

---

## 🍺 Homebrew Distribution

### Step 1: Create a Homebrew Tap Repository

1. **Create a new GitHub repository:**
   ```
   Repository name: homebrew-tap
   Organization: Tracekit-Dev
   Public repository
   ```

2. **Clone the repository:**
   ```bash
   git clone https://github.com/Tracekit-Dev/homebrew-tap.git
   cd homebrew-tap
   ```

3. **Create the formula file:**
   ```bash
   mkdir -p Formula
   ```

4. **Create `Formula/tracekit.rb`:**
   ```ruby
   class Tracekit < Formula
     desc "TraceKit CLI - Zero-friction APM setup"
     homepage "https://tracekit.dev"
     version "1.0.1"

     on_macos do
       if Hardware::CPU.arm?
         url "https://github.com/Tracekit-Dev/cli/releases/download/v1.0.1/tracekit-darwin-arm64"
         sha256 "DARWIN_ARM64_SHA256"
       else
         url "https://github.com/Tracekit-Dev/cli/releases/download/v1.0.1/tracekit-darwin-amd64"
         sha256 "DARWIN_AMD64_SHA256"
       end
     end

     on_linux do
       if Hardware::CPU.arm?
         url "https://github.com/Tracekit-Dev/cli/releases/download/v1.0.1/tracekit-linux-arm64"
         sha256 "LINUX_ARM64_SHA256"
       else
         url "https://github.com/Tracekit-Dev/cli/releases/download/v1.0.1/tracekit-linux-amd64"
         sha256 "LINUX_AMD64_SHA256"
       end
     end

     def install
       if OS.mac?
         if Hardware::CPU.arm?
           bin.install "tracekit-darwin-arm64" => "tracekit"
         else
           bin.install "tracekit-darwin-amd64" => "tracekit"
         end
       elsif OS.linux?
         if Hardware::CPU.arm?
           bin.install "tracekit-linux-arm64" => "tracekit"
         else
           bin.install "tracekit-linux-amd64" => "tracekit"
         end
       end
     end

     test do
       system "#{bin}/tracekit", "--version"
     end
   end
   ```

### Step 2: Get SHA256 Checksums

Download the SHA256 files from the v1.0.1 release:

```bash
# macOS ARM64
curl -sL https://github.com/Tracekit-Dev/cli/releases/download/v1.0.1/tracekit-darwin-arm64.sha256

# macOS AMD64
curl -sL https://github.com/Tracekit-Dev/cli/releases/download/v1.0.1/tracekit-darwin-amd64.sha256

# Linux ARM64
curl -sL https://github.com/Tracekit-Dev/cli/releases/download/v1.0.1/tracekit-linux-arm64.sha256

# Linux AMD64
curl -sL https://github.com/Tracekit-Dev/cli/releases/download/v1.0.1/tracekit-linux-amd64.sha256
```

Replace `DARWIN_ARM64_SHA256`, etc. in the formula with actual checksums.

### Step 3: Commit and Push

```bash
git add Formula/tracekit.rb
git commit -m "Add tracekit formula v1.0.1"
git push origin main
```

### Step 4: Users Can Install

```bash
brew tap Tracekit-Dev/tap
brew install tracekit
```

Or one-line install:
```bash
brew install Tracekit-Dev/tap/tracekit
```

### Automated Updates

Create a GitHub Action in the `homebrew-tap` repo to auto-update the formula:

**.github/workflows/update-formula.yml:**
```yaml
name: Update Formula

on:
  repository_dispatch:
    types: [new-release]

jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Update formula
        env:
          VERSION: ${{ github.event.client_payload.version }}
        run: |
          # Download checksums
          DARWIN_ARM64=$(curl -sL https://github.com/Tracekit-Dev/cli/releases/download/${VERSION}/tracekit-darwin-arm64.sha256 | awk '{print $1}')
          DARWIN_AMD64=$(curl -sL https://github.com/Tracekit-Dev/cli/releases/download/${VERSION}/tracekit-darwin-amd64.sha256 | awk '{print $1}')
          LINUX_ARM64=$(curl -sL https://github.com/Tracekit-Dev/cli/releases/download/${VERSION}/tracekit-linux-arm64.sha256 | awk '{print $1}')
          LINUX_AMD64=$(curl -sL https://github.com/Tracekit-Dev/cli/releases/download/${VERSION}/tracekit-linux-amd64.sha256 | awk '{print $1}')

          # Update formula
          sed -i "s/version \".*\"/version \"${VERSION#v}\"/" Formula/tracekit.rb
          sed -i "s|download/v[^/]*/|download/${VERSION}/|g" Formula/tracekit.rb
          sed -i "s/DARWIN_ARM64_SHA256/${DARWIN_ARM64}/" Formula/tracekit.rb
          sed -i "s/DARWIN_AMD64_SHA256/${DARWIN_AMD64}/" Formula/tracekit.rb
          sed -i "s/LINUX_ARM64_SHA256/${LINUX_ARM64}/" Formula/tracekit.rb
          sed -i "s/LINUX_AMD64_SHA256/${LINUX_AMD64}/" Formula/tracekit.rb

      - name: Commit changes
        run: |
          git config user.name "GitHub Actions"
          git config user.email "actions@github.com"
          git add Formula/tracekit.rb
          git commit -m "Update tracekit to ${VERSION}"
          git push
```

Trigger from CLI repo's release workflow by adding:
```yaml
- name: Trigger Homebrew update
  run: |
    curl -X POST \
      -H "Authorization: token ${{ secrets.HOMEBREW_TAP_TOKEN }}" \
      -H "Accept: application/vnd.github.v3+json" \
      https://api.github.com/repos/Tracekit-Dev/homebrew-tap/dispatches \
      -d '{"event_type":"new-release","client_payload":{"version":"${{ steps.version.outputs.VERSION }}"}}'
```

---

## 📦 APT Repository (Debian/Ubuntu)

APT distribution requires more infrastructure. Here are two approaches:

### Option 1: Simple - Using GitHub Pages

1. **Create `deb` packages in your release workflow:**

Add to `.github/workflows/release.yml`:

```yaml
- name: Create DEB package
  if: matrix.goos == 'linux'
  run: |
    VERSION="${{ steps.version.outputs.VERSION }}"
    ARCH="${{ matrix.goarch }}"

    # Convert Go arch to Debian arch
    case $ARCH in
      amd64) DEB_ARCH="amd64" ;;
      arm64) DEB_ARCH="arm64" ;;
      *) exit 1 ;;
    esac

    # Create package structure
    mkdir -p tracekit_${VERSION#v}_${DEB_ARCH}/usr/local/bin
    mkdir -p tracekit_${VERSION#v}_${DEB_ARCH}/DEBIAN

    # Copy binary
    cp tracekit-linux-$ARCH tracekit_${VERSION#v}_${DEB_ARCH}/usr/local/bin/tracekit
    chmod +x tracekit_${VERSION#v}_${DEB_ARCH}/usr/local/bin/tracekit

    # Create control file
    cat > tracekit_${VERSION#v}_${DEB_ARCH}/DEBIAN/control <<EOF
    Package: tracekit
    Version: ${VERSION#v}
    Architecture: ${DEB_ARCH}
    Maintainer: TraceKit Team <support@tracekit.dev>
    Description: TraceKit CLI - Zero-friction APM setup
     TraceKit CLI enables single-command account creation,
     framework detection, and SDK installation for application monitoring.
    Homepage: https://tracekit.dev
    EOF

    # Build .deb
    dpkg-deb --build tracekit_${VERSION#v}_${DEB_ARCH}

    # Upload to release
    echo "tracekit_${VERSION#v}_${DEB_ARCH}.deb" >> release_files.txt

- name: Upload DEB package
  if: matrix.goos == 'linux'
  uses: softprops/action-gh-release@v1
  with:
    files: tracekit_*.deb
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

2. **Users install with:**
```bash
# Download and install
curl -LO https://github.com/Tracekit-Dev/cli/releases/latest/download/tracekit_1.0.1_amd64.deb
sudo dpkg -i tracekit_1.0.1_amd64.deb
```

### Option 2: Full APT Repository (Recommended for Production)

Use a service like:
- **GitHub Packages** (free for public repos)
- **Cloudsmith** (free tier available)
- **PackageCloud** (paid)
- **Self-hosted** (using reprepro)

**Using Cloudsmith (Easiest):**

1. Sign up at https://cloudsmith.io
2. Create a repository: `tracekit-dev/cli`
3. Get API key
4. Add to release workflow:

```yaml
- name: Upload to Cloudsmith
  if: matrix.goos == 'linux'
  env:
    CLOUDSMITH_API_KEY: ${{ secrets.CLOUDSMITH_API_KEY }}
  run: |
    pip install cloudsmith-cli
    cloudsmith push deb tracekit-dev/cli/ubuntu/jammy tracekit_*.deb
```

5. **Users install with:**
```bash
# Add repository
curl -1sLf 'https://dl.cloudsmith.io/public/tracekit-dev/cli/setup.deb.sh' | sudo -E bash

# Install
sudo apt-get install tracekit
```

---

## 🐧 Other Linux Package Managers

### Snap

```yaml
name: tracekit
version: '1.0.1'
summary: TraceKit CLI - Zero-friction APM setup
description: |
  TraceKit CLI enables single-command account creation,
  framework detection, and SDK installation.

grade: stable
confinement: strict
base: core20

apps:
  tracekit:
    command: bin/tracekit
    plugs: [network, home]

parts:
  tracekit:
    plugin: go
    source: .
    build-snaps: [go/latest/stable]
```

Build and publish:
```bash
snapcraft
snapcraft upload --release=stable tracekit_1.0.1_amd64.snap
```

### Flatpak

Requires more setup. Consider if you need sandboxed distribution.

---

## 📊 Distribution Priority

**Recommended order:**

1. ✅ **GitHub Releases** (Done)
2. ✅ **Install Script** (Done)
3. 🔄 **Homebrew** (Easy, high value for macOS/Linux users)
4. 🔄 **DEB packages** (Medium effort, good for Ubuntu/Debian users)
5. ⏳ **APT repository** (Higher effort, best UX for Linux)
6. ⏳ **Snap/Flatpak** (Optional, niche use cases)

---

## 🚀 Quick Start: Homebrew First

**To enable Homebrew distribution now:**

1. Create `Tracekit-Dev/homebrew-tap` repository on GitHub
2. Get SHA256 checksums from v1.0.1 release
3. Create `Formula/tracekit.rb` with checksums
4. Commit and push
5. Update README with: `brew install Tracekit-Dev/tap/tracekit`

**Want me to create the Homebrew tap repository files for you?**
