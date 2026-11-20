# apple-reminders reference

## Fields
`id, title, list, list_id, completed, completed_at, due, due_all_day, priority, notes, url, created, modified`

- `due` is `YYYY-MM-DD` for all-day reminders and RFC 3339 (local offset) otherwise.
- `priority` is one of `none|high|medium|low` (EventKit 0 / 1–4 / 5 / 6–9).
- `id` is the EventKit `calendarItemIdentifier`. It is stable on this Mac and can be passed back to `get`, `update`, `complete` and `delete`.

## Commands
| command | policy op | notes |
|---|---|---|
| `lists` | read | lists hidden by `scopes.read.deny` are omitted |
| `list` | read | `--list` (repeatable, name or id), `--status incomplete\|completed\|all`, `--due-before`, `--due-after` (reminders without a due date are excluded when a due filter is set) |
| `search <text>` | read | default status `all` |
| `get <id>` | read | exit 4 if the reminder's list is read-denied |
| `add` | add | `--list` and `--title` required; `-` reads JSON objects `{list,title,notes,due,priority,url}` |
| `update <id>` | update | `--title --notes --due --clear-due --priority` |
| `complete <id>…` | complete | `--undo` reopens |
| `delete <id>…` | delete | |
| `list-create <title>` | list-create | conflict if a list with that title exists |
| `list-delete <list>` | list-delete | only empty lists (exit 8 otherwise) |

## Policy (written by the user, read-only for agents)
`~/.config/agent-cli/apple-reminders.json`. Scopes match **list titles**. Example: allow adding to `Agent*` lists only:
```json
{"version":1,"operations":{"allow":["read","add","complete"]},"scopes":{"read":{"allow":["*"]},"write":{"allow":["Agent*"]}}}
```
`apple-reminders policy` shows the effective policy and `apple-reminders policy --example` prints a template. Agents must not create or edit this file; ask the user instead.

## Batches
`complete`, `delete` and `add -` act on several items. `limits.max_write_batch` caps the batch size. If a batch fails part-way, the error message starts with `N of M writes succeeded`.
