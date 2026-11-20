# ADR-0004: macOS bridge: static JXA via osascript, EventKit for Calendar/Reminders

- Status: accepted
- Date: 2026-11-20

## Context
Apple apps expose data via AppleScript/JXA (Mail, Notes) and EventKit (Calendar, Reminders). The options are cgo/Objective-C, a Swift helper, or `osascript`. The hard rule is that scripts are never built by string concatenation.

## Decision
- Exec `/usr/bin/osascript -l JavaScript -e <constant script> <one JSON argument>`. Scripts are `go:embed` constants with a shared prelude that `JSON.parse`s `argv[0]` and dispatches to `OPS[op]`. Input is only data, and fuzz tests check that the argv shape and script are invariant for arbitrary input. An integration test round-trips hostile strings through the real osascript.
- Calendar/Reminders use EventKit through the JXA ObjC bridge (`ObjC.import('EventKit')`). This is fast, has stable `calendarItemIdentifier`s and needs no Calendar.app UI. Mail/Notes use their scripting dictionaries.
- No cgo, so release builds cross-compile with `CGO_ENABLED=0`.
- TCC: permissions attach to the responsible app (Terminal, iTerm, VS Code, Claude desktop). `doctor` reports the status and the exit code 6 hint names the System Settings pane.

## Consequences
+ No compiled native code, a simple build, and identical binaries for amd64/arm64.
- Each call costs about 100–250 ms of osascript start-up. That's acceptable for agent use, and list operations batch their property reads.
