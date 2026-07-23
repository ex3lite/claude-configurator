package main

import (
	"bytes"
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
