# apple-reminders CLI reference

`apple-reminders` version 0.x. `apple-reminders schema` prints the same information as JSON and is always current for the installed version.

## Synopsis

```text
apple-reminders <command> [arguments] [flags]
```

Flags can appear before or after arguments. Every flag has two dashes. `--` ends flag parsing.

## Commands

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
| `list-create <title>` / `list-delete <list>` | write | create a list / delete an empty list |
| `schema`, `policy`, `guard`, `version` | builtin | contract, effective policy, hook guard, version |

Dates accept `YYYY-MM-DD` (all-day), `YYYY-MM-DDTHH:MM`, RFC 3339, `today`, `tomorrow` and `+Nd`. Priority accepts `none|high|medium|low` or `0-9`.

## Global flags

| Flag | Type | Description |
|---|---|---|
| `--json` | bool | Print one JSON document. |
| `--ndjson` | bool | Print one JSON object per line. |
| `--fields a,b` | string | Select and order fields. Unknown names are a usage error. |
| `--limit N` | int | Maximum number of items. Default 50, at most 1000. |
| `--cursor C` | string | Continue a list from a `next_cursor`. A cursor only works with the same command and flags. |
| `--max-chars N` | int | Cap the size of stdout. Lists drop whole items, single items shorten their longest text field. |
| `--dry-run` | bool | Writes only. Print the plan, change nothing. |
| `--confirm` | bool | Writes only. Apply the change. |
| `--timeout D` | duration | Abort after this time (default `60s`). |
| `--help` | bool | Show help. |

## Output formats

| Format | Lists | Single items |
|---|---|---|
| default | One line per item, tab-separated default fields, `-` for empty values. Trailer lines start with `#`: `# next_cursor=…`, `# truncated: …` | `field: value` lines |
| `--json` | `{"items":[…],"count":N,"next_cursor":"…","truncated":true}` | One object; `"truncated":true` when shortened |
| `--ndjson` | One object per line, then `{"_meta":{"next_cursor":…,"truncated":…}}` when relevant | One object |

Write commands print the changed items with an `op` field. With `--dry-run`, or without `--confirm`, they print the planned items with `"dry_run":true`.

Text output never contains control characters, tabs or newlines inside a value, or bidirectional override characters.

Errors are one JSON line on stderr:

```json
{"error":{"code":"policy_denied","message":"writing to \"Private\" is not allowed by policy","hint":"…","exit":4}}
```

## Exit codes

| Code | Name | Meaning |
|---|---|---|
| 0 | ok | Success. |
| 1 | internal | Unexpected error. |
| 2 | usage | Invalid arguments or flags. |
| 3 | not_found | The item or container does not exist. |
| 4 | policy_denied | The policy does not allow the operation or scope, or the policy file is invalid. No flag overrides this. |
| 5 | confirm_required | A write without `--confirm`. The plan is printed on stdout. |
| 6 | permission_denied | Missing macOS privacy permission, or invalid credentials. |
| 7 | unavailable | The backend did not answer, or the timeout expired. |
| 8 | conflict | An ambiguous name, or the item already exists. |

## Stdin

Write commands that take `-` as an argument read JSON objects from stdin: one object, an array of objects, or one object per line. Input is limited to 4 MiB, and unknown fields are rejected.

## Dates

Date flags accept `YYYY-MM-DD` (a whole day), `YYYY-MM-DDTHH:MM` and `YYYY-MM-DD HH:MM` in local time, RFC 3339, `today`, `tomorrow`, `yesterday`, `+Nd` and `-Nd`.

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

## Policy file

Path: `~/.config/agent-cli/apple-reminders.json`. The home directory comes from the user database, not from `$HOME`. No flag or environment variable selects a different file.

| Key | Type | Meaning |
|---|---|---|
| `version` | int | Must be `1`. |
| `operations.allow` | string list | `read`, `write` (every write operation), or single operations: `add`, `update`, `complete`, `delete`, `list-create`, `list-delete`. |
| `operations.deny` | string list | Denied operations. Deny wins over allow. |
| `scopes.read.allow` | glob list | Containers that may be read. Empty means all. |
| `scopes.read.deny` | glob list | Containers hidden from every read. |
| `scopes.write.allow` | glob list | Containers that may be changed. Empty means none. |
| `scopes.write.deny` | glob list | Containers that may never be changed. |
| `limits.max_write_batch` | int | Maximum number of items one write command may change (default 100). |

Scope strings are reminder list titles (e.g. `Shopping`, `Agent*`). In patterns `*` matches any text, including `/`; matching is case-sensitive.

Without a file, the default policy allows `read` on everything and no writes. The CLI rejects files that are not regular files, that are writable by group or others, that another user owns, or that contain unknown keys or invalid JSON. A rejected file denies every command except `schema`, `policy`, `version`, `help` and `guard`.

## Built-in commands

| Command | Description |
|---|---|
| `schema` | The machine-readable contract as JSON. |
| `policy` | The effective policy and its path. `--example` prints a template, and `--claude-settings` prints recommended Claude Code settings. |
| `guard` | Claude Code `PreToolUse` hook. It reads the hook JSON on stdin and denies tool calls that would modify agent-cli policy files. |
| `version` | Name and version. |
| `help` | Usage. |
