// Package osa runs static JavaScript for Automation (JXA) scripts through
// /usr/bin/osascript. User input never becomes script source: the script is a
// compile-time constant and input travels as one JSON argv element that the
// script decodes with JSON.parse. Copied from agent-cli-plugins/scaffold.
package osa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/Bam679/apple-reminders-cli/internal/agentcli"
)

// Osascript is the absolute path of the interpreter (never resolved via $PATH).
const Osascript = "/usr/bin/osascript"

// MaxPayload caps the JSON argument (ARG_MAX on macOS is 1 MiB in total).
const MaxPayload = 256 << 10

// Prelude wraps every tool script: it decodes argv[0], dispatches to
// OPS[op] and encodes the result envelope. Tool scripts define `const OPS`.
const Prelude = `
function fail(code, message) { const e = new Error(message); e.agentCode = code; throw e; }
function run(argv) {
  let req;
  try { req = JSON.parse(argv[0]); } catch (e) { return JSON.stringify({ok:false, error:{code:"usage", message:"bad request"}}); }
  const fn = OPS[req.op];
  if (typeof fn !== "function") return JSON.stringify({ok:false, error:{code:"usage", message:"unknown op"}});
  try {
    return JSON.stringify({ok:true, data: fn(req.args || {})});
  } catch (e) {
    return JSON.stringify({ok:false, error:{code: e.agentCode || "internal", message: String(e && e.message || e)}});
  }
}
`

// Runner executes an operation of a static script.
type Runner interface {
	Run(ctx context.Context, script, op string, args, out any) error
}

// Exec is the real Runner backed by osascript.
type Exec struct{}

type request struct {
	Op   string `json:"op"`
	Args any    `json:"args"`
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Argv builds the osascript argument vector. It is exported for fuzzing: the
// script must appear verbatim and the payload must be exactly one element.
func Argv(script, op string, args any) ([]string, error) {
	payload, err := json.Marshal(request{Op: op, Args: args})
	if err != nil {
		return nil, agentcli.Usagef("cannot encode request: %v", err)
	}
	if len(payload) > MaxPayload {
		return nil, agentcli.Usagef("request too large (%d bytes, max %d)", len(payload), MaxPayload)
	}
	return []string{"-l", "JavaScript", "-e", Prelude + script, string(payload)}, nil
}

// Run executes op and decodes the result data into out (may be nil).
func (Exec) Run(ctx context.Context, script, op string, args, out any) error {
	argv, err := Argv(script, op, args)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, Osascript, argv...) // #nosec G204 -- fixed binary, static script, JSON argv
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitWriter{w: &stdout, n: 64 << 20}
	cmd.Stderr = &limitWriter{w: &stderr, n: 64 << 10}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return agentcli.Unavailablef("osascript timed out or was cancelled")
		}
		return MapStderr(stderr.String())
	}
	return Decode(stdout.Bytes(), out)
}

// Decode parses a script envelope and maps script error codes.
func Decode(raw []byte, out any) error {
	var env envelope
	if err := json.Unmarshal(bytes.TrimSpace(raw), &env); err != nil {
		return agentcli.Internalf("unexpected osascript output")
	}
	if !env.OK {
		if env.Error == nil {
			return agentcli.Internalf("script failed without error")
		}
		msg := env.Error.Message
		switch env.Error.Code {
		case "not_found":
			return agentcli.NotFoundf("%s", msg)
		case "conflict":
			return agentcli.Conflictf("%s", msg)
		case "usage":
			return agentcli.Usagef("%s", msg)
		case "permission":
			return agentcli.Permissionf("%s", msg)
		default:
			return MapStderr(msg)
		}
	}
	if out == nil || len(env.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return agentcli.Internalf("cannot decode script result: %v", err)
	}
	return nil
}

// MapStderr maps osascript/Apple Event error text to a structured error.
func MapStderr(s string) error {
	s = strings.TrimSpace(s)
	switch {
	case strings.Contains(s, "-1743"), strings.Contains(s, "Not authorized"), strings.Contains(s, "nicht berechtigt"):
		return agentcli.Permissionf("automation access denied: allow your terminal under Privacy & Security > Automation")
	case strings.Contains(s, "-1712"):
		return agentcli.Unavailablef("the app did not respond in time (AppleEvent timeout)")
	case strings.Contains(s, "-600"), strings.Contains(s, "-609"):
		return agentcli.Unavailablef("the app is not running or quit unexpectedly")
	case s == "":
		return agentcli.Internalf("osascript failed")
	}
	return agentcli.Internalf("osascript: %s", s)
}

type limitWriter struct {
	w io.Writer
	n int
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if len(p) > l.n {
		return 0, errors.New("output limit exceeded")
	}
	l.n -= len(p)
	return l.w.Write(p)
}

// Fake is an in-memory Runner for unit tests: Handler receives the op and the
// JSON-encoded args and returns JSON data or an error.
type Fake struct {
	Handler func(op string, args json.RawMessage) (any, error)
	Calls   []string
}

// Run implements Runner.
func (f *Fake) Run(_ context.Context, _ string, op string, args, out any) error {
	f.Calls = append(f.Calls, op)
	raw, err := json.Marshal(args)
	if err != nil {
		return err
	}
	data, err := f.Handler(op, raw)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("fake: %w", err)
	}
	return json.Unmarshal(b, out)
}
