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
