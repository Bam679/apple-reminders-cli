package reminders

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Bam679/apple-reminders-cli/internal/agentcli"
)

var update = flag.Bool("update", false, "rewrite golden files")

// TestMain pins the zone so golden files match on every machine.
func TestMain(m *testing.M) {
	time.Local = time.UTC
	os.Exit(m.Run())
}

// fakeBridge is an in-memory Bridge. Fixture data is synthetic only.
type fakeBridge struct {
	lists []List
	rems  []Reminder
	next  int
	fail  error
}

func newFake() *fakeBridge {
	due := time.Date(2026, 12, 1, 0, 0, 0, 0, time.Local)
	at := time.Date(2026, 11, 21, 9, 30, 0, 0, time.Local)
	return &fakeBridge{
		lists: []List{
			{ID: "L1", Title: "Shopping", Account: "iCloud", Writable: true},
			{ID: "L2", Title: "Agent Sandbox", Account: "iCloud", Writable: true},
			{ID: "L3", Title: "Private", Account: "iCloud", Writable: true},
			{ID: "L4", Title: "Dup", Account: "iCloud"}, {ID: "L5", Title: "Dup", Account: "Local"},
		},
		rems: []Reminder{
			{ID: "R1", Title: "Buy milk", List: "Shopping", ListID: "L1", Due: due, DueAllDay: true, Priority: 1},
			{ID: "R2", Title: "Call plumber", List: "Agent Sandbox", ListID: "L2", Due: at, Notes: "ask about\nleak"},
			{ID: "R3", Title: "Diary", List: "Private", ListID: "L3"},
			{ID: "R4", Title: "Old task", List: "Shopping", ListID: "L1", Completed: true},
		},
	}
}

func (f *fakeBridge) Lists(context.Context) ([]List, error) {
	return append([]List{}, f.lists...), f.fail
}

func (f *fakeBridge) Reminders(_ context.Context, flt Filter) ([]Reminder, error) {
	var out []Reminder
	for _, r := range f.rems {
		if len(flt.ListIDs) > 0 && !agentcliContains(flt.ListIDs, r.ListID) {
			continue
		}
		if (flt.Status == "incomplete" && r.Completed) || (flt.Status == "completed" && !r.Completed) {
			continue
		}
		out = append(out, r)
	}
	return out, f.fail
}

func agentcliContains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func (f *fakeBridge) idx(id string) int {
	for i, r := range f.rems {
		if r.ID == id {
			return i
		}
	}
	return -1
}

func (f *fakeBridge) Get(_ context.Context, id string) (Reminder, error) {
	if i := f.idx(id); i >= 0 {
		return f.rems[i], nil
	}
	return Reminder{}, agentcli.NotFoundf("no reminder with id %s", id)
}

func (f *fakeBridge) Add(_ context.Context, n NewReminder) (Reminder, error) {
	if f.fail != nil {
		return Reminder{}, f.fail
	}
	f.next++
	var title string
	for _, l := range f.lists {
		if l.ID == n.ListID {
			title = l.Title
		}
	}
	r := Reminder{ID: fmt.Sprintf("N%d", f.next), Title: n.Title, List: title, ListID: n.ListID, Notes: n.Notes,
		URL: n.URL, Priority: n.Priority, Due: n.Due, DueAllDay: n.DueAllDay}
	f.rems = append(f.rems, r)
	return r, nil
}

func (f *fakeBridge) Update(_ context.Context, id string, p Patch) (Reminder, error) {
	i := f.idx(id)
	r := &f.rems[i]
	if p.Title != nil {
		r.Title = *p.Title
	}
	if p.Notes != nil {
		r.Notes = *p.Notes
	}
	if p.Priority != nil {
		r.Priority = *p.Priority
	}
	if p.ClearDue {
		r.Due = time.Time{}
	} else if !p.Due.IsZero() {
		r.Due, r.DueAllDay = p.Due, p.DueAllDay
	}
	return *r, nil
}

func (f *fakeBridge) SetCompleted(_ context.Context, id string, done bool) (Reminder, error) {
	if f.fail != nil {
		return Reminder{}, f.fail
	}
	i := f.idx(id)
	f.rems[i].Completed = done
	return f.rems[i], nil
}

