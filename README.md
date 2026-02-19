# gerrit-tools

AI-assisted Gerrit tooling in Go:
- `gerrit-reviewer`: reviews Gerrit patchsets via Claude CLI and posts results back to Gerrit.
- `gerrit-cli`: script-friendly Gerrit CLI for querying changes, diffs, comments, drafts, and posting reviews.

## Disclaimer

This project is completely a vibe-coding artifact. Use at your own risk.

- No warranty.
- Interfaces may change without notice.
- Always validate tool output before using it in CI/CD or merge gates.

## Features

- Gerrit REST integration (query + review posting)
- Gerrit SSH event listening (`serve` mode)
- Automated reviewer worker pool
- JSON-first CLI output for automation
- Config via `config.yaml` and/or env vars
- Configurable Claude permission mode (safe by default)

## Repository Status

- Public use: supported on a best-effort basis
- Security fixes: default branch only
- License: MIT (`LICENSE`)

## Requirements

- Go 1.24+
- `git`
- `ssh` connectivity to Gerrit
- Gerrit account with required permissions
- Claude CLI in `PATH` (required for `gerrit-reviewer`)

## Installation

```bash
make deps
make build
```

Binaries:
- `./dist/gerrit-reviewer`
- `./dist/gerrit-cli`

## Configuration

Use `config.yaml.example` as template:

```bash
cp config.yaml.example config.yaml
```

Or use env vars directly.

### Required env vars

```bash
export GERRIT_SSH_ALIAS="gerrit-review"
export GERRIT_HTTP_URL="https://gerrit.example.com"
export GERRIT_HTTP_USER="your-user"
export GERRIT_HTTP_PASSWORD="your-password"
export GIT_REPO_BASE_PATH="/tmp/ai-review-repos"
```

### Review env vars

```bash
export CLAUDE_TIMEOUT=600
export CLAUDE_SKIP_PERMISSIONS=false
```

`CLAUDE_SKIP_PERMISSIONS=true` adds Claude flag `--dangerously-skip-permissions`.
Default is `false`.

## Usage

### One-shot review

```bash
./dist/gerrit-reviewer \
  --project "my/project" \
  --change-number 12345 \
  --patchset-number 3
```

### One-shot review (unsafe permission bypass)

```bash
./dist/gerrit-reviewer \
  --project "my/project" \
  --change-number 12345 \
  --patchset-number 3 \
  --dangerously-skip-permissions
```

### Serve mode

```bash
./dist/gerrit-reviewer serve
```

Serve mode also supports:

```bash
./dist/gerrit-reviewer serve --dangerously-skip-permissions
```

### gerrit-cli examples

```bash
./dist/gerrit-cli change list "status:open project:my/project" --limit 5
./dist/gerrit-cli change get 12345
./dist/gerrit-cli patchset diff 12345 --list-files
./dist/gerrit-cli comment list 12345 --unresolved
./dist/gerrit-cli draft list 12345
./dist/gerrit-cli review post 12345 --message "LGTM" --vote 1
```

## Development

```bash
make fmt
make test
make build
```

## CI

GitHub Actions CI (`.github/workflows/ci.yml`) runs:
- formatting check (`gofmt -l`)
- build
- tests
- `go vet`

## Security Notes

- Never commit credentials.
- Use least-privilege Gerrit service account.
- Keep `CLAUDE_SKIP_PERMISSIONS=false` unless you explicitly accept the risk.
- Treat model-generated reviews as untrusted suggestions until verified.

## Governance

- Contributing guide: `CONTRIBUTING.md`
- Security policy: `SECURITY.md`
- Code of conduct: `CODE_OF_CONDUCT.md`
- License: `LICENSE`
