//go:build e2e

// Package e2e runs the real oolog binary against a throwaway OpenObserve in
// Docker. Run with `make e2e`. Skips cleanly when Docker is unavailable.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	email    = "root@example.com"
	password = "Complexpass#123"
)

var images = []string{"public.ecr.aws/zinclabs/openobserve:latest", "openobserve/openobserve:latest"}

type env struct {
	base string // http://127.0.0.1:port
	bin  string
	home string
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func startOpenObserve(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not installed")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not available")
	}
	port := freePort(t)
	name := fmt.Sprintf("oolog-e2e-%d", time.Now().UnixNano())
	var lastOut []byte
	started := false
	for _, img := range images {
		out, err := exec.Command("docker", "run", "-d", "--name", name,
			"-p", fmt.Sprintf("127.0.0.1:%d:5080", port),
			"-e", "ZO_ROOT_USER_EMAIL="+email, "-e", "ZO_ROOT_USER_PASSWORD="+password,
			"-e", "ZO_DATA_DIR=/data", img).CombinedOutput()
		if err == nil {
			started = true
			break
		}
		lastOut = out
	}
	if !started {
		t.Skipf("could not start OpenObserve container: %s", lastOut)
	}
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return base
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	logs, _ := exec.Command("docker", "logs", "--tail", "30", name).CombinedOutput()
	t.Fatalf("OpenObserve did not become healthy:\n%s", logs)
	return ""
}

func apiDo(t *testing.T, method, url string, body []byte) []byte {
	t.Helper()
	req, _ := http.NewRequest(method, url, bytes.NewReader(body))
	req.SetBasicAuth(email, password)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		t.Fatalf("%s %s: %d %s", method, url, resp.StatusCode, b)
	}
	return b
}

func ingest(t *testing.T, base, stream string, rows []map[string]any) {
	t.Helper()
	b, _ := json.Marshal(rows)
	apiDo(t, "POST", base+"/api/default/"+stream+"/_json", b)
}

// waitCount polls until the stream holds n rows.
func waitCount(t *testing.T, base, stream string, n int) {
	t.Helper()
	q, _ := json.Marshal(map[string]any{"query": map[string]any{
		"sql":        fmt.Sprintf(`SELECT count(*) AS c FROM "%s"`, stream),
		"start_time": time.Now().Add(-time.Hour).UnixMicro(),
		"end_time":   time.Now().Add(time.Hour).UnixMicro(),
		"from":       0, "size": 1,
	}})
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest("POST", base+"/api/default/_search?type=logs", bytes.NewReader(q))
		req.SetBasicAuth(email, password)
		req.Header.Set("Content-Type", "application/json")
		if resp, err := http.DefaultClient.Do(req); err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var r struct {
				Hits []struct {
					C float64 `json:"c"`
				} `json:"hits"`
			}
			if json.Unmarshal(b, &r) == nil && len(r.Hits) == 1 && int(r.Hits[0].C) == n {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("stream %s never reached %d rows", stream, n)
}

func (e *env) run(t *testing.T, extra []string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(e.bin, args...)
	cmd.Env = append(os.Environ(), "OOLOG_HOME="+e.home, "OOLOG_CONFIG=", "OOLOG_PASSWORD=")
	cmd.Env = append(cmd.Env, extra...)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	cmd.Stdin = strings.NewReader("")
	err := cmd.Run()
	return so.String(), se.String(), err
}

// ok runs with the password in env and fails on error.
func (e *env) ok(t *testing.T, args ...string) string {
	t.Helper()
	out, errOut, err := e.run(t, []string{"OOLOG_PASSWORD=" + password}, args...)
	if err != nil {
		t.Fatalf("oolog %s: %v\nstdout: %s\nstderr: %s", strings.Join(args, " "), err, out, errOut)
	}
	return out
}

// fail runs expecting a non-zero exit; returns stderr.
func (e *env) fail(t *testing.T, extra []string, args ...string) string {
	t.Helper()
	out, errOut, err := e.run(t, extra, args...)
	if err == nil {
		t.Fatalf("oolog %s: expected failure, got stdout: %s", strings.Join(args, " "), out)
	}
	return errOut
}

func contains(t *testing.T, label, s string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(s, w) {
			t.Errorf("%s: missing %q in:\n%s", label, w, s)
		}
	}
}

