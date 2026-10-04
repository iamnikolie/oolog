# Contributing

Thanks for taking a look. This is a small, focused CLI — bug reports and pull
requests are welcome, and so is a plain question in an issue.

## Reporting a bug

Include the output of `oolog version`, your OpenObserve version, the exact
command you ran, and what you expected instead. `--verbose` shows the SQL and
the HTTP exchange — **redact hostnames and log contents before pasting it.**
Credentials are redacted.

## Pull requests

Before opening one:

```bash
make fmt           # gofmt -w
make vet           # go vet ./...
make test          # go test -race ./...
make e2e           # real OpenObserve in Docker (skips without Docker)
```

CI runs the same on Linux and macOS (e2e on Linux), so a green local run
usually means a green PR.

House rules:

- **One concern per PR.** A bug fix and a refactor in the same diff take three
  times as long to review.
- **No deployment assumptions.** Stream names and field names come from the
  config (`stream:`, `fields:`), never from constants.
- **An e2e case for behaviour that depends on OpenObserve** (SQL it accepts,
  time-range semantics). Unit tests for pure SQL building and rendering.
- **Keep the output token-lean.** The default rendering exists so an agent can
  read it without burning context; machine output goes behind `--json`/`--format`.
- **Update the docs in the same commit.** Any change to the CLI surface must
  also update `cmd/skill.md` (embedded in the binary, printed by `oolog skill`)
  and `README.md`.
- **Conventional commit subjects** — `feat:`, `fix:`, `docs:`, `refactor:`,
  `test:`, `chore:`. Release notes are generated from them.

## Releases

Maintainer-only. Tag and push:

```bash
git tag -a v1.2.3 -m "v1.2.3"
git push origin v1.2.3
```

GoReleaser builds archives for linux/darwin/windows on amd64 and arm64 and
publishes the GitHub release with a generated changelog.
