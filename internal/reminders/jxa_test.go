package reminders

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Bam679/apple-reminders-cli/internal/osa"
)

func TestScriptIsStatic(t *testing.T) {
	if !strings.Contains(script, "const OPS") || strings.Contains(script, "%s") || strings.Contains(script, "%v") {
		t.Fatal("script must be a static OPS table")
	}
	for _, op := range []string{"lists", "reminders", "get", "add", "update", "set_completed", "delete", "list_create", "list_delete"} {
		if !strings.Contains(script, "  "+op+": function") {
			t.Errorf("script lacks op %s", op)
		}
	}
}

func TestJXAWireMapping(t *testing.T) {
	ms := int64(1767225600000) // 2026-01-01T00:00:00Z
	wire := map[string]any{"id": "X", "title": "T", "list": "L", "list_id": "LID", "completed": true,
		"completed_ms": ms, "priority": 5, "notes": "n", "url": "https://e.x", "created_ms": ms, "modified_ms": nil,
		"due_ms": ms, "due_all_day": true}
	var seen = map[string]json.RawMessage{}
	f := &osa.Fake{Handler: func(op string, args json.RawMessage) (any, error) {
		seen[op] = args
		switch op {
		case "lists", "list_create":
			if op == "lists" {
				return []List{{ID: "1", Title: "A"}}, nil
			}
			return List{ID: "2", Title: "B"}, nil
		case "reminders":
			return []any{wire}, nil
		case "delete", "list_delete":
			return map[string]string{"id": "X"}, nil
		}
		return wire, nil
	}}
	j := &JXA{Runner: f}
	ctx := context.Background()
	ls, err := j.Lists(ctx)
	if err != nil || len(ls) != 1 {
		t.Fatal(err)
	}
	rs, err := j.Reminders(ctx, Filter{ListIDs: []string{"a"}, Status: "all"})
	if err != nil || len(rs) != 1 || !rs[0].Due.Equal(time.UnixMilli(ms)) || !rs[0].Modified.IsZero() || !rs[0].DueAllDay {
		t.Fatalf("mapping: %+v %v", rs, err)
	}
	if _, err := j.Get(ctx, "X"); err != nil {
		t.Fatal(err)
	}
	due := time.UnixMilli(ms)
	if _, err := j.Add(ctx, NewReminder{ListID: "L", Title: "T", Due: due}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(seen["add"]), `"due_ms":1767225600000`) {
		t.Fatalf("add args: %s", seen["add"])
	}
	title := "new"
	if _, err := j.Update(ctx, "X", Patch{Title: &title, ClearDue: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(seen["update"]), `"due_ms":null`) || !strings.Contains(string(seen["update"]), `"clear_due":true`) {
		t.Fatalf("update args: %s", seen["update"])
	}
	if _, err := j.SetCompleted(ctx, "X", true); err != nil {
		t.Fatal(err)
	}
	if err := j.Delete(ctx, "X"); err != nil {
		t.Fatal(err)
	}
	if _, err := j.CreateList(ctx, "B"); err != nil {
		t.Fatal(err)
	}
	if err := j.DeleteList(ctx, "X", false); err != nil {
		t.Fatal(err)
	}
	if NewJXA().Runner == nil {
		t.Fatal("production runner")
	}
}

func TestJXAErrorPropagates(t *testing.T) {
	f := &osa.Fake{Handler: func(string, json.RawMessage) (any, error) { return nil, context.DeadlineExceeded }}
	j := &JXA{Runner: f}
	if _, err := j.Reminders(context.Background(), Filter{}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := j.Get(context.Background(), "x"); err == nil {
		t.Fatal("expected error")
	}
}
