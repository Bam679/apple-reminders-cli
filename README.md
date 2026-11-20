# apple-reminders

Read and manage Apple Reminders from the command line, built for AI coding agents.

[![ci](https://github.com/Bam679/apple-reminders-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/Bam679/apple-reminders-cli/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/Bam679/apple-reminders-cli)](https://github.com/Bam679/apple-reminders-cli/releases)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/Bam679/apple-reminders-cli/badge)](https://scorecard.dev/viewer/?uri=github.com/Bam679/apple-reminders-cli)
[![licence](https://img.shields.io/badge/licence-MIT-blue.svg)](LICENSE)

`apple-reminders` reads and writes reminders through EventKit, the macOS framework behind the Reminders app, and prints one line per reminder or JSON. Everything runs on your Mac, and the CLI itself makes no network connections. Writes stay off until a policy file allows them.

Measured on 20 open reminders (2026-11-20, v0.1.0, macOS 26.6.2): the default text output was 1,385 bytes and `--json` 6,510 bytes, and the command took 0.56 s. Bytes are not tokens, and your lists will differ; `apple-reminders list --limit 20 | wc -c` shows your own number.

## Why a CLI instead of an MCP server

An MCP server loads its tool definitions into the agent's context in every session, and each result travels back through the model before the agent can act on it. A CLI costs nothing until it is called. The agent learns it from a short skill, filters the output with `jq` or `grep`, and chains several steps in one shell script, for example turning this week's events into reminders in a single run.

`apple-reminders` is one of five CLIs with the same output format, exit codes and permission model: [apple-mail](https://github.com/Bam679/apple-mail-cli), [apple-notes](https://github.com/Bam679/apple-notes-cli), [apple-calendar](https://github.com/Bam679/apple-calendar-cli), [apple-reminders](https://github.com/Bam679/apple-reminders-cli) and [paperless](https://github.com/Bam679/paperless-cli). Together they let an agent manage everyday things on a Mac with one set of rules. If your agent cannot run shell commands, an MCP server is the better fit.

## Install

1. Install the CLI:

   ```sh
   brew install Bam679/tap/apple-reminders
   ```

   Signed release archives and `go install` are described in [How to install and verify a release](docs/how-to/install-and-verify.md).

2. Check that it can reach Reminders:

   ```sh
   apple-reminders doctor
   ```

   On the first run macOS asks whether your terminal app may access Reminders. See [Security and permissions](#security-and-permissions).

3. Connect Claude Code. The plugin adds a skill that teaches the agent the CLI, and a hook that stops the agent from editing the policy file:

   ```sh
   claude plugin marketplace add Bam679/apple-reminders-cli
   claude plugin install apple-reminders@apple-reminders-cli
   ```

   To get all five tools and the shared `agent-cli` skill from one place, use the [agent-cli-plugins](https://github.com/Bam679/agent-cli-plugins) marketplace instead. Install each plugin from one marketplace only.

To remove everything: `claude plugin uninstall apple-reminders`, `brew uninstall apple-reminders`, and delete `~/.config/agent-cli/apple-reminders.json` if you created one.

## Quick start

```sh
apple-reminders doctor
```

```text
reminders_access	true	8 lists visible
```

```sh
apple-reminders lists                                 # one line per list: id, title, account
apple-reminders list --due-before tomorrow            # open reminders due today or earlier
apple-reminders search dentist --json --fields id,title,due
```

Without a policy file every write is refused, a dry run included:

```sh
apple-reminders add --list "Agent Inbox" --title "Call Bob" --dry-run
```

```text
{"error":{"code":"policy_denied","message":"operation \"add\" is not allowed by policy","hint":"the policy file must allow this; agents must not edit it","exit":4}}
```

The [tutorial](docs/tutorial/getting-started.md) continues from here to a first write in a container made for the exercise.

## Use it with your agent

With the plugin installed, Claude Code picks up the skill on its own. For other agents (Codex, Cursor, Gemini CLI), add this to `AGENTS.md`:

```markdown
Use `apple-reminders` for the user's reminders and to-do lists. Run `apple-reminders schema` once to learn the commands.
Writes need the user's policy file: preview with `--dry-run`, then run with `--confirm`
only after the user agreed. On exit code 4, stop and tell the user.
```

What an agent can rely on: the default output has one item per line with tab-separated fields; `--json` prints `{"items":[…],"count":N,"next_cursor":"…"}` and `--ndjson` one object per line. `--fields`, `--limit` (default 50, at most 1000) and `--max-chars` keep output small, and `--cursor` continues a list. Errors are one JSON line on stderr, and exit codes are fixed:

| Exit | Meaning |
|---|---|
| 0 | success |
| 1 | unexpected error |
| 2 | invalid arguments or flags |
| 3 | item or container not found |
| 4 | denied by the policy; no flag overrides this |
| 5 | write without `--confirm`; the plan is printed, nothing changed |
| 6 | missing macOS permission or credentials |
| 7 | backend did not answer in time |
| 8 | ambiguous name, or the item already exists |

The [CLI reference](docs/reference/cli.md) has every command, flag, field and the JSON shapes.

## How it works

`apple-reminders` runs one fixed JXA script through `/usr/bin/osascript`. The script talks to EventKit and receives your input as a single JSON argument, so text from you or the agent never becomes code. Every write command first builds a plan. The binary checks the plan against the policy file, then prints it (`--dry-run`, or no `--confirm`) or applies it (`--confirm`). [Security model](docs/explanation/security-model.md) explains why the checks live in the binary.

## When not to use it

- You need tags, subtasks or location-based alerts. EventKit doesn't expose them, so the CLI can't either.
- Your agent cannot run shell commands. An MCP server fits better there.
- You want a person to approve every single change in a dialog. The CLI's `--confirm` is typed by the agent; use Claude Code's `ask` rules from `apple-reminders policy --claude-settings` for that.

## Security and permissions

`apple-reminders` only reads until you allow writes in `~/.config/agent-cli/apple-reminders.json`:

```json
   {
     "version": 1,
     "operations": {
       "allow": [
         "read",
         "add",
         "update",
         "complete"
       ],
       "deny": [
         "delete",
         "list-delete"
       ]
     },
     "scopes": {
       "read": {
         "allow": [
           "*"
         ],
         "deny": [
           "Health*"
         ]
       },
       "write": {
         "allow": [
           "Agent*",
           "Shopping"
         ],
         "deny": []
       }
     },
     "limits": {
       "max_write_batch": 10
     }
   }
```

Operations are `read`, `write` (all writes) or single operations: `add`, `update`, `complete`, `delete`, `list-create`, `list-delete`. Scopes are glob patterns over reminder list titles (e.g. `Shopping`, `Agent*`). Deny wins over allow, and an empty `scopes.read.allow` means everything may be read. `apple-reminders policy` shows what is allowed now.

The binary reads the policy from that fixed path only. It takes the home directory from the user database, not from `$HOME`, and no flag or environment variable points it elsewhere. A file that is invalid, writable by other users or owned by someone else makes every command fail with exit code 4.

`apple-reminders policy --claude-settings` prints Claude Code settings that allow read commands, ask before every write, deny access to the policy directory, and turn on the Bash sandbox with only `apple-reminders` excluded. In the sandbox the agent cannot reach Reminders through `osascript`, the network or the policy file directly, so `apple-reminders` and its policy are the only way to the data. [How to configure permissions](docs/how-to/configure-permissions.md) walks through it.

### macOS privacy permission

`apple-reminders` reaches Reminders through EventKit, which it runs via `/usr/bin/osascript`. macOS gives the permission to the app that runs the CLI: Terminal, iTerm, VS Code or Claude. On first use macOS asks for Reminders access. If you declined, open System Settings > Privacy & Security > Reminders and switch on your terminal app. `apple-reminders doctor` shows whether access works; without it, commands exit with code 6.

### What leaves your machine

`apple-reminders` makes no network connections. Reminders stay in EventKit on your Mac, and iCloud syncs them as usual. There is no telemetry and no update check. Whatever an agent reads through `apple-reminders` is sent to the model provider by the agent, like any other tool output.

### Threat model

| Threat | Mitigation |
|---|---|
| A prompt-injected agent tries destructive writes | Writes are off by default. The binary checks an operation and scope allowlist, needs `--confirm` and limits batch sizes. |
| The agent edits its own permissions | The policy path is fixed. The plugin hook and the recommended deny rules block edits, and the sandbox blocks writes outside the project. |
| Input turns into code | Static JXA script (`go:embed`). Input travels only as one JSON argv element to `/usr/bin/osascript` (absolute path), and fuzz tests check the script is never altered by input. An integration test round-trips hostile strings through real osascript. |
| Text in the data steers the agent | Output is data only and control characters are stripped. The skill tells agents to treat returned content as untrusted, and read scopes can hide containers. |
| Secrets or personal data leak | No credentials involved; macOS TCC controls access. Errors never contain credentials. |
| Supply chain | No third-party Go dependencies. Actions are pinned to commit SHAs. CI runs govulncheck, gosec, CodeQL, gitleaks, dependency review and OpenSSF Scorecard. Releases ship signed checksums, SBOMs and build provenance. |

An agent with unrestricted shell access can go around any CLI, for example by calling `osascript` or `curl` directly. Only the sandbox closes that gap.

### Protecting PII with AI agents

- Hide sensitive containers with `scopes.read.deny`. Hidden containers never appear in the output.
- Ask for less. `--fields`, `--limit` and `--max-chars` keep personal data out of the context, and the skill tells agents to use them.
- Mask personal data before it reaches the model. [PasqualePillitteri/claude-privacy-tool](https://github.com/PasqualePillitteri/claude-privacy-tool) adds an offline masking hook to Claude Code, and [gregmos/PII-Shield](https://github.com/gregmos/PII-Shield) anonymises PII through an MCP server, a skill or a CLI.
- Scan tool output with a `PostToolUse` hook: [gitleaks/gitleaks](https://github.com/gitleaks/gitleaks) and [trufflesecurity/trufflehog](https://github.com/trufflesecurity/trufflehog) find secrets, and [data-privacy-stack/presidio](https://github.com/data-privacy-stack/presidio) (formerly microsoft/presidio) finds names, addresses, IBANs and phone numbers.
- Keep real data out of bug reports. Redact output or use synthetic examples.

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

## Documentation

| Page | Use it when |
|---|---|
| [Getting started](docs/tutorial/getting-started.md) | you are new and want a first read and a first write |
| [How to configure permissions](docs/how-to/configure-permissions.md) | you want an agent to write, or to hide private data |
| [How to install and verify a release](docs/how-to/install-and-verify.md) | you install without Homebrew or want to check signatures |
| [CLI reference](docs/reference/cli.md) | you need a flag, a field, an exit code or the policy format |
| [Security model](docs/explanation/security-model.md) | you want to know why the checks work this way and where they end |
| [Architecture decisions](docs/adr) | you want the reasoning behind the design |

## Contributing

[CONTRIBUTING.md](CONTRIBUTING.md) has the rules for changes, and [SECURITY.md](SECURITY.md) explains how to report a vulnerability privately.

```sh
git clone https://github.com/Bam679/apple-reminders-cli && cd apple-reminders-cli
lefthook install   # gofmt, gitleaks and commit-message checks
make check         # everything CI runs
make integration   # tests against the real backend on this machine
```

## Licence

MIT. See [LICENSE](LICENSE).
