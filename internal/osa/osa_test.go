package osa

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Bam679/apple-reminders-cli/internal/agentcli"
)

const echoScript = `const OPS = { echo: function (a) { return a; } };`

func checkArgv(t *testing.T, s string) {
	t.Helper()
	argv, err := Argv(echoScript, "echo", map[string]string{"s": s})
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 5 || argv[0] != "-l" || argv[1] != "JavaScript" || argv[2] != "-e" {
		t.Fatalf("unexpected argv shape: %q", argv[:3])
	}
	if argv[3] != Prelude+echoScript {
		t.Fatal("script source must never contain input")
	}
	if strings.ContainsRune(argv[4], 0) || !strings.HasPrefix(argv[4], "{") {
		t.Fatalf("payload must be a single JSON object without NUL: %q", argv[4])
	}
	var req struct {
		Op   string            `json:"op"`
		Args map[string]string `json:"args"`
	}
	if err := json.Unmarshal([]byte(argv[4]), &req); err != nil || req.Op != "echo" {
		t.Fatalf("payload does not decode: %v", err)
	}
	if want := strings.ToValidUTF8(s, "�"); req.Args["s"] != want {
		t.Fatalf("roundtrip mismatch: %q != %q", req.Args["s"], want)
	}
}

func FuzzArgv(f *testing.F) {
	for _, s := range []string{`"); do shell script "rm -rf ~"; ("`, "`$(id)`", "\x00", "'\\", " ", "-e", "--", "</script>"} {
		f.Add(s)
	}
	f.Fuzz(checkArgv)
}

func TestArgvTooLarge(t *testing.T) {
	if _, err := Argv(echoScript, "echo", strings.Repeat("x", MaxPayload)); err == nil {
		t.Fatal("oversized payload accepted")
	}
	if _, err := Argv(echoScript, "echo", func() {}); err == nil {
		t.Fatal("unencodable args accepted")
	}
}

func TestDecode(t *testing.T) {
	var out map[string]int
	if err := Decode([]byte(`{"ok":true,"data":{"a":1}}`+"\n"), &out); err != nil || out["a"] != 1 {
		t.Fatalf("decode ok: %v %v", out, err)
	}
	if err := Decode([]byte(`{"ok":true}`), nil); err != nil {
		t.Fatal(err)
	}
	cases := map[string]int{
		`{"ok":false,"error":{"code":"not_found","message":"m"}}`:    agentcli.ExitNotFound,
		`{"ok":false,"error":{"code":"conflict","message":"m"}}`:     agentcli.ExitConflict,
		`{"ok":false,"error":{"code":"usage","message":"m"}}`:        agentcli.ExitUsage,
		`{"ok":false,"error":{"code":"permission","message":"m"}}`:   agentcli.ExitPermission,
		`{"ok":false,"error":{"code":"internal","message":"-1743"}}`: agentcli.ExitPermission,
		`{"ok":false}`: agentcli.ExitError,
		`not json`:     agentcli.ExitError,
	}
	for raw, want := range cases {
		if e := agentcli.AsError(Decode([]byte(raw), nil)); e == nil || e.Exit != want {
			t.Errorf("%s: got %v want exit %d", raw, e, want)
		}
	}
	if err := Decode([]byte(`{"ok":true,"data":"str"}`), &out); err == nil {
		t.Fatal("type mismatch must fail")
	}
}

func TestMapStderr(t *testing.T) {
	cases := map[string]int{
		"execution error: Not authorized to send Apple events to Reminders. (-1743)": agentcli.ExitPermission,
		"AppleEvent timed out. (-1712)":                                              agentcli.ExitUnavailable,
		"Application isn't running. (-600)":                                          agentcli.ExitUnavailable,
		"":                                                                           agentcli.ExitError,
		"some other failure":                                                         agentcli.ExitError,
	}
	for s, want := range cases {
		if e := agentcli.AsError(MapStderr(s)); e.Exit != want {
			t.Errorf("%q: exit %d want %d", s, e.Exit, want)
		}
	}
}

func TestFake(t *testing.T) {
	f := &Fake{Handler: func(op string, args json.RawMessage) (any, error) {
		if op == "boom" {
			return nil, errors.New("boom")
		}
		return map[string]string{"op": op, "args": string(args)}, nil
	}}
	var out map[string]string
	if err := f.Run(context.Background(), "", "x", map[string]int{"a": 1}, &out); err != nil || out["args"] != `{"a":1}` {
		t.Fatalf("fake: %v %v", out, err)
	}
	if err := f.Run(context.Background(), "", "boom", nil, nil); err == nil {
		t.Fatal("expected error")
	}
	if err := f.Run(context.Background(), "", "x", nil, nil); err != nil || len(f.Calls) != 3 {
		t.Fatal("calls not recorded")
	}
}

func TestLimitWriter(t *testing.T) {
	var sb strings.Builder
	w := &limitWriter{w: &sb, n: 3}
	if _, err := w.Write([]byte("ab")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("cd")); err == nil {
		t.Fatal("limit not enforced")
	}
}
