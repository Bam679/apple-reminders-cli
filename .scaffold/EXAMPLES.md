```sh
apple-reminders list --due-before tomorrow              # what's due today (one line each)
apple-reminders search dentist --json --fields id,title,due
apple-reminders add --list "Agent Inbox" --title "Call Bob" --due 2026-12-01T09:00 --dry-run
apple-reminders add --list "Agent Inbox" --title "Call Bob" --due 2026-12-01T09:00 --confirm
printf '%s\n' '{"list":"Shopping","title":"Milk"}' '{"list":"Shopping","title":"Eggs"}' \
  | apple-reminders add - --confirm --ndjson                # batch via stdin
apple-reminders complete 7FB0…-4B42 --confirm
apple-reminders list --limit 20 --cursor "$(apple-reminders list --limit 20 --json | jq -r .next_cursor)"
```
