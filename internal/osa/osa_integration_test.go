//go:build integration && darwin

package osa

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestRealOsascriptRoundTrip proves that hostile strings survive the real
// osascript boundary as data: the script only ever echoes its JSON input.
func TestRealOsascriptRoundTrip(t *testing.T) {
	hostile := []string{
		`"); do shell script "touch /tmp/agent-cli-pwned"; ("`,
		"`$(id)` ${HOME} \\ ' \" \n \t     🙂",
		"-e", "--", "</script>", strings.Repeat("ä", 2000), "\x01\x02\x7f",
	}
	for _, s := range hostile {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		var out map[string]string
		err := Exec{}.Run(ctx, echoScript, "echo", map[string]string{"s": s}, &out)
		cancel()
		if err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		if out["s"] != s {
			t.Fatalf("roundtrip mismatch for %q: %q", s, out["s"])
		}
	}
}

func TestRealOsascriptErrors(t *testing.T) {
	err := Exec{}.Run(context.Background(), `const OPS = { f: function () { fail("not_found", "nope"); } };`, "f", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("expected not_found, got %v", err)
	}
	err = Exec{}.Run(context.Background(), echoScript, "missing", nil, nil)
	if err == nil {
		t.Fatal("unknown op must fail")
	}
}
