# oolog — OpenObserve log CLI

Query an OpenObserve log store from the shell. Read stdout directly.

## Setup
- `oolog config init` — URL/org/email/password (+ optional default stream) into ~/.oolog/config.yaml.
- Password may come from `$OOLOG_PASSWORD` or `password_command:` (stdout) instead of the file.
- No public URL? An `ssh:` block tunnels in-process. Set exactly one of
  `target: host:port` (reachable from the SSH host) or `service: ns/name` + `service_port`
  (Kubernetes ClusterIP via kubectl on the node; `kubeconfig` default is k3s's). Auth: `key:` then ssh-agent; a protected key needs `ssh-add` or `$OOLOG_SSH_PASSPHRASE`. An `ssh:` error
  means key/known_hosts/host trouble, not the query.
- `oolog streams sync` — refresh the cached stream list/schema; `streams list|show <stream>`.

## Commands
| Command | Use |
|---|---|
| `oolog tail -n 50 --since 15m --ns NS --pod POD --container C --level L --grep TEXT` | Most recent lines, newest first. |
| `oolog search --since 1h --match timeout --fields _timestamp,message --limit 100` | Filtered search. |
| `oolog hist --since 1h --by '1 minute' --grep error` | Time-bucketed counts (spikes). |
| `oolog top <field> -n 20 --since 1h` | Most frequent values of a field. |
| `oolog around 2026-06-22T09:50:01Z --before 5 --after 5` | Context around a moment (RFC3339). |
| `oolog query 'SELECT count(*) FROM "mystream"' --since 1h` | Raw SQL. |
| `oolog exec '{"query":{...}}'` | Raw search body. |
| `oolog streams list\|show <stream>\|sync` | Schema introspection. |
| `oolog whoami` | Verify auth, show account. |

## Streams
Stream = `--stream`, else `stream:` in config, else the only stream if exactly one, else an
error listing the streams. Start with `oolog streams list`.

## Filter flags (shared by tail/search/hist/top/around)
- `--ns` / `--pod` / `--container` — equality on the fields mapped in config `fields:`
  (`namespace`/`pod`/`container`; defaults `kubernetes_namespace_name`, `kubernetes_pod_name`,
  `kubernetes_container_name`). If the field is not in the stream, the command errors and names
  the config key to set: check `oolog streams show <stream>` and fix `fields:`.
- `--level TEXT` — equality on `fields.level` if mapped, else full-text `match_all('TEXT')`.
- `--grep TEXT` (and `--match` on search) — full-text `match_all('TEXT')`.
- `--where "<raw SQL>"` — arbitrary WHERE condition, ANDed with the above.
- The displayed line uses `fields.message` first, then `message`, `log`, `msg`, `body`.

## Notes
- Default account is `default` (switch with `--config <name>` or `OOLOG_CONFIG`).
- Time is relative (`15m`, `2h`, `3d`) via `--since`, or RFC3339 via `--from/--to`.
- `--json` / `--format csv|tsv` change output; default is one concise line per hit.
- `--verbose` prints HTTP requests/responses to stderr (credentials are never printed).
