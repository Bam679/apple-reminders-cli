# ADR-0004: Static JXA through osascript, with EventKit for Calendar and Reminders

- Status: accepted
- Date: 2026-11-20

## Context

Apple apps expose their data through AppleScript and JXA (Mail, Notes) and through EventKit (Calendar, Reminders). The options were cgo with Objective-C, a Swift helper, or `osascript`. One rule was fixed from the start: scripts are never built by joining strings.

## Decision

The CLIs run `/usr/bin/osascript -l JavaScript -e <constant script> <one JSON argument>`. Scripts are `go:embed` constants. A shared prelude decodes `argv[0]` with `JSON.parse` and calls `OPS[op]`, so input is only ever data. Fuzz tests check that the argument vector and the script stay the same for any input, and an integration test sends hostile strings through the real `osascript` and back.

Calendar and Reminders use EventKit through the JXA Objective-C bridge (`ObjC.import('EventKit')`). It is fast, has stable identifiers and doesn't need the app to run. Mail and Notes use their scripting dictionaries.

Without cgo, release builds cross-compile with `CGO_ENABLED=0`.

macOS attributes privacy permissions (TCC) to the app that runs the CLI, such as Terminal, iTerm, VS Code or Claude. The `doctor` command reports the state, and exit code 6 comes with a hint that names the System Settings pane.

## Consequences

There is no native code to compile, the build is simple, and Intel and Apple Silicon get the same binary. Each call costs about 100 to 250 ms to start `osascript`, which is acceptable for agent use. List operations read properties in bulk to keep large mailboxes and note collections fast.
