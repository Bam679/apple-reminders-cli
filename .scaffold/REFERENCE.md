## Fields

| Field | Description |
|---|---|
| `id` | EventKit `calendarItemIdentifier`. Stable on this Mac; pass it to `get`, `update`, `complete` and `delete`. |
| `title`, `notes`, `url` | Text fields. |
| `list`, `list_id` | Title and identifier of the reminder list. |
| `completed`, `completed_at` | Completion state and time. |
| `due`, `due_all_day` | `YYYY-MM-DD` for all-day reminders, RFC 3339 in local time otherwise. |
| `priority` | `none`, `high` (EventKit 1–4), `medium` (5) or `low` (6–9). |
| `created`, `modified` | RFC 3339 timestamps. |

The default text columns are `id due title list` for reminders and `id title account` for lists. Reminders are sorted by due date (undated ones last), then title.

## Scopes

Policy scopes match reminder list titles. A write needs write scope on the list of every reminder it touches.

## Behaviour notes

- `--list` takes a list title or identifier. If two lists have the same title, the CLI exits with code 8; use the identifier.
- `list` shows incomplete reminders by default; `--status completed` or `--status all` changes that. `--due-before` and `--due-after` leave out reminders without a due date.
- `list-delete` only deletes empty lists.
- If a batch fails part-way, the error message starts with `N of M writes succeeded`.
