# Installing Go on WSL

Use these steps to install the Go toolchain inside WSL (Ubuntu/Debian).

## If `apt install golang-go` didn’t put `go` on your PATH

The Ubuntu package sometimes installs the binary in a place that’s not in your PATH (e.g. in a versioned path). Check and fix:

```bash
# See where the package put files
dpkg -L golang-go | grep -E 'bin/go$'

# If you see something like /usr/lib/go-1.18/bin/go, add that dir to PATH:
export PATH=$PATH:/usr/lib/go-1.18/bin
# Add the same line to ~/.profile or ~/.bashrc so it’s permanent
```

If no `go` binary is listed, use the **official binary** method below.

---

## Option 1: Official binary (recommended)

This gives you a recent Go and a predictable path: `/usr/local/go/bin/go`.

1. **Download** the latest Linux tarball from [go.dev/dl](https://go.dev/dl/).  
   Choose **Linux** and **amd64** (or **arm64** on ARM). File will be like `go1.22.0.linux-amd64.tar.gz`.

2. **Install** (replace the filename with yours; use your actual download path):

   ```bash
   rm -rf /usr/local/go
   sudo tar -C /usr/local -xzf ~/Downloads/go1.22.0.linux-amd64.tar.gz
   ```

3. **Add Go to PATH** — put this in `~/.profile` or `~/.zshrc` (since you use zsh):

   ```bash
   export PATH=$PATH:/usr/local/go/bin
   ```

   Then reload:

   ```bash
   source ~/.zshrc
   ```
   (or open a new terminal)

4. **Verify:**

   ```bash
   go version
   ```

   You should see something like `go version go1.22.0 linux/amd64`.

---

## Option 2: apt (simpler, often older)

```bash
sudo apt update
sudo apt install golang-go
```

If `go version` then says “command not found”, run:

```bash
dpkg -L golang-go | grep -E 'bin/go$'
```

and add the directory that contains `go` to your PATH (see top of this doc). Or remove the package and use Option 1:

```bash
sudo apt remove golang-go
# then follow Option 1
```

---

## GOBIN and GOPATH (optional)

- **GOPATH** defaults to `~/go`. You can leave it at that.
- **GOBIN** defaults to `$GOPATH/bin`. Put that on your PATH so `go install ./cmd/...` puts binaries somewhere you can run:

  ```bash
  export PATH=$PATH:$(go env GOPATH)/bin
  ```

  Add that to `~/.zshrc` (or `~/.profile`) if you want it in every shell.

---

## After installing Go

From the GDQL repo:

```bash
cd ~/git/gdql
go mod tidy
go build ./...
go test ./...
go install ./cmd/...
```

Then run `gdql 'SHOWS FROM 1977;'` (if `$GOBIN` is on your PATH).
