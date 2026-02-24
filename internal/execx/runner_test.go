package execx

import (
	"bytes"
	"strings"
	"testing"
)

func TestOSRunnerBasics(t *testing.T) {
	r := OSRunner{}
	if _, err := r.LookPath("sh"); err != nil {
		t.Fatalf("expected sh in path: %v", err)
	}

	out, err := r.Output("sh", "-c", "printf paw")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "paw" {
		t.Fatalf("unexpected output: %q", string(out))
	}

	combined, err := r.CombinedOutput("sh", "-c", "printf test")
	if err != nil {
		t.Fatal(err)
	}
	if string(combined) != "test" {
		t.Fatalf("unexpected combined output: %q", string(combined))
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := r.RunWith("sh", []string{"-c", "printf ok >&2"}, CommandOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "ok") {
		t.Fatalf("expected stderr output, got %q", stderr.String())
	}
}