func (f *fakeBridge) Delete(_ context.Context, id string) error {
	i := f.idx(id)
	f.rems = append(f.rems[:i], f.rems[i+1:]...)
	return nil
}

func (f *fakeBridge) CreateList(_ context.Context, title string) (List, error) {
	l := List{ID: "L" + title, Title: title, Account: "iCloud", Writable: true}
	f.lists = append(f.lists, l)
	return l, nil
}

func (f *fakeBridge) DeleteList(_ context.Context, id string, force bool) error {
	for _, r := range f.rems {
		if r.ListID == id && !force {
			return agentcli.Conflictf("list is not empty")
		}
	}
	for i, l := range f.lists {
		if l.ID == id {
			f.lists = append(f.lists[:i], f.lists[i+1:]...)
		}
	}
	return nil
}

var (
	readOnly  = agentcli.Effective{Layers: []agentcli.Policy{agentcli.DefaultPolicy()}}
	sandboxRW = agentcli.Effective{Layers: []agentcli.Policy{{Version: 1,
		Operations: agentcli.OpRule{Allow: []string{agentcli.OpRead, agentcli.OpWrite}},
		Scopes: agentcli.ScopeRules{Read: agentcli.Rule{Allow: []string{"*"}, Deny: []string{"Private"}},
			Write: agentcli.Rule{Allow: []string{"Agent*"}}}}}}
)

