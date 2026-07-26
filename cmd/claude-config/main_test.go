package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionAndValidation(t *testing.T) {
	old := version
	version = "v0.1.0-test"
	t.Cleanup(func() { version = old })

	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("--version exit = %d, stderr = %s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "v0.1.0-test" {
		t.Fatalf("stdout = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("--help exit = %d", code)
	}
	if !strings.Contains(stderr.String(), "--no-update") {
		t.Fatalf("--help does not document update opt-out: %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--scope", "system"}, &stdout, &stderr); code != 2 {
		t.Fatalf("invalid scope exit = %d", code)
	}
	if !strings.Contains(stderr.String(), "invalid scope") {
		t.Fatalf("stderr = %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	missing := filepath.Join(t.TempDir(), "missing")
	if code := run([]string{"--project", missing}, &stdout, &stderr); code != 2 {
		t.Fatalf("missing project exit = %d", code)
	}
	if !strings.Contains(stderr.String(), "invalid project path") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestStatuslineSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LC_ALL", "en_US.UTF-8")
	input := strings.NewReader(fmt.Sprintf(
		`{"cwd":%q,"model":{"display_name":"Fable 5"},"context_window":{"remaining_percentage":97}}`,
		t.TempDir(),
	))
	if code := runStatusline([]string{"--theme", "mono"}, input, &stdout, &stderr); code != 0 {
		t.Fatalf("statusline exit = %d, stderr = %s", code, stderr.String())
	}
	if output := stdout.String(); !strings.Contains(output, "Fable 5") || !strings.Contains(output, "ctx:97% left") {
		t.Fatalf("statusline output = %q", output)
	}
}
