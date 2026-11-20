## macOS privacy permission

`apple-reminders` reaches Reminders through EventKit, which it runs via `/usr/bin/osascript`. macOS gives the permission to the app that runs the CLI: Terminal, iTerm, VS Code or Claude. On first use macOS asks for Reminders access. If you declined, open System Settings > Privacy & Security > Reminders and switch on your terminal app. `apple-reminders doctor` shows whether access works; without it, commands exit with code 6.