func run(t *testing.T, b *fakeBridge, pol agentcli.Effective, stdin string, args ...string) (int, string, string) {
	t.Helper()
	app := NewApp(b, "test")
	app.LoadPolicy = func() (agentcli.Effective, error) { return pol, nil }
	app.Now = func() time.Time { return time.Date(2026, 11, 20, 12, 0, 0, 0, time.Local) }
	var out, errb bytes.Buffer
	code := app.Main(context.Background(), args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	p := filepath.Join("testdata", "golden", name)
	if *update {
		_ = os.MkdirAll(filepath.Dir(p), 0o750)
		if err := os.WriteFile(p, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(p) // #nosec G304 -- test fixture
	if err != nil {
		t.Fatalf("missing golden %s: run go test -update", p)
	}
	if got != string(want) {
		t.Fatalf("golden %s mismatch:\n%s\nwant:\n%s", name, got, want)
	}
}

func TestReadCommandsGolden(t *testing.T) {
	cases := map[string][]string{
		"lists.txt":        {"lists"},
		"list.txt":         {"list"},
		"list_all.json":    {"list", "--status", "all", "--json"},
		"list_due.txt":     {"list", "--due-before", "2026-11-30"},
		"list_shop.ndjson": {"list", "--list", "Shopping", "--ndjson", "--fields", "id,title,priority"},
		"search.txt":       {"search", "PLUMB"},
		"get.txt":          {"get", "R2"},
		"doctor.txt":       {"doctor"},
	}
	for name, args := range cases {
		code, out, errOut := run(t, newFake(), sandboxRW, "", args...)
		if code != 0 {
			t.Fatalf("%s: exit %d %s", name, code, errOut)
		}
		golden(t, name, out)
	}
}

func TestReadScopeHidesPrivate(t *testing.T) {
	for _, args := range [][]string{{"lists"}, {"list", "--status", "all"}, {"search", "diary"}} {
		_, out, _ := run(t, newFake(), sandboxRW, "", args...)
		if strings.Contains(out, "Diary") || strings.Contains(out, "Private") {
			t.Fatalf("%v leaked private scope: %s", args, out)
		}
	}
	if code, _, _ := run(t, newFake(), sandboxRW, "", "get", "R3"); code != agentcli.ExitPolicy {
		t.Fatalf("get private: %d", code)
	}
	if code, _, _ := run(t, newFake(), sandboxRW, "", "list", "--list", "Private"); code != agentcli.ExitPolicy {
		t.Fatalf("list private: %d", code)
	}
}

func TestPolicyDeniedWriteIsBlocked(t *testing.T) {
	b := newFake()
	before := len(b.rems)
	for _, args := range [][]string{
		{"add", "--list", "Agent Sandbox", "--title", "x", "--confirm"},
		{"add", "--list", "Agent Sandbox", "--title", "x", "--dry-run"},
		{"delete", "R2", "--confirm"},
		{"complete", "R2", "--confirm"},
	} {
		code, _, errOut := run(t, b, readOnly, "", args...)
		if code != agentcli.ExitPolicy || !strings.Contains(errOut, "policy_denied") {
			t.Fatalf("%v: exit %d %s", args, code, errOut)
		}
	}
	// Scope outside write allowlist is denied even with a write-enabled policy.
	if code, _, _ := run(t, b, sandboxRW, "", "delete", "R1", "--confirm"); code != agentcli.ExitPolicy {
		t.Fatalf("delete in Shopping: %d", code)
	}
	if len(b.rems) != before {
		t.Fatal("a denied write changed data")
	}
}

func TestWriteFlow(t *testing.T) {
	b := newFake()
	code, out, _ := run(t, b, sandboxRW, "", "add", "--list", "Agent Sandbox", "--title", "New", "--due", "tomorrow", "--priority", "high", "--dry-run")
	if code != 0 || !strings.Contains(out, "true") || len(b.rems) != 4 {
		t.Fatalf("dry-run: %d %s", code, out)
	}
	if code, _, _ := run(t, b, sandboxRW, "", "add", "--list", "Agent Sandbox", "--title", "New"); code != agentcli.ExitConfirm {
		t.Fatalf("missing confirm: %d", code)
	}
	code, out, errOut := run(t, b, sandboxRW, "", "add", "--list", "Agent Sandbox", "--title", "New", "--due", "2026-12-24T18:00", "--url", "https://example.com", "--confirm", "--json")
	if code != 0 || len(b.rems) != 5 {
		t.Fatalf("add: %d %s %s", code, out, errOut)
	}
	in := `{"list":"Agent Sandbox","title":"A"}` + "\n" + `{"list":"L2","title":"B","priority":"5"}`
	if code, _, e := run(t, b, sandboxRW, in, "add", "-", "--confirm"); code != 0 || len(b.rems) != 7 {
		t.Fatalf("stdin add: %d %s", code, e)
	}
	if code, _, e := run(t, b, sandboxRW, "", "update", "R2", "--title", "Renamed", "--due", "+2d", "--priority", "low", "--notes", "n", "--confirm"); code != 0 || b.rems[1].Title != "Renamed" {
		t.Fatalf("update: %d %s", code, e)
	}
	if code, _, _ := run(t, b, sandboxRW, "", "update", "R2", "--clear-due", "--confirm"); code != 0 || !b.rems[1].Due.IsZero() {
		t.Fatal("clear-due")
	}
	if code, _, _ := run(t, b, sandboxRW, "", "complete", "R2", "--confirm"); code != 0 || !b.rems[1].Completed {
		t.Fatal("complete")
	}
	if code, _, _ := run(t, b, sandboxRW, "", "complete", "R2", "--undo", "--confirm"); code != 0 || b.rems[1].Completed {
		t.Fatal("undo")
	}
	if code, out, _ := run(t, b, sandboxRW, "", "delete", "R2", "--confirm", "--json"); code != 0 || !strings.Contains(out, `"deleted":true`) {
		t.Fatalf("delete: %s", out)
	}
	if code, _, _ := run(t, b, sandboxRW, "", "list-create", "Agent New", "--confirm"); code != 0 {
		t.Fatal("list-create")
	}
	if code, _, _ := run(t, b, sandboxRW, "", "list-create", "Agent New", "--confirm"); code != agentcli.ExitConflict {
		t.Fatal("duplicate list")
	}
	if code, _, _ := run(t, b, sandboxRW, "", "list-delete", "Agent New", "--confirm"); code != 0 {
		t.Fatal("list-delete")
	}
	if code, _, _ := run(t, b, sandboxRW, "", "list-delete", "Agent Sandbox", "--confirm"); code != agentcli.ExitConflict {
		t.Fatal("non-empty list must not be deleted")
	}
}

func TestWriteValidation(t *testing.T) {
	cases := map[string][]string{
		"no title":       {"add", "--list", "Agent Sandbox", "--dry-run"},
		"bad priority":   {"add", "--list", "Agent Sandbox", "--title", "x", "--priority", "urgent", "--dry-run"},
		"bad url":        {"add", "--list", "Agent Sandbox", "--title", "x", "--url", "javascript:alert(1)", "--dry-run"},
		"bad due":        {"add", "--list", "Agent Sandbox", "--title", "x", "--due", "someday", "--dry-run"},
		"bad arg":        {"add", "x", "--dry-run"},
		"nothing":        {"update", "R2", "--dry-run"},
		"empty title":    {"update", "R2", "--title", " ", "--dry-run"},
		"due and clear":  {"update", "R2", "--due", "today", "--clear-due", "--dry-run"},
		"empty list":     {"list-create", " ", "--dry-run"},
		"bad update pri": {"update", "R2", "--priority", "99", "--dry-run"},
		"bad update due": {"update", "R2", "--due", "x", "--dry-run"},
		"bad before":     {"list", "--due-before", "x"},
	}
	for name, args := range cases {
		if code, _, _ := run(t, newFake(), sandboxRW, "", args...); code != agentcli.ExitUsage {
			t.Errorf("%s: exit %d", name, code)
		}
	}
	if code, _, _ := run(t, newFake(), sandboxRW, "", "add", "--list", "Dup", "--title", "x", "--dry-run"); code != agentcli.ExitConflict {
		t.Error("ambiguous list name must conflict")
	}
	if code, _, _ := run(t, newFake(), sandboxRW, "", "add", "--list", "Nope", "--title", "x", "--dry-run"); code != agentcli.ExitNotFound {
		t.Error("unknown list")
	}
	if code, _, _ := run(t, newFake(), sandboxRW, "", "complete", "R2", "nope", "--dry-run"); code != agentcli.ExitNotFound {
		t.Error("unknown id in batch")
	}
}

func TestBridgeErrors(t *testing.T) {
	b := newFake()
	b.fail = agentcli.Permissionf("denied")
	for _, args := range [][]string{{"lists"}, {"list"}, {"search", "x"}} {
		if code, _, _ := run(t, b, sandboxRW, "", args...); code != agentcli.ExitPermission {
			t.Errorf("%v: %d", args, code)
		}
	}
	if code, out, _ := run(t, b, sandboxRW, "", "doctor"); code != agentcli.ExitPermission || !strings.Contains(out, "false") {
		t.Errorf("doctor: %d %s", code, out)
	}
	b2 := newFake()
	if code, _, _ := run(t, b2, sandboxRW, "", "get", "nope"); code != agentcli.ExitNotFound {
		t.Error("get missing")
	}
	b2.fail = agentcli.Unavailablef("x")
	b2.rems = append(b2.rems, Reminder{ID: "R9", List: "Agent Sandbox", ListID: "L2"})
	if code, _, e := run(t, b2, sandboxRW, "", "complete", "R2", "R9", "--confirm"); code != agentcli.ExitUnavailable || strings.Contains(e, "of 2") {
		t.Errorf("first failure is not partial: %d %s", code, e)
	}
}

func TestPartial(t *testing.T) {
	e := agentcli.AsError(partial(agentcli.Unavailablef("boom"), 2, 5))
	if e.Exit != agentcli.ExitUnavailable || !strings.HasPrefix(e.Message, "2 of 5") {
		t.Fatal(e)
	}
}

func TestPriorityMapping(t *testing.T) {
	for in, want := range map[int]string{0: "none", 1: "high", 4: "high", 5: "medium", 6: "low", 9: "low"} {
		if priorityName(in) != want {
			t.Errorf("%d", in)
		}
	}
}

func TestSchemaListsWriteOps(t *testing.T) {
	_, out, _ := run(t, newFake(), readOnly, "", "schema")
	var s struct {
		Commands []struct{ Name, Kind, Op string }
	}
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Fatal(err)
	}
	writes := 0
	for _, c := range s.Commands {
		if c.Kind == "write" {
			writes++
		}
	}
	if writes != 6 {
		t.Fatalf("expected 6 write commands, got %d", writes)
	}
}
