package reminders

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Bam679/apple-reminders-cli/internal/agentcli"
)

var (
	listFields     = []string{"id", "title", "account", "writable"}
	reminderFields = []string{"id", "title", "list", "list_id", "completed", "completed_at", "due", "due_all_day",
		"priority", "notes", "url", "created", "modified"}
	reminderCols = []string{"id", "due", "title", "list"}
)

// NewApp wires the CLI commands to bridge b.
func NewApp(b Bridge, version string) *agentcli.App {
	h := &handlers{b: b}
	listFlag := agentcli.Flag{Name: "list", Type: agentcli.Strings, Usage: "list name or id (repeatable)"}
	statusFlag := agentcli.Flag{Name: "status", Type: agentcli.String, Usage: "which reminders", Enum: []string{"incomplete", "completed", "all"}}
	return &agentcli.App{
		Name:         "apple-reminders",
		Version:      version,
		Summary:      "Apple Reminders for AI agents (read-only unless the policy allows writes)",
		ScopeHelp:    "reminder list title, e.g. \"Shopping\"",
		ScopeExample: "Agent*",
		Commands: []*agentcli.Command{
			{Name: "lists", Summary: "list reminder lists", Kind: agentcli.Read,
				Fields: []string{"id", "title", "account"}, AllFields: listFields, Run: h.lists,
				Example: "apple-reminders lists --json"},
			{Name: "list", Summary: "list reminders (default: incomplete, all lists)", Kind: agentcli.Read,
				Flags: []agentcli.Flag{listFlag, statusFlag,
					{Name: "due-before", Type: agentcli.String, Usage: "only reminders due before this date"},
					{Name: "due-after", Type: agentcli.String, Usage: "only reminders due on/after this date"}},
				Fields: reminderCols, AllFields: reminderFields, Run: h.list,
				Example: "apple-reminders list --list Shopping --due-before tomorrow"},
			{Name: "search", Summary: "find reminders whose title or notes contain text (default: all statuses)", Kind: agentcli.Read,
				Args:  []agentcli.Arg{{Name: "query", Usage: "case-insensitive text", Required: true}},
				Flags: []agentcli.Flag{listFlag, statusFlag}, Fields: reminderCols, AllFields: reminderFields, Run: h.search,
				Example: "apple-reminders search dentist --json"},
			{Name: "get", Summary: "show one reminder", Kind: agentcli.Read,
				Args:   []agentcli.Arg{{Name: "id", Usage: "reminder id", Required: true}},
				Fields: reminderFields, AllFields: reminderFields, Run: h.get},
			{Name: "doctor", Summary: "check permissions and connectivity", Kind: agentcli.Read,
				Fields: []string{"check", "ok", "detail"}, Run: h.doctor},
			{Name: "add", Summary: "create reminders", Kind: agentcli.Write, Op: "add",
				Args: []agentcli.Arg{{Name: "input", Usage: "'-' to read JSON/NDJSON objects from stdin"}},
				Flags: []agentcli.Flag{
					{Name: "list", Type: agentcli.String, Usage: "target list name or id"},
					{Name: "title", Type: agentcli.String, Usage: "title"},
					{Name: "notes", Type: agentcli.String, Usage: "notes"},
					{Name: "due", Type: agentcli.String, Usage: "due date (YYYY-MM-DD = all day) or date time"},
					{Name: "priority", Type: agentcli.String, Usage: "none|high|medium|low or 0-9"},
					{Name: "url", Type: agentcli.String, Usage: "http(s) URL"}},
				Stdin:  `{"list":"Agent Inbox","title":"…","notes":"…","due":"2026-12-01","priority":"high","url":"https://…"} (one per line)`,
				Fields: reminderCols, AllFields: reminderFields, Plan: h.add,
				Example: "apple-reminders add --list 'Agent Inbox' --title 'Call Bob' --due tomorrow --dry-run"},
			{Name: "update", Summary: "change a reminder", Kind: agentcli.Write, Op: "update",
				Args: []agentcli.Arg{{Name: "id", Usage: "reminder id", Required: true}},
				Flags: []agentcli.Flag{
					{Name: "title", Type: agentcli.String, Usage: "new title"},
					{Name: "notes", Type: agentcli.String, Usage: "new notes"},
					{Name: "due", Type: agentcli.String, Usage: "new due date"},
					{Name: "clear-due", Type: agentcli.Bool, Usage: "remove the due date"},
					{Name: "priority", Type: agentcli.String, Usage: "none|high|medium|low or 0-9"}},
				Fields: reminderCols, AllFields: reminderFields, Plan: h.update},
			{Name: "complete", Summary: "mark reminders completed (or --undo)", Kind: agentcli.Write, Op: "complete",
				Args:   []agentcli.Arg{{Name: "id", Usage: "reminder id(s)", Required: true, Variadic: true}},
				Flags:  []agentcli.Flag{{Name: "undo", Type: agentcli.Bool, Usage: "mark as not completed"}},
				Fields: reminderCols, AllFields: reminderFields, Plan: h.complete},
			{Name: "delete", Summary: "delete reminders", Kind: agentcli.Write, Op: "delete",
				Args:   []agentcli.Arg{{Name: "id", Usage: "reminder id(s)", Required: true, Variadic: true}},
				Fields: reminderCols, AllFields: reminderFields, Plan: h.delete},
			{Name: "list-create", Summary: "create a reminder list", Kind: agentcli.Write, Op: "list-create",
				Args:   []agentcli.Arg{{Name: "title", Usage: "new list title", Required: true}},
				Fields: []string{"id", "title", "account"}, AllFields: listFields, Plan: h.listCreate},
			{Name: "list-delete", Summary: "delete an empty reminder list", Kind: agentcli.Write, Op: "list-delete",
				Args:   []agentcli.Arg{{Name: "list", Usage: "list name or id", Required: true}},
				Fields: []string{"id", "title"}, AllFields: listFields, Plan: h.listDelete},
		},
	}
}

