| Command | Kind | Description |
|---|---|---|
| `lists` | read | reminder lists (id, title, account) |
| `list [--list L]… [--status incomplete\|completed\|all] [--due-before D] [--due-after D]` | read | reminders, sorted by due date |
| `search <text> [--list L] [--status S]` | read | title/notes contain text (case-insensitive) |
| `get <id>` | read | one reminder with all fields |
| `doctor` | read | permission check |
| `add --list L --title T [--notes] [--due] [--priority] [--url]` / `add -` | write `add` | create reminders (stdin: JSON/NDJSON) |
| `update <id> [--title] [--notes] [--due\|--clear-due] [--priority]` | write `update` | change a reminder |
| `complete <id>… [--undo]` | write `complete` | mark completed / not completed |
| `delete <id>…` | write `delete` | delete reminders |
| `list-create <title>` / `list-delete <list>` | write | create a list / delete an **empty** list |
| `schema`, `policy`, `guard`, `version` | builtin | contract, effective policy, hook guard, version |

Dates accept `YYYY-MM-DD` (all-day), `YYYY-MM-DDTHH:MM`, RFC 3339, `today`, `tomorrow` and `+Nd`. Priority accepts `none|high|medium|low` or `0-9`.
