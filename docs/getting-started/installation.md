# Installation

Repro is distributed as a single static binary with zero external dependencies. Choose your preferred installation method below.

## Automated Installers

### Linux & macOS

Run the official POSIX installer script in your terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/BaimPriyatna/repro/main/install.sh | sh
```

The installer detects your OS (`Linux`, `Darwin`) and CPU architecture (`amd64`, `arm64`), downloads the latest release, verifies its SHA-256 checksum against `checksums.txt`, extracts the binary, and places it into `/usr/local/bin` (or `$HOME/.local/bin`).

To install a specific version, pass `REPRO_VERSION`:

```bash
curl -fsSL https://raw.githubusercontent.com/BaimPriyatna/repro/main/install.sh | REPRO_VERSION=v1.1.6 sh
```

### Windows

Run the installer command in PowerShell:

```powershell
irm https://raw.githubusercontent.com/BaimPriyatna/repro/main/install.ps1 | iex
```

The PowerShell installer detects your CPU architecture (`amd64`, `arm64`), downloads the appropriate `.zip` archive, verifies the SHA-256 hash using `Get-FileHash`, extracts the binary using `Expand-Archive`, and installs `repro.exe` into `$HOME\.repro\bin` without requiring Administrator privileges.

To install a specific version:

```powershell
$env:REPRO_VERSION = "v1.1.6"; irm https://raw.githubusercontent.com/BaimPriyatna/repro/main/install.ps1 | iex
```

---

## Go Install

If you have Go 1.24 or newer installed on your system:

```bash
go install github.com/BaimPriyatna/repro/cmd/repro@v1.1.6
```

Make sure your `$GOPATH/bin` or `$HOME/go/bin` is included in your system `PATH`.

---

## Manual Installation

If you prefer to download and verify release binaries manually:

1. Browse to the [GitHub Releases](https://github.com/BaimPriyatna/repro/releases) page.
2. Download the archive corresponding to your platform:
   - Linux (x86_64): `repro_linux_amd64.tar.gz`
   - Linux (ARM64): `repro_linux_arm64.tar.gz`
   - macOS (Intel): `repro_darwin_amd64.tar.gz`
   - macOS (Apple Silicon): `repro_darwin_arm64.tar.gz`
   - Windows (x86_64): `repro_windows_amd64.zip`
   - Windows (ARM64): `repro_windows_arm64.zip`
3. Download `checksums.txt` from the same release.
4. Verify the SHA-256 integrity:

   **Linux**:
   ```bash
   sha256sum -c checksums.txt --ignore-missing
   ```

   **macOS**:
   ```bash
   shasum -a 256 -c checksums.txt --ignore-missing
   ```

   **Windows (PowerShell)**:
   ```powershell
   Get-FileHash .\repro_windows_amd64.zip -Algorithm SHA256
   ```

5. Extract the archive:
   ```bash
   tar -xzf repro_linux_amd64.tar.gz
   ```
6. Move the binary into a directory present in your `PATH`:
   ```bash
   sudo mv repro /usr/local/bin/
   ```

---

## Build from Source

To compile Repro locally from source:

```bash
git clone https://github.com/BaimPriyatna/repro.git
cd repro
go mod tidy
make build
./bin/repro version
```

---

## Verification

After installation, verify that the binary is available:

```bash
repro version
```

Expected output:
```text
v1.1.6
```
