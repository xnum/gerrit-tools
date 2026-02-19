# Contributing

Thanks for contributing.

## Development Setup

1. Install Go 1.24+.
2. Clone the repository.
3. Build and test:

```bash
make deps
make build
make test
```

## Pull Request Rules

- Keep PRs focused and small.
- Include tests for behavior changes.
- Keep backward compatibility where possible.
- Update `README.md` when CLI behavior or configuration changes.
- Do not commit secrets, tokens, or local config files.

## Commit and Review Expectations

- Use clear commit messages.
- Explain why a change is needed, not only what changed.
- For risky changes, include rollback notes.

## Security Reporting

Do not open public issues for security vulnerabilities.
Follow `SECURITY.md` for private reporting instructions.
