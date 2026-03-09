# Releasing the CLI (Beta)

This describes how to build the CLI as static binaries and publish them so users can install via `install.sh` from GitHub assets.

## 1. Build static binaries

From the repo root (requires Go):

```bash
make build-release
```

This produces under `dist/`:

- `cicd-linux-amd64`  (Ubuntu 24.04 x86_64)
- `cicd-linux-arm64`
- `cicd-darwin-amd64`
- `cicd-darwin-arm64`

Binaries are statically linked (`CGO_ENABLED=0`), so no extra runtimes (JVM, Python, etc.) are needed.

## 2. Create a GitHub Release and upload assets

1. On GitHub: **Releases** → **Create a new release**.
2. Choose or create a **tag** (e.g. `v0.1.0`). Tag must match what users pass to `install.sh` (e.g. `v0.1.0`).
3. Upload the four files from `dist/` as **Release assets** (keep the names exactly: `cicd-linux-amd64`, `cicd-linux-arm64`, `cicd-darwin-amd64`, `cicd-darwin-arm64`).
4. Publish the release.

## 3. How users install

**With a specific version:**

```bash
curl -sSL https://raw.githubusercontent.com/CS7580-SEA-SP26/e-team/main/scripts/install.sh | sh -s v0.1.0
```

Or download the script and run:

```bash
./install.sh v0.1.0
```

**Latest release (script fetches latest tag from GitHub API):**

```bash
curl -sSL https://raw.githubusercontent.com/CS7580-SEA-SP26/e-team/main/scripts/install.sh | sh
```

The script installs the binary to `$HOME/bin` (or `$CICD_BIN` / `$PREFIX/bin` if set) and prints how to add it to PATH if needed.

## 4. Optional: checksums

For extra verification you can add a `sha256sum.txt` (or similar) to the release with checksums of the four binaries. The install script does not verify checksums by default; that can be added later if required.