func notContains(t *testing.T, label, s string, bad ...string) {
	t.Helper()
	for _, w := range bad {
		if strings.Contains(s, w) {
			t.Errorf("%s: unexpected %q in:\n%s", label, w, s)
		}
	}
}

func TestE2E(t *testing.T) {
	base := startOpenObserve(t)

	bin := filepath.Join(t.TempDir(), "oolog")
	if out, err := exec.Command("go", "build", "-o", bin, "..").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	e := &env{base: base, bin: bin, home: t.TempDir()}

	// Fixture: second-aligned timestamps, 10..1 minutes ago, oldest first.
	t0 := time.Now().Truncate(time.Second).Add(-10 * time.Minute)
	ts := func(i int) int64 { return t0.Add(time.Duration(i) * time.Second).UnixMicro() }
	k8s := []map[string]any{
		{"_timestamp": ts(-1), "kubernetes_namespace_name": "shop", "kubernetes_pod_name": "web-1", "kubernetes_container_name": "web", "log": "boot complete"},
		{"_timestamp": ts(0), "kubernetes_namespace_name": "shop", "kubernetes_pod_name": "web-1", "kubernetes_container_name": "web", "log": "GET /health 200"},
		{"_timestamp": ts(1), "kubernetes_namespace_name": "shop", "kubernetes_pod_name": "web-1", "kubernetes_container_name": "web", "log": "ERROR timeout contacting payments"},
		{"_timestamp": ts(2), "kubernetes_namespace_name": "shop", "kubernetes_pod_name": "web-2", "kubernetes_container_name": "web", "log": "GET /cart 200"},
		{"_timestamp": ts(3), "kubernetes_namespace_name": "billing", "kubernetes_pod_name": "invoicer-1", "kubernetes_container_name": "invoicer", "log": "invoice 42 created"},
		{"_timestamp": ts(4), "kubernetes_namespace_name": "billing", "kubernetes_pod_name": "invoicer-1", "kubernetes_container_name": "sidecar", "log": "ERROR timeout on ledger"},
		{"_timestamp": ts(5), "kubernetes_namespace_name": "billing", "kubernetes_pod_name": "invoicer-1", "kubernetes_container_name": "invoicer", "log": "invoice 43 created"},
	}
	app := []map[string]any{
		{"_timestamp": ts(0), "service": "checkout", "host": "node-a", "lvl": "info", "msg": "order placed"},
		{"_timestamp": ts(1), "service": "checkout", "host": "node-a", "lvl": "error", "msg": "payment declined"},
		{"_timestamp": ts(2), "service": "search", "host": "node-b", "lvl": "info", "msg": "query served"},
		{"_timestamp": ts(3), "service": "search", "host": "node-b", "lvl": "error", "msg": "index unavailable"},
	}

	ingest(t, base, "k8s", k8s)
	waitCount(t, base, "k8s", len(k8s))

	// --- config init + one-stream default resolution ---
	t.Run("config init validates and saves (password from env, not written)", func(t *testing.T) {
		out, errOut, err := e.run(t, []string{"OOLOG_PASSWORD=" + password},
			"config", "init", "--url", base, "--org", "default", "--email", email)
		if err != nil {
			t.Fatalf("config init: %v\n%s\n%s", err, out, errOut)
		}
		contains(t, "init", out, "saved config", "1 streams")
		b, _ := os.ReadFile(filepath.Join(e.home, "config.yaml"))
		notContains(t, "config.yaml", string(b), password)
	})

	t.Run("config init rejects bad credentials", func(t *testing.T) {
		stderr := e.fail(t, []string{"OOLOG_PASSWORD=wrong-pass"},
			"--config", "bad", "config", "init", "--url", base, "--org", "default", "--email", email)
		contains(t, "stderr", stderr, "validation failed")
		notContains(t, "stderr", stderr, "wrong-pass")
	})

	t.Run("single stream is the default", func(t *testing.T) {
		out := e.ok(t, "tail", "-n", "2", "--since", "1h")
		contains(t, "tail", out, "invoice 43 created", "billing/invoicer-1[invoicer]")
	})

	ingest(t, base, "app", app)
	waitCount(t, base, "app", len(app))

	t.Run("stale cache keeps working until sync, then two streams demand a choice", func(t *testing.T) {
		out := e.ok(t, "streams", "sync")
		contains(t, "sync", out, "synced 2 streams")
		stderr := e.fail(t, []string{"OOLOG_PASSWORD=" + password}, "tail", "--since", "1h")
		contains(t, "stderr", stderr, "multiple streams", "app, k8s", "stream:")
		out = e.ok(t, "tail", "--stream", "k8s", "-n", "1", "--since", "1h")
		contains(t, "tail --stream", out, "invoice 43 created")
	})

	t.Run("streams list/show", func(t *testing.T) {
		out := e.ok(t, "streams", "list")
		contains(t, "list", out, "app", "k8s")
		out = e.ok(t, "streams", "show", "k8s")
		contains(t, "show", out, "kubernetes_namespace_name", "kubernetes_pod_name", "log")
		out = e.ok(t, "streams", "show", "app")
		contains(t, "show app", out, "service", "lvl", "msg")
		stderr := e.fail(t, []string{"OOLOG_PASSWORD=" + password}, "streams", "show", "nope")
		contains(t, "show nope", stderr, "unknown stream")
	})

	// Profile for the k8s stream with config default (stream set, default fields).
	writeProfile(t, e.home, "k8sprof", fmt.Sprintf("url: %s\norg: default\nemail: %s\nstream: k8s\n", base, email))
	// Profile for the plain app stream with a custom field mapping.
	writeProfile(t, e.home, "app", fmt.Sprintf(`url: %s
org: default
email: %s
stream: app
fields:
  namespace: service
  pod: host
  level: lvl
  message: msg
`, base, email))

	t.Run("stream from config + k8s default field mapping", func(t *testing.T) {
		out := e.ok(t, "--config", "k8sprof", "tail", "--since", "1h", "--ns", "shop")
		contains(t, "tail --ns", out, "shop/web-1[web]", "GET /cart 200")
		notContains(t, "tail --ns", out, "billing")

		out = e.ok(t, "--config", "k8sprof", "tail", "--since", "1h", "--ns", "billing", "--container", "sidecar")
		contains(t, "container", out, "ERROR timeout on ledger")
		notContains(t, "container", out, "invoice 42")

		out = e.ok(t, "--config", "k8sprof", "tail", "--since", "1h", "--pod", "web-2")
		contains(t, "pod", out, "GET /cart 200")
		notContains(t, "pod", out, "web-1")

		out = e.ok(t, "--config", "k8sprof", "tail", "--since", "1h", "--level", "ERROR")
		contains(t, "level fallback (match_all)", out, "timeout contacting payments", "timeout on ledger")
		notContains(t, "level", out, "invoice 4")

		out = e.ok(t, "--config", "k8sprof", "tail", "--since", "1h", "--grep", "invoice", "--where", `"kubernetes_pod_name" = 'invoicer-1'`)
		contains(t, "grep+where", out, "invoice 42 created", "invoice 43 created")

		out = e.ok(t, "--config", "k8sprof", "tail", "-n", "2", "--since", "1h")
		if lines := nonEmpty(out); len(lines) != 2 {
			t.Errorf("tail -n 2 returned %d lines:\n%s", len(lines), out)
		}
		// newest first
		if !strings.Contains(nonEmpty(out)[0], "invoice 43") {
			t.Errorf("tail not newest-first:\n%s", out)
		}
	})

	t.Run("custom field mapping on plain stream", func(t *testing.T) {
		out := e.ok(t, "--config", "app", "tail", "--since", "1h", "--ns", "checkout")
		contains(t, "ns->service", out, "checkout/node-a", "order placed", "payment declined")
		notContains(t, "ns->service", out, "search")

		out = e.ok(t, "--config", "app", "tail", "--since", "1h", "--level", "error")
		contains(t, "level->lvl", out, "ERROR", "payment declined", "index unavailable")
		notContains(t, "level->lvl", out, "order placed")

		out = e.ok(t, "--config", "app", "tail", "--since", "1h", "--pod", "node-b")
		contains(t, "pod->host", out, "query served")
		notContains(t, "pod->host", out, "order placed")
	})

	t.Run("unmapped or absent field errors clearly", func(t *testing.T) {
		// App stream without a mapping: default k8s names do not exist in it.
		writeProfile(t, e.home, "appnomap", fmt.Sprintf("url: %s\norg: default\nemail: %s\nstream: app\n", base, email))
		stderr := e.fail(t, []string{"OOLOG_PASSWORD=" + password}, "--config", "appnomap", "tail", "--since", "1h", "--ns", "checkout")
		contains(t, "ns absent", stderr, "--ns", "kubernetes_namespace_name", `stream "app"`, "fields.namespace")
		// Mapped level field that does not exist.
		writeProfile(t, e.home, "badlvl", fmt.Sprintf("url: %s\norg: default\nemail: %s\nstream: k8s\nfields:\n  level: severity\n", base, email))
		stderr = e.fail(t, []string{"OOLOG_PASSWORD=" + password}, "--config", "badlvl", "tail", "--since", "1h", "--level", "error")
		contains(t, "level absent", stderr, "--level", "severity", "fields.level")
		// Without the flag, no error.
		e.ok(t, "--config", "appnomap", "tail", "--since", "1h")
	})

	t.Run("search --match and --fields", func(t *testing.T) {
		out := e.ok(t, "--config", "k8sprof", "search", "--since", "1h", "--match", "timeout", "--fields", "_timestamp,log", "--json")
		var rows []map[string]any
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("not JSON: %v\n%s", err, out)
		}
		if len(rows) != 2 {
			t.Fatalf("want 2 matches, got %d: %s", len(rows), out)
		}
		for _, r := range rows {
			if len(r) != 2 || r["log"] == nil || r["_timestamp"] == nil {
				t.Errorf("--fields not honored: %v", r)
			}
		}
		out = e.ok(t, "--config", "app", "search", "--since", "1h", "--match", "declined")
		contains(t, "search app", out, "payment declined")
		notContains(t, "search app", out, "order placed")
	})

	t.Run("hist", func(t *testing.T) {
		out := e.ok(t, "--config", "k8sprof", "hist", "--since", "1h", "--by", "1 hour", "--json")
		var rows []map[string]any
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("not JSON: %v\n%s", err, out)
		}
		total := 0.0
		for _, r := range rows {
			total += r["count"].(float64)
		}
		if total != 7 {
			t.Errorf("hist total = %v, want 7 (%s)", total, out)
		}
		out = e.ok(t, "--config", "k8sprof", "hist", "--since", "1h", "--by", "1 hour", "--grep", "timeout")
		contains(t, "hist table", out, "bucket", "count")
	})

	t.Run("top", func(t *testing.T) {
		out := e.ok(t, "--config", "k8sprof", "top", "kubernetes_namespace_name", "--since", "1h")
		lines := nonEmpty(out)
		if len(lines) != 3 || !strings.HasPrefix(lines[1], "shop") || !strings.HasPrefix(lines[2], "billing") {
			t.Errorf("top output wrong:\n%s", out)
		}
		out = e.ok(t, "--config", "app", "top", "lvl", "--since", "1h", "--json")
		var rows []map[string]any
		if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 2 {
			t.Fatalf("top json: %v\n%s", err, out)
		}
		if rows[0]["count"].(float64) != 2 {
			t.Errorf("unexpected counts: %s", out)
		}
	})

	t.Run("around", func(t *testing.T) {
		pivot := t0.Add(3 * time.Second).UTC().Format(time.RFC3339) // "invoice 42 created"
		out := e.ok(t, "--config", "k8sprof", "around", pivot, "--before", "2", "--after", "2")
		lines := nonEmpty(out)
		want := []string{"GET /cart 200", "invoice 42 created"} // before (chronological, includes pivot)
		for _, w := range want {
			contains(t, "around before", out, w)
		}
		contains(t, "around after", out, "ERROR timeout on ledger", "invoice 43 created")
		notContains(t, "around window", out, "GET /health 200")
		if len(lines) != 4 {
			t.Errorf("around returned %d lines, want 4:\n%s", len(lines), out)
		}
		if !strings.Contains(lines[0], "GET /cart") || !strings.Contains(lines[3], "invoice 43") {
			t.Errorf("around not chronological:\n%s", out)
		}
	})

	t.Run("query raw SQL", func(t *testing.T) {
		out := e.ok(t, "--config", "k8sprof", "query", `SELECT count(*) AS n FROM "k8s"`, "--since", "1h", "--json")
		var rows []map[string]any
		if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 1 || rows[0]["n"].(float64) != 7 {
			t.Fatalf("query: %v\n%s", err, out)
		}
		out = e.ok(t, "--config", "k8sprof", "query", `SELECT lvl, count(*) AS n FROM "app" GROUP BY lvl ORDER BY lvl`, "--since", "1h", "--format", "csv")
		contains(t, "csv", out, "lvl,n", "error,2", "info,2")
	})

	t.Run("json output shape", func(t *testing.T) {
		out := e.ok(t, "--config", "k8sprof", "tail", "-n", "1", "--since", "1h", "--json")
		var rows []map[string]any
		if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 1 {
			t.Fatalf("tail json: %v\n%s", err, out)
		}
		r := rows[0]
		if r["log"] != "invoice 43 created" || r["kubernetes_namespace_name"] != "billing" {
			t.Errorf("unexpected row: %v", r)
		}
		if _, ok := r["_timestamp"].(float64); !ok {
			t.Errorf("_timestamp should be numeric micros: %v", r["_timestamp"])
		}
		out = e.ok(t, "streams", "list", "--json")
		var sl struct {
			List []struct {
				Name   string `json:"name"`
				Fields []struct{ Name, Type string }
			} `json:"list"`
		}
		if err := json.Unmarshal([]byte(out), &sl); err != nil || len(sl.List) != 2 || len(sl.List[0].Fields) == 0 {
			t.Fatalf("streams json: %v\n%s", err, out)
		}
	})

	t.Run("password sources and redaction", func(t *testing.T) {
		// No password anywhere.
		stderr := e.fail(t, nil, "--config", "k8sprof", "tail", "--since", "1h")
		contains(t, "no password", stderr, "OOLOG_PASSWORD")
		// Wrong env password: HTTP 401, secret not echoed.
		stderr = e.fail(t, []string{"OOLOG_PASSWORD=not-the-password"}, "--config", "k8sprof", "tail", "--since", "1h")
		notContains(t, "wrong pw", stderr, "not-the-password")
		// password_command.
		writeProfile(t, e.home, "cmdpw", fmt.Sprintf("url: %s\norg: default\nemail: %s\nstream: k8s\npassword_command: printf '%%s\\n' '%s'\n", base, email, password))
		out, errOut, err := e.run(t, nil, "--config", "cmdpw", "tail", "-n", "1", "--since", "1h")
		if err != nil {
			t.Fatalf("password_command: %v\n%s", err, errOut)
		}
		contains(t, "password_command", out, "invoice 43 created")
		// Failing password_command.
		writeProfile(t, e.home, "badcmd", fmt.Sprintf("url: %s\norg: default\nemail: %s\npassword_command: exit 1\n", base, email))
		stderr = e.fail(t, nil, "--config", "badcmd", "tail", "--since", "1h")
		contains(t, "bad command", stderr, "password_command")
		// Plain yaml password still works; --verbose never prints it.
		writeProfile(t, e.home, "plain", fmt.Sprintf("url: %s\norg: default\nemail: %s\npassword: %s\nstream: k8s\n", base, email, password))
		out, errOut, err = e.run(t, nil, "--config", "plain", "--verbose", "tail", "-n", "1", "--since", "1h")
		if err != nil {
			t.Fatalf("plain pw: %v\n%s", err, errOut)
		}
		contains(t, "verbose", errOut, "POST", "200")
		notContains(t, "verbose stderr", errOut, password, "Basic ", "Authorization")
		notContains(t, "verbose stdout", out, password)
		wo := e.ok(t, "--config", "plain", "whoami")
		notContains(t, "whoami", wo, password)
	})
}

func writeProfile(t *testing.T, home, name, yaml string) {
	t.Helper()
	dir := filepath.Join(home, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
}

func nonEmpty(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
