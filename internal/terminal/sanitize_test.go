package terminal

import "testing"

func TestEscapeControls(t *testing.T) {
	got := EscapeControls("ok\x1b[31m\x7f\u0085\u202e")
	want := `ok\x1B[31m\x7F\x85\u202E`
	if got != want {
		t.Fatalf("EscapeControls() = %q, want %q", got, want)
	}
}
