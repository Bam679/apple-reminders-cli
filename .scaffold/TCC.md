## macOS privacy (TCC)

`apple-reminders` uses EventKit through `/usr/bin/osascript`. macOS attributes the access to the **app that runs the CLI** (Terminal, iTerm, VS Code, Claude desktop …). On first use, macOS asks for **Reminders** access. If you denied it, enable your terminal app under **System Settings > Privacy & Security > Reminders** (full access). `apple-reminders doctor` reports the status, and errors use exit code 6 with a hint.