type handlers struct{ b Bridge }

func listItem(l List) agentcli.Item {
	return agentcli.Item{"id": l.ID, "title": l.Title, "account": l.Account, "writable": l.Writable}
}

func reminderItem(r Reminder) agentcli.Item {
	return agentcli.Item{
		"id": r.ID, "title": r.Title, "list": r.List, "list_id": r.ListID, "completed": r.Completed,
		"completed_at": agentcli.FormatTime(r.CompletedAt, false), "due": agentcli.FormatTime(r.Due, r.DueAllDay),
		"due_all_day": r.DueAllDay, "priority": priorityName(r.Priority), "notes": r.Notes, "url": r.URL,
		"created": agentcli.FormatTime(r.Created, false), "modified": agentcli.FormatTime(r.Modified, false),
	}
}

func priorityName(p int) string {
	switch {
	case p == 0:
		return "none"
	case p <= 4:
		return "high"
	case p == 5:
		return "medium"
	default:
		return "low"
	}
}

func parsePriority(s string) (int, error) {
	switch strings.ToLower(s) {
	case "", "none":
		return 0, nil
	case "high":
		return 1, nil
	case "medium":
		return 5, nil
	case "low":
		return 9, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 9 {
		return 0, agentcli.Usagef("priority must be none|high|medium|low or 0-9")
	}
	return n, nil
}

func (h *handlers) resolveList(ctx context.Context, nameOrID string) (List, error) {
	ls, err := h.b.Lists(ctx)
	if err != nil {
		return List{}, err
	}
	var matches []List
	for _, l := range ls {
		if l.ID == nameOrID {
			return l, nil
		}
		if l.Title == nameOrID {
			matches = append(matches, l)
		}
	}
	switch len(matches) {
	case 0:
		return List{}, agentcli.NotFoundf("no reminder list %q", nameOrID)
	case 1:
		return matches[0], nil
	}
	return List{}, agentcli.Conflictf("%d lists are named %q; pass the list id instead", len(matches), nameOrID)
}

func (h *handlers) lists(c *agentcli.Ctx) error {
	ls, err := h.b.Lists(c)
	if err != nil {
		return err
	}
	sort.Slice(ls, func(i, j int) bool {
		return ls[i].Title < ls[j].Title || (ls[i].Title == ls[j].Title && ls[i].ID < ls[j].ID)
	})
	var items []agentcli.Item
	for _, l := range ls {
		if c.CanRead(l.Title) {
			items = append(items, listItem(l))
		}
	}
	return c.List(items)
}

// fetch loads reminders for the --list/--status flags, filtered by read scope.
func (h *handlers) fetch(c *agentcli.Ctx, defStatus string) ([]Reminder, error) {
	f := Filter{Status: c.String("status")}
	if f.Status == "" {
		f.Status = defStatus
	}
	for _, name := range c.Strings("list") {
		l, err := h.resolveList(c, name)
		if err != nil {
			return nil, err
		}
		if err := c.CheckRead(l.Title); err != nil {
			return nil, err
		}
		f.ListIDs = append(f.ListIDs, l.ID)
	}
	rs, err := h.b.Reminders(c, f)
	if err != nil {
		return nil, err
	}
	out := rs[:0]
	for _, r := range rs {
		if c.CanRead(r.List) {
			out = append(out, r)
		}
	}
	sortReminders(out)
	return out, nil
}

func sortReminders(rs []Reminder) {
	sort.SliceStable(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		if a.Due.IsZero() != b.Due.IsZero() {
			return !a.Due.IsZero()
		}
		if !a.Due.Equal(b.Due) {
			return a.Due.Before(b.Due)
		}
		if a.Title != b.Title {
			return a.Title < b.Title
		}
		return a.ID < b.ID
	})
}

