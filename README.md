# oolog

A command-line client for [OpenObserve](https://openobserve.ai) logs. OpenObserve
ships no CLI of its own; `oolog` fills that gap: tail, search, histogram,
top-N, context-around-a-line and raw SQL, from the shell, with output that is
easy for both humans and LLM agents to read (`--json`, `--format csv|tsv` for
machines).

- Works with any OpenObserve instance (HTTP basic auth: email + password/passcode).
- Optional built-in SSH tunnel for instances with no public ingress.
- Field names are configurable, so it is not tied to Kubernetes log layouts.
- Single static binary, no runtime dependencies.

## Install

**Homebrew:**

```bash
brew trust iamnikolie/tap   # Homebrew 6 refuses untrusted third-party taps
brew tap iamnikolie/tap
brew install iamnikolie/tap/oolog
```

**Prebuilt binary** from [Releases](https://github.com/iamnikolie/oolog/releases)
(macOS, Linux, Windows; amd64/arm64), or **from source**:

```bash
go install github.com/iamnikolie/oolog@latest
# or from a checkout:
make install        # builds ./oolog and symlinks ~/.local/bin/oolog (PREFIX=... to change)
```

## Configure

```bash
oolog config init
```

Prompts for URL (default `http://localhost:5080`), org (`default`), email and
password, validates them by listing streams, and optionally records a default
stream. Config is `~/.oolog/config.yaml` (mode 0600). For several instances use
named profiles: `oolog --config prod ...` (or `OOLOG_CONFIG=prod`) reads
`~/.oolog/prod/config.yaml`. `OOLOG_HOME` relocates `~/.oolog`.

Full schema (everything except `url`/`org`/`email` is optional):

```yaml
url: https://logs.example.com
org: default
email: reader@example.com

# Password, in order of precedence: $OOLOG_PASSWORD, password_command, password.
password_command: pass show oolog/example     # stdout is the password
# password: "..."                             # works, but keeps a secret on disk

stream: app          # default stream; --stream overrides

fields:              # map the shared filter flags to your field names
  namespace: kubernetes_namespace_name   # --ns        (these three are the defaults)
  pod: kubernetes_pod_name               # --pod
  container: kubernetes_container_name   # --container
  level: level                           # --level; unset = full-text match
  message: message                       # preferred field for the displayed line

ssh:                 # optional, see below
  host: 203.0.113.10
  user: deploy
  key: ~/.ssh/id_ed25519
  target: 127.0.0.1:5080
```

### Secrets

Keep the password out of the file: export `OOLOG_PASSWORD`, or set
`password_command` to anything that prints it (`op read ...`, `pass show ...`,
`security find-generic-password -w ...`). The password is never printed;
`--verbose` logs requests and responses but not credentials. A read-only
(viewer) OpenObserve user is recommended.

### Default stream

Resolved in this order: `--stream`, `stream:` in config, the only stream on the
instance if there is exactly one, otherwise an error listing the streams.
`oolog streams sync` refreshes the cached list (`oolog streams list` shows it).

### Field mapping

`--ns`, `--pod`, `--container` and `--level` filter by equality on the mapped
field; the displayed line uses the same mapping (`message` is tried first, then
`message`, `log`, `msg`, `body`). Defaults are the OpenObserve Kubernetes
collector names. If a flag is used and its field does not exist in the stream's
schema, oolog fails with a message naming the field and the config key to set,
instead of returning an empty result. `--level` with no `fields.level` is a
full-text `match_all(...)` on the term.

Example for a plain app stream with fields `service`, `host`, `lvl`, `msg`:

```yaml
stream: app
fields: {namespace: service, pod: host, level: lvl, message: msg}
```

## Connecting

### Direct

Without an `ssh` block, oolog talks straight to `url` (`https://logs.example.com`,
or `http://localhost:5080` for a local instance or a port you forwarded yourself).

### Through SSH

For an instance that is not exposed publicly. oolog opens the SSH connection on
the first request and sends everything through it; nothing to start beforehand,
no local port. `url` is only used for the Host header. Authentication works like plain
`ssh`: the `key:` file, then any keys in ssh-agent (`$SSH_AUTH_SOCK`). A
passphrase-protected key works after `ssh-add`, or with `$OOLOG_SSH_PASSPHRASE`;
`key:` can be omitted when the agent holds it. The host must be in
`~/.ssh/known_hosts` (or set `known_hosts:`).
Exactly one of `target` or `service` must be set.

Plain mode: forward to a fixed `host:port` reachable from the SSH host, for
example OpenObserve in Docker on a VPS:

```yaml
url: http://openobserve:5080
org: default
email: reader@example.com
ssh:
  host: 203.0.113.10
  user: deploy
  key: ~/.ssh/id_ed25519
  target: 127.0.0.1:5080
```

Kubernetes mode: resolve a Service's ClusterIP with `kubectl` on the node
(re-resolved on every run):

```yaml
url: http://openobserve.openobserve.svc:5080
org: default
email: reader@example.com
ssh:
  host: 203.0.113.10
  user: root
  key: ~/.ssh/id_ed25519
  service: openobserve/openobserve     # <namespace>/<name>
  service_port: 5080
  # kubeconfig: /etc/rancher/k3s/k3s.yaml   # node-side path; this is k3s's default, change for other distros
```

An error starting with `ssh:` is a key, `known_hosts` or host problem, not a
query problem.

## Use

```bash
oolog tail -n 50 --since 15m --ns shop --level error
oolog search --since 1h --match timeout --fields _timestamp,log
oolog hist --since 2h --by '1 minute' --grep error
oolog top kubernetes_namespace_name --since 1h
oolog around 2026-06-22T09:50:01Z --before 5 --after 5
oolog query 'SELECT count(*) FROM "app"' --since 1h
oolog streams list | show <stream> | sync
oolog whoami                    # verify auth and connectivity
```

All read commands take `--stream`, a time window (`--since 15m|2h|3d`, or
`--from/--to` RFC3339) and the shared filters `--ns --pod --container --level
--grep --where` (`--where` is a raw SQL condition; `search` also has `--match`).
Output is one concise line per hit by default; `--json` or `--format csv|tsv`
for machine use.

`oolog skill` prints a compact command reference intended to be pasted into an
AI agent's instructions.

## Development

```bash
make check    # gofmt + vet + unit tests (race)
make e2e      # real OpenObserve in Docker; skips if Docker is unavailable
make build
```

`make e2e` starts `public.ecr.aws/zinclabs/openobserve` (falling back to Docker
Hub) on a random port, ingests a small fixture into two streams and drives the
built binary against it.

## License

MIT. Not affiliated with OpenObserve.
