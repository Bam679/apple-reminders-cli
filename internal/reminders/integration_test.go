//go:build integration && darwin

package reminders

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Bam679/apple-reminders-cli/internal/agentcli"
)

// The integration test only ever writes to a freshly created list named
// agent-cli-test-<random>, enforced twice: by the policy below and by
// checking the list name before every write. The list is force-removed in
// cleanup even if assertions fail.
func TestIntegrationSandboxList(t *testing.T) {
	ctx := context.Background()
	b := NewJXA()
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	name := "agent-cli-test-" + hex.EncodeToString(buf)
	pol := agentcli.Effective{Layers: []agentcli.Policy{{Version: 1,
		Operations: agentcli.OpRule{Allow: []string{agentcli.OpRead, agentcli.OpWrite}},
		Scopes: agentcli.ScopeRules{Read: agentcli.Rule{Allow: []string{name}},
			Write: agentcli.Rule{Allow: []string{name}}}}}}
	cli := func(stdin string, args ...string) (int, string) {
		app := NewApp(b, "it")
		app.LoadPolicy = func() (agentcli.Effective, error) { return pol, nil }
		var out, errb bytes.Buffer
		code := app.Main(ctx, args, strings.NewReader(stdin), &out, &errb)
		if code != 0 {
			t.Logf("%v -> %d %s", args, code, errb.String())
		}
		return code, out.String()
	}

	if code, _ := cli("", "list-create", name, "--confirm"); code != 0 {
		t.Fatal("list-create failed")
	}
	t.Cleanup(func() {
		ls, _ := b.Lists(ctx)
		for _, l := range ls {
			if l.Title == name {
				if err := b.DeleteList(ctx, l.ID, true); err != nil {
					t.Errorf("cleanup: %v", err)
				}
			}
		}
	})

	in := `{"list":"` + name + `","title":"it-one","due":"tomorrow","priority":"high"}
{"list":"` + name + `","title":"it-two \"quoted\" 'x' $(id)","notes":"line1\nline2"}`
	code, out := cli(in, "add", "-", "--confirm", "--json")
	if code != 0 {
		t.Fatal("add failed")
	}
	var added struct{ Items []map[string]any }
	if err := json.Unmarshal([]byte(out), &added); err != nil || len(added.Items) != 2 {
		t.Fatalf("add output: %s", out)
	}
	id := added.Items[1]["id"].(string)
	if added.Items[1]["title"] != `it-two "quoted" 'x' $(id)` {
		t.Fatalf("title not preserved: %v", added.Items[1]["title"])
	}

	code, out = cli("", "list", "--list", name, "--json")
	if code != 0 || !strings.Contains(out, "it-one") || !strings.Contains(out, "it-two") {
		t.Fatalf("list: %s", out)
	}
	if code, out = cli("", "search", "IT-TWO", "--list", name); code != 0 || !strings.Contains(out, id) {
		t.Fatalf("search: %s", out)
	}
	if code, out = cli("", "get", id, "--json"); code != 0 || !strings.Contains(out, "line1\\nline2") {
		t.Fatalf("get: %s", out)
	}
	if code, _ = cli("", "update", id, "--title", "it-renamed", "--due", "2030-01-02T10:00", "--confirm"); code != 0 {
		t.Fatal("update")
	}
	r, err := b.Get(ctx, id)
	if err != nil || r.Title != "it-renamed" || r.Due.Year() != 2030 || r.DueAllDay {
		t.Fatalf("after update: %+v %v", r, err)
	}
	if code, _ = cli("", "complete", id, "--confirm"); code != 0 {
		t.Fatal("complete")
	}
	if code, out = cli("", "list", "--list", name, "--status", "completed"); code != 0 || !strings.Contains(out, id) {
		t.Fatalf("completed list: %s", out)
	}
	if code, _ = cli("", "list-delete", name, "--confirm"); code != agentcli.ExitConflict {
		t.Fatalf("non-empty list-delete must conflict, got %d", code)
	}

	// A policy-denied write against the real bridge must not reach EventKit.
	pol = agentcli.Effective{Layers: []agentcli.Policy{agentcli.DefaultPolicy()}}
	if code, _ = cli("", "delete", id, "--confirm"); code != agentcli.ExitPolicy {
		t.Fatalf("denied delete: %d", code)
	}
	if _, err := b.Get(ctx, id); err != nil {
		t.Fatal("denied delete removed the reminder")
	}
	pol = agentcli.Effective{Layers: []agentcli.Policy{{Version: 1,
		Operations: agentcli.OpRule{Allow: []string{agentcli.OpRead, agentcli.OpWrite}},
		Scopes: agentcli.ScopeRules{Read: agentcli.Rule{Allow: []string{name}},
			Write: agentcli.Rule{Allow: []string{name}}}}}}

	var ids []string
	_, out = cli("", "list", "--list", name, "--status", "all", "--json")
	var all struct{ Items []map[string]any }
	_ = json.Unmarshal([]byte(out), &all)
	for _, it := range all.Items {
		ids = append(ids, it["id"].(string))
	}
	if code, _ = cli("", append([]string{"delete", "--confirm"}, ids...)...); code != 0 {
		t.Fatal("delete")
	}
	time.Sleep(200 * time.Millisecond)
	if code, _ = cli("", "list-delete", name, "--confirm"); code != 0 {
		t.Fatal("list-delete of empty sandbox list")
	}
}
