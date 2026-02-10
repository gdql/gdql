# How to build GDQL and the HTTP server (sandbox)

## Summary

| Step | Command |
|------|--------|
| **Build gdql** | `cd gdql && go build -o gdql ./cmd/gdql` |
| **Run gdql** | `./gdql "SHOWS FROM 1969 LIMIT 5"` (no `-db` = embedded default in config dir; use `-db <path>` to override) |
| **Build sandbox API** | `cd sandbox && go build -o sandbox-api ./cmd/sandbox-api` (needs `../gdql` or `replace` in go.mod) |
| **Run sandbox API** | `RUN_HTTP_SERVER=1 ./sandbox-api` |
| **Frontend** | `cd sandbox/web && npm install && npm run dev` |
| **Regenerate embedded DB** | `cd gdql && go run ./cmd/build_embed_db` then rebuild gdql |
| **Tests** | `cd gdql && go test ./...` |
| **Reproducible build+test** | `docker build -t gdql .` (runs `go test` + `gdql -f query.gdql` inside image) |
| **CI** | Push to `main`/`master` or open a PR → GitHub Actions runs build and the same query |

---

## Reproducible build and test (Docker / CI)

To build and test in a fixed environment (same as CI), use Docker or rely on GitHub Actions.

**Docker** (requires Docker installed):

```bash
cd /path/to/gdql
docker build -t gdql .
# Build runs: go build, go test, creates query.gdql with ASCII, runs ./gdql -f query.gdql
# If any step fails, the build fails. On success:
docker run --rm gdql   # runs the query again
```

Or use the script: `./scripts/docker-build-and-test.sh` (from repo root).

**GitHub Actions**: On every push to `main`/`master` and on pull requests, the workflow **Build and test** (`.github/workflows/build-and-test.yml`) runs:

1. `go test ./...`
2. `go build -o gdql ./cmd/gdql`
3. Creates `query.gdql` with: `SHOWS FROM 1969 WHERE PLAYED "St Stephen" > "The Eleven";`
4. `./gdql -f query.gdql`

If the query fails (e.g. parse error at column 44), the workflow fails and the log shows the exact error. Use this to verify fixes without running Windows locally.

---

## 1. GDQL (CLI and engine)

**Repo:** `gdql` (this repo)

### Build

```bash
cd /path/to/gdql
go mod tidy
go build -o gdql ./cmd/gdql
```

- **Install to `$GOBIN`:** `go install ./cmd/gdql`
- **Windows:** `go build -o gdql.exe ./cmd/gdql`

### Run

No `-db` = always use the **embedded default** (unpacked to config dir, e.g. `~/.config/gdql/shows.db`). Use `-db <path>` to point at a different database.

```bash
# No -db: uses embedded default
./gdql "SHOWS FROM 1969 LIMIT 5"
./gdql "SHOWS FROM 1969 WHERE PLAYED \"St Stephen\";"

# Override with a specific DB
./gdql -db shows.db "SHOWS FROM 1977 LIMIT 5"

# From file
./gdql -f query.gdql
```

### Regenerate embedded default DB

After changing schema or seed data:

```bash
go run ./cmd/build_embed_db
# Optionally embed a full DB: go run ./cmd/build_embed_db --from full.db
```

Then rebuild: `go build -o gdql ./cmd/gdql`.

### Tests

```bash
go test ./...
go test -v ./test/acceptance/
```

---

## 2. Sandbox HTTP server (API + frontend)

**Repo:** `sandbox` (sibling of `gdql`, or path set in `replace`)

The sandbox exposes `POST /api/query` and runs the GDQL engine with an embedded DB. It can run as a local HTTP server or as AWS Lambda.

### Dependencies

- **gdql** must be on the Go module path. Sandbox uses a `replace` in `go.mod`:

  ```go
  replace github.com/gdql/gdql => ../gdql
  ```

  So from the sandbox repo, `../gdql` must be the gdql repo. If your layout differs, change the `replace` path (e.g. `=> /home/sam/git/gdql`).

### Build API (Go server)

```bash
cd /path/to/sandbox
go mod tidy
go build -o sandbox-api ./cmd/sandbox-api
```

### Run API locally

```bash
# Run HTTP server (no Lambda)
RUN_HTTP_SERVER=1 ./sandbox-api
# Or: AWS_LAMBDA_RUNTIME_API= ./sandbox-api

# Optional: set port (default may be 8080 or from env)
PORT=8080 RUN_HTTP_SERVER=1 ./sandbox-api
```

Then:

```bash
curl -X POST http://localhost:8080/api/query -H "Content-Type: application/json" -d '{"query":"SHOWS FROM 1969 WHERE PLAYED \"St Stephen\" LIMIT 5"}'
```

### Build frontend (Svelte)

```bash
cd /path/to/sandbox/web
npm install
npm run build
# Dev: npm run dev
```

---

## 3. Recommended order when developing both

1. **Build gdql**  
   `cd gdql && go build -o gdql ./cmd/gdql`

2. **Smoke-test gdql**  
   `./gdql "SHOWS FROM 1969 WHERE PLAYED \"St Stephen\";"` (no `-db` = embedded default)

3. **Build sandbox API** (uses gdql via `replace`)  
   `cd sandbox && go build -o sandbox-api ./cmd/sandbox-api`

4. **Run sandbox API**  
   `RUN_HTTP_SERVER=1 ./sandbox-api`

5. **Build/run frontend** (optional)  
   `cd sandbox/web && npm run dev`

---

## 4. Example queries (now supported)

- `SHOWS FROM 1969 WHERE PLAYED "St Stephen";`
- `SHOWS FROM 1969 WHERE PLAYED "St Stephen" > "The Eleven";`

Song names resolve via `song_aliases` (e.g. "St Stephen" → "St. Stephen"). Use `gdql import aliases data/song_aliases.json` if your DB doesn’t have aliases loaded.
