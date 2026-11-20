# Getting started with apple-reminders

In this tutorial you install `apple-reminders`, read some of your Apple Reminders data with it, and then let it make one change in a container that exists only for this exercise. It takes about ten minutes. You need a terminal and macOS 14+, Apple Silicon and Intel.

## Install the CLI

```sh
brew install Bam679/tap/apple-reminders
apple-reminders version
```

You see `apple-reminders` followed by a version number.

## Give it access to Reminders

```sh
apple-reminders doctor
```

macOS asks whether your terminal may access Reminders. Choose Allow. The command prints a line like `reminders_access  true  8 lists visible`.

## Read your reminders

```sh
apple-reminders lists
apple-reminders list --limit 5
```

The first command prints one line per list, the second your next five open reminders with their due dates. Notice that each line starts with an identifier: that is how an agent refers to a reminder later.

## Try to change something

```sh
apple-reminders list-create "Agent Tutorial" --dry-run
```

The command exits with code 4 and prints `operation "list-create" is not allowed by policy`. Without a policy file the CLI only reads.

## Allow writes in one list

Create a policy that allows writes in a list called "Agent Tutorial" and nowhere else:

```sh
mkdir -p ~/.config/agent-cli
cat > ~/.config/agent-cli/apple-reminders.json <<'JSON'
{
  "version": 1,
  "operations": { "allow": ["read", "write"] },
  "scopes": { "write": { "allow": ["Agent Tutorial"] } }
}
JSON
chmod 600 ~/.config/agent-cli/apple-reminders.json
apple-reminders policy
```

The last command shows the policy you just wrote.

## Make your first change

```sh
apple-reminders list-create "Agent Tutorial" --confirm
apple-reminders add --list "Agent Tutorial" --title "Water the plants" --due tomorrow
```

The second command exits with code 5 and prints what it would create, marked `dry_run`. Run it again with `--confirm`:

```sh
apple-reminders add --list "Agent Tutorial" --title "Water the plants" --due tomorrow --confirm
apple-reminders list --list "Agent Tutorial"
```

The new reminder appears, and you can see it in the Reminders app as well.

## Clean up

```sh
apple-reminders list --list "Agent Tutorial" --status all --fields id
apple-reminders delete <id-from-the-previous-line> --confirm
apple-reminders list-delete "Agent Tutorial" --confirm
rm ~/.config/agent-cli/apple-reminders.json
```

## What you have done

You installed `apple-reminders` and read data with it. You saw that it refuses to write until a policy allows it, previewed a change with `--dry-run`, and applied it with `--confirm`.

Next steps:

- [How to configure permissions](../how-to/configure-permissions.md) when you want an agent to use the CLI
- [CLI reference](../reference/cli.md) for every command and flag