func items(rs []Reminder) []agentcli.Item {
	out := make([]agentcli.Item, 0, len(rs))
	for _, r := range rs {
		out = append(out, reminderItem(r))
	}
	return out
}

func (h *handlers) list(c *agentcli.Ctx) error {
	var before, after time.Time
	for flag, dst := range map[string]*time.Time{"due-before": &before, "due-after": &after} {
		if s := c.String(flag); s != "" {
			t, _, err := agentcli.ParseTime(s, c.Now(), time.Local)
			if err != nil {
				return err
			}
			*dst = t
		}
	}
	rs, err := h.fetch(c, "incomplete")
	if err != nil {
		return err
	}
	var out []Reminder
	for _, r := range rs {
		if (!before.IsZero() || !after.IsZero()) && r.Due.IsZero() {
			continue
		}
		if (!before.IsZero() && !r.Due.Before(before)) || (!after.IsZero() && r.Due.Before(after)) {
			continue
		}
		out = append(out, r)
	}
	return c.List(items(out), strings.Join(c.Strings("list"), "\x00"), c.String("status"), c.String("due-before"), c.String("due-after"))
}

func (h *handlers) search(c *agentcli.Ctx) error {
	q := strings.ToLower(c.Args[0])
	rs, err := h.fetch(c, "all")
	if err != nil {
		return err
	}
	var out []Reminder
	for _, r := range rs {
		if strings.Contains(strings.ToLower(r.Title), q) || strings.Contains(strings.ToLower(r.Notes), q) {
			out = append(out, r)
		}
	}
	return c.List(items(out), q, strings.Join(c.Strings("list"), "\x00"), c.String("status"))
}

func (h *handlers) get(c *agentcli.Ctx) error {
	r, err := h.b.Get(c, c.Args[0])
	if err != nil {
		return err
	}
	if err := c.CheckRead(r.List); err != nil {
		return err
	}
	return c.Show(reminderItem(r))
}

