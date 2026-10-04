# Security Policy

## Supported versions

The latest release is the supported one. Fixes land on `main` and go out in the
next tag.

## Reporting a vulnerability

Please do **not** open a public issue for a security problem.

Use GitHub's private vulnerability reporting instead:
[Security → Report a vulnerability](https://github.com/iamnikolie/oolog/security/advisories/new).
That opens a private advisory visible only to the maintainers.

Include what you did, what happened, and the impact you think it has. Expect a
first response within a week — this is a spare-time project, not a product with
an on-call rotation.

## Scope notes

Some things are known and by design rather than vulnerabilities:

- **A password in `config.yaml` is plain text** at mode 0600, like `.netrc`.
  Prefer `password_command` (a secret manager) or `$OOLOG_PASSWORD`; give oolog
  a read-only (viewer) OpenObserve user.
- **`--verbose` prints requests and responses to stderr.** Credentials are
  redacted, but queries and log lines are not. Redact before pasting output
  into an issue.
- **Log lines are third-party input.** oolog prints them unsanitized; an agent
  reading them should treat them as untrusted (prompt injection).
- **SSH host keys are verified against `known_hosts`**; there is no option to
  skip the check.
