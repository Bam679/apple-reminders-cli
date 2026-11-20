// Package reminders implements the apple-reminders CLI on top of a Bridge to
// Apple Reminders (EventKit via JXA in production, a fake in tests).
package reminders

import (
	"context"
	_ "embed"
	"time"

	"github.com/Bam679/apple-reminders-cli/internal/osa"
)

// List is a reminder list (an EventKit calendar).
type List struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Account  string `json:"account"`
	Writable bool   `json:"writable"`
}

// Reminder is one reminder.
type Reminder struct {
	ID          string
	Title       string
	List        string
	ListID      string
	Completed   bool
	CompletedAt time.Time
	Due         time.Time
	DueAllDay   bool
	Priority    int
	Notes       string
	URL         string
	Created     time.Time
	Modified    time.Time
}

// Filter selects reminders.
type Filter struct {
	ListIDs []string
	Status  string // incomplete (default), completed, all
}

// NewReminder is the input for Add.
type NewReminder struct {
	ListID    string
	Title     string
	Notes     string
	URL       string
	Priority  int
	Due       time.Time // zero = no due date
	DueAllDay bool
}

// Patch changes selected fields; nil means unchanged.
type Patch struct {
	Title     *string
	Notes     *string
	Priority  *int
	Due       time.Time
	DueAllDay bool
	ClearDue  bool
}

// Bridge is the OS boundary. Implementations must not interpret input as code.
type Bridge interface {
	Lists(ctx context.Context) ([]List, error)
	Reminders(ctx context.Context, f Filter) ([]Reminder, error)
	Get(ctx context.Context, id string) (Reminder, error)
	Add(ctx context.Context, r NewReminder) (Reminder, error)
	Update(ctx context.Context, id string, p Patch) (Reminder, error)
	SetCompleted(ctx context.Context, id string, done bool) (Reminder, error)
	Delete(ctx context.Context, id string) error
	CreateList(ctx context.Context, title string) (List, error)
	// DeleteList refuses non-empty lists unless force (force is only used by
	// integration-test cleanup; the CLI never sets it).
	DeleteList(ctx context.Context, id string, force bool) error
}

//go:embed reminders.js
var script string

// JXA is the production Bridge backed by EventKit through osascript.
type JXA struct {
	Runner osa.Runner
}

// NewJXA returns the production bridge.
func NewJXA() *JXA { return &JXA{Runner: osa.Exec{}} }

type wireReminder struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	List        string `json:"list"`
	ListID      string `json:"list_id"`
	Completed   bool   `json:"completed"`
	CompletedMS *int64 `json:"completed_ms"`
	Priority    int    `json:"priority"`
	Notes       string `json:"notes"`
	URL         string `json:"url"`
	CreatedMS   *int64 `json:"created_ms"`
	ModifiedMS  *int64 `json:"modified_ms"`
	DueMS       *int64 `json:"due_ms"`
	DueAllDay   bool   `json:"due_all_day"`
}

func fromMS(v *int64) time.Time {
	if v == nil {
		return time.Time{}
	}
	return time.UnixMilli(*v).Local()
}

func toMS(t time.Time) *int64 {
	if t.IsZero() {
		return nil
	}
	v := t.UnixMilli()
	return &v
}

func (w wireReminder) reminder() Reminder {
	return Reminder{
		ID: w.ID, Title: w.Title, List: w.List, ListID: w.ListID, Completed: w.Completed,
		CompletedAt: fromMS(w.CompletedMS), Priority: w.Priority, Notes: w.Notes, URL: w.URL,
		Created: fromMS(w.CreatedMS), Modified: fromMS(w.ModifiedMS), Due: fromMS(w.DueMS), DueAllDay: w.DueAllDay,
	}
}

func (j *JXA) one(ctx context.Context, op string, args any) (Reminder, error) {
	var w wireReminder
	if err := j.Runner.Run(ctx, script, op, args, &w); err != nil {
		return Reminder{}, err
	}
	return w.reminder(), nil
}

// Lists implements Bridge.
func (j *JXA) Lists(ctx context.Context) ([]List, error) {
	var out []List
	err := j.Runner.Run(ctx, script, "lists", nil, &out)
	return out, err
}

// Reminders implements Bridge.
func (j *JXA) Reminders(ctx context.Context, f Filter) ([]Reminder, error) {
	var ws []wireReminder
	args := map[string]any{"list_ids": f.ListIDs, "status": f.Status}
	if err := j.Runner.Run(ctx, script, "reminders", args, &ws); err != nil {
		return nil, err
	}
	out := make([]Reminder, len(ws))
	for i, w := range ws {
		out[i] = w.reminder()
	}
	return out, nil
}

// Get implements Bridge.
func (j *JXA) Get(ctx context.Context, id string) (Reminder, error) {
	return j.one(ctx, "get", map[string]string{"id": id})
}

// Add implements Bridge.
func (j *JXA) Add(ctx context.Context, r NewReminder) (Reminder, error) {
	return j.one(ctx, "add", map[string]any{
		"list_id": r.ListID, "title": r.Title, "notes": r.Notes, "url": r.URL,
		"priority": r.Priority, "due_ms": toMS(r.Due), "due_all_day": r.DueAllDay,
	})
}

// Update implements Bridge.
func (j *JXA) Update(ctx context.Context, id string, p Patch) (Reminder, error) {
	return j.one(ctx, "update", map[string]any{
		"id": id, "title": p.Title, "notes": p.Notes, "priority": p.Priority,
		"due_ms": toMS(p.Due), "due_all_day": p.DueAllDay, "clear_due": p.ClearDue,
	})
}

// SetCompleted implements Bridge.
func (j *JXA) SetCompleted(ctx context.Context, id string, done bool) (Reminder, error) {
	return j.one(ctx, "set_completed", map[string]any{"id": id, "completed": done})
}

// Delete implements Bridge.
func (j *JXA) Delete(ctx context.Context, id string) error {
	return j.Runner.Run(ctx, script, "delete", map[string]string{"id": id}, nil)
}

// CreateList implements Bridge.
func (j *JXA) CreateList(ctx context.Context, title string) (List, error) {
	var l List
	err := j.Runner.Run(ctx, script, "list_create", map[string]string{"title": title}, &l)
	return l, err
}

// DeleteList implements Bridge.
func (j *JXA) DeleteList(ctx context.Context, id string, force bool) error {
	return j.Runner.Run(ctx, script, "list_delete", map[string]any{"id": id, "force": force}, nil)
}