func (h *handlers) doctor(c *agentcli.Ctx) error {
	ls, err := h.b.Lists(c)
	row := agentcli.Item{"check": "reminders_access", "ok": err == nil, "detail": "full access"}
	if err != nil {
		row["detail"] = agentcli.AsError(err).Message
	} else {
		row["detail"] = strconv.Itoa(len(ls)) + " lists visible"
	}
	if err := c.List([]agentcli.Item{row}); err != nil {
		return err
	}
	return err
}

type addInput struct {
	List     string `json:"list"`
	Title    string `json:"title"`
	Notes    string `json:"notes"`
	Due      string `json:"due"`
	Priority string `json:"priority"`
	URL      string `json:"url"`
}

func validURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func (h *handlers) add(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	var ins []addInput
	if len(c.Args) > 0 {
		if c.Args[0] != "-" {
			return nil, agentcli.Usagef("the only positional argument is '-' (stdin)")
		}
		var err error
		if ins, err = agentcli.DecodeInputs[addInput](c.Stdin); err != nil {
			return nil, err
		}
	} else {
		ins = []addInput{{List: c.String("list"), Title: c.String("title"), Notes: c.String("notes"),
			Due: c.String("due"), Priority: c.String("priority"), URL: c.String("url")}}
	}
	var news []NewReminder
	var scopes []string
	var targets []agentcli.Item
	for i, in := range ins {
		if strings.TrimSpace(in.Title) == "" || in.List == "" {
			return nil, agentcli.Usagef("item %d: list and title are required", i+1)
		}
		l, err := h.resolveList(c, in.List)
		if err != nil {
			return nil, err
		}
		n := NewReminder{ListID: l.ID, Title: in.Title, Notes: in.Notes, URL: in.URL}
		if n.Priority, err = parsePriority(in.Priority); err != nil {
			return nil, err
		}
		if in.URL != "" && !validURL(in.URL) {
			return nil, agentcli.Usagef("item %d: url must be http(s)", i+1)
		}
		if in.Due != "" {
			if n.Due, n.DueAllDay, err = agentcli.ParseTime(in.Due, c.Now(), time.Local); err != nil {
				return nil, err
			}
		}
		news = append(news, n)
		scopes = append(scopes, l.Title)
		targets = append(targets, agentcli.Item{"title": n.Title, "list": l.Title, "due": agentcli.FormatTime(n.Due, n.DueAllDay)})
	}
	return &agentcli.WritePlan{Scopes: scopes, Targets: targets, Apply: func(ctx context.Context) ([]agentcli.Item, error) {
		var out []agentcli.Item
		for _, n := range news {
			r, err := h.b.Add(ctx, n)
			if err != nil {
				return nil, partial(err, len(out), len(news))
			}
			out = append(out, reminderItem(r))
		}
		return out, nil
	}}, nil
}

// partial annotates an error that happened after some writes succeeded.
func partial(err error, done, total int) error {
	if done == 0 {
		return err
	}
	e := *agentcli.AsError(err)
	e.Message = strconv.Itoa(done) + " of " + strconv.Itoa(total) + " writes succeeded before: " + e.Message
	return &e
}

func (h *handlers) update(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	r, err := h.b.Get(c, c.Args[0])
	if err != nil {
		return nil, err
	}
	var p Patch
	target := reminderItem(r)
	changed := false
	if c.IsSet("title") {
		v := c.String("title")
		if strings.TrimSpace(v) == "" {
			return nil, agentcli.Usagef("title must not be empty")
		}
		p.Title, target["title"], changed = &v, v, true
	}
	if c.IsSet("notes") {
		v := c.String("notes")
		p.Notes, target["notes"], changed = &v, v, true
	}
	if c.IsSet("priority") {
		n, err := parsePriority(c.String("priority"))
		if err != nil {
			return nil, err
		}
		p.Priority, target["priority"], changed = &n, priorityName(n), true
	}
	if c.Bool("clear-due") && c.IsSet("due") {
		return nil, agentcli.Usagef("--due and --clear-due are mutually exclusive")
	}
	if c.Bool("clear-due") {
		p.ClearDue, target["due"], changed = true, "", true
	} else if c.IsSet("due") {
		if p.Due, p.DueAllDay, err = agentcli.ParseTime(c.String("due"), c.Now(), time.Local); err != nil {
			return nil, err
		}
		target["due"], changed = agentcli.FormatTime(p.Due, p.DueAllDay), true
	}
	if !changed {
		return nil, agentcli.Usagef("nothing to change; pass --title, --notes, --due, --clear-due or --priority")
	}
	return &agentcli.WritePlan{Scopes: []string{r.List}, Targets: []agentcli.Item{target},
		Apply: func(ctx context.Context) ([]agentcli.Item, error) {
			u, err := h.b.Update(ctx, r.ID, p)
			if err != nil {
				return nil, err
			}
			return []agentcli.Item{reminderItem(u)}, nil
		}}, nil
}

// each loads every id and builds a plan that applies fn to each reminder.
func (h *handlers) each(c *agentcli.Ctx, fn func(context.Context, Reminder) (agentcli.Item, error)) (*agentcli.WritePlan, error) {
	var rs []Reminder
	var scopes []string
	var targets []agentcli.Item
	for _, id := range c.Args {
		r, err := h.b.Get(c, id)
		if err != nil {
			return nil, err
		}
		rs = append(rs, r)
		scopes = append(scopes, r.List)
		targets = append(targets, reminderItem(r))
	}
	return &agentcli.WritePlan{Scopes: scopes, Targets: targets, Apply: func(ctx context.Context) ([]agentcli.Item, error) {
		var out []agentcli.Item
		for _, r := range rs {
			it, err := fn(ctx, r)
			if err != nil {
				return nil, partial(err, len(out), len(rs))
			}
			out = append(out, it)
		}
		return out, nil
	}}, nil
}

func (h *handlers) complete(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	done := !c.Bool("undo")
	return h.each(c, func(ctx context.Context, r Reminder) (agentcli.Item, error) {
		u, err := h.b.SetCompleted(ctx, r.ID, done)
		if err != nil {
			return nil, err
		}
		return reminderItem(u), nil
	})
}

func (h *handlers) delete(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	return h.each(c, func(ctx context.Context, r Reminder) (agentcli.Item, error) {
		if err := h.b.Delete(ctx, r.ID); err != nil {
			return nil, err
		}
		it := reminderItem(r)
		it["deleted"] = true
		return it, nil
	})
}

func (h *handlers) listCreate(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	title := strings.TrimSpace(c.Args[0])
	if title == "" {
		return nil, agentcli.Usagef("title must not be empty")
	}
	if _, err := h.resolveList(c, title); err == nil {
		return nil, agentcli.Conflictf("a list named %q already exists", title)
	} else if agentcli.AsError(err).Exit != agentcli.ExitNotFound {
		return nil, err
	}
	return &agentcli.WritePlan{Scopes: []string{title}, Targets: []agentcli.Item{{"title": title}},
		Apply: func(ctx context.Context) ([]agentcli.Item, error) {
			l, err := h.b.CreateList(ctx, title)
			if err != nil {
				return nil, err
			}
			return []agentcli.Item{listItem(l)}, nil
		}}, nil
}

func (h *handlers) listDelete(c *agentcli.Ctx) (*agentcli.WritePlan, error) {
	l, err := h.resolveList(c, c.Args[0])
	if err != nil {
		return nil, err
	}
	return &agentcli.WritePlan{Scopes: []string{l.Title}, Targets: []agentcli.Item{listItem(l)},
		Apply: func(ctx context.Context) ([]agentcli.Item, error) {
			if err := h.b.DeleteList(ctx, l.ID, false); err != nil {
				return nil, err
			}
			it := listItem(l)
			it["deleted"] = true
			return []agentcli.Item{it}, nil
		}}, nil
}
