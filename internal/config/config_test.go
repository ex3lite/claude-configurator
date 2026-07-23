package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSetGetUnset(t *testing.T) {
	data := map[string]any{
		"unknown": map[string]any{"keep": true},
		"env":     map[string]any{"OTHER": "value"},
	}
	Set(data, "env.CLAUDE_CODE_SUBAGENT_MODEL", "sonnet")
	if got, ok := Get(data, "env.CLAUDE_CODE_SUBAGENT_MODEL"); !ok || got != "sonnet" {
		t.Fatalf("Get() = %v, %v", got, ok)
	}
	Unset(data, "env.CLAUDE_CODE_SUBAGENT_MODEL")
	if _, ok := Get(data, "env.CLAUDE_CODE_SUBAGENT_MODEL"); ok {
		t.Fatal("Unset did not remove value")
	}
	if got, _ := Get(data, "env.OTHER"); got != "value" {
		t.Fatal("Unset removed sibling value")
	}
	if got, _ := Get(data, "unknown.keep"); got != true {
		t.Fatal("unknown setting was changed")
	}
}

func TestDocumentSaveBackupAndConflict(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("{\n  \"unknown\": {\"keep\": true},\n  \"model\": \"opus\"\n}\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	next := Clone(doc.Data)
	Set(next, "model", "fable")
	backups := filepath.Join(root, "backups")
	if err := doc.Save(next, backups, Global); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := Get(reloaded.Data, "model"); got != "fable" {
		t.Fatalf("model = %v", got)
	}
	if got, _ := Get(reloaded.Data, "unknown.keep"); got != true {
		t.Fatal("unknown setting was not preserved")
	}
	foundBackup := false
	_ = filepath.WalkDir(backups, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			raw, readErr := os.ReadFile(path)
			if readErr == nil && string(raw) == string(original) {
				foundBackup = true
			}
		}
		return nil
	})
	if !foundBackup {
		t.Fatal("original file was not backed up")
	}

	if err := os.WriteFile(path, []byte(`{"model":"sonnet"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := doc.Save(next, backups, Global); !errors.Is(err, ErrConflict) {
		t.Fatalf("Save() error = %v, want ErrConflict", err)
	}
}

func TestNewDocumentAddsSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	Set(doc.Data, "model", "fable")
	if err := doc.Save(doc.Data, filepath.Join(t.TempDir(), "backups"), Project); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := Get(reloaded.Data, "$schema"); got != schemaURL {
		t.Fatalf("$schema = %v", got)
	}
}

func TestReadOnlyDocumentIsNotReplaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"model":"opus"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	next := Clone(doc.Data)
	Set(next, "model", "fable")
	if err := doc.Save(next, t.TempDir(), Global); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("Save() error = %v, want read-only error", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"model":"opus"}` {
		t.Fatalf("read-only file changed: %s", raw)
	}
}

func TestBackupRetention(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	if err := os.WriteFile(path, []byte(`{"model":"opus"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	backups := filepath.Join(root, "backups")
	for i := range 12 {
		next := Clone(doc.Data)
		Set(next, "model", "model-"+string(rune('a'+i)))
		if err := doc.Save(next, backups, Global); err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	if err := filepath.WalkDir(backups, func(_ string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			count++
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if count != 10 {
		t.Fatalf("backup count = %d, want 10", count)
	}
}

func TestInvalidJSONHasLocation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{\n  \"model\":,\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("Load() error = %v, want ParseError", err)
	}
	if parseErr.Line != 2 {
		t.Fatalf("line = %d, want 2", parseErr.Line)
	}
}

func TestEffectiveValues(t *testing.T) {
	w := &Workspace{Docs: map[Scope]*Document{
		Global:  {Data: map[string]any{"model": "fable", "permissions": map[string]any{"deny": []any{"Read(.env)"}}}},
		Project: {Data: map[string]any{"model": "sonnet", "permissions": map[string]any{"deny": []any{"Bash(curl *)"}}}},
		Local:   {Data: map[string]any{}},
	}}
	if got, source, ok := w.Effective(Local, "model", false); !ok || got != "sonnet" || source != "project" {
		t.Fatalf("Effective(model) = %v, %q, %v", got, source, ok)
	}
	got, source, ok := w.Effective(Local, "permissions.deny", true)
	if !ok || source != "global+project" || len(got.([]any)) != 2 {
		t.Fatalf("Effective(deny) = %#v, %q, %v", got, source, ok)
	}
}

func TestResolvePathsGitAndWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	root := t.TempDir()
	home := filepath.Join(root, "home")
	repo := filepath.Join(root, "repo")
	worktree := filepath.Join(root, "worktree")
	mustRun(t, root, "git", "init", repo)
	mustRun(t, repo, "git", "config", "user.email", "test@example.com")
	mustRun(t, repo, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, repo, "git", "add", "README")
	mustRun(t, repo, "git", "commit", "-m", "init")
	mustRun(t, repo, "git", "worktree", "add", worktree, "-b", "worktree-test")

	nested := filepath.Join(worktree, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	paths, err := ResolvePaths(home, nested)
	if err != nil {
		t.Fatal(err)
	}
	if clean(paths.ProjectRoot) != clean(realPath(worktree)) {
		t.Fatalf("ProjectRoot = %q, want %q", paths.ProjectRoot, worktree)
	}
	if clean(paths.LocalRoot) != clean(realPath(repo)) {
		t.Fatalf("LocalRoot = %q, want %q", paths.LocalRoot, repo)
	}
	if paths.Global != filepath.Join(home, ".claude", "settings.json") {
		t.Fatalf("Global = %q", paths.Global)
	}
}

func TestLocalSaveAddsGitExclude(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	mustRun(t, root, "git", "init", repo)
	workspace, err := LoadWorkspace(home, repo)
	if err != nil {
		t.Fatal(err)
	}
	data := Clone(workspace.Docs[Local].Data)
	Set(data, "model", "sonnet")
	if err := workspace.Save(Local, data, filepath.Join(root, "backups")); err != nil {
		t.Fatal(err)
	}
	exclude, ok := gitOutput(repo, "rev-parse", "--path-format=absolute", "--git-path", "info/exclude")
	if !ok {
		t.Fatal("cannot resolve exclude")
	}
	raw, err := os.ReadFile(exclude)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), ".claude/settings.local.json") {
		t.Fatalf("exclude = %q", raw)
	}
}

func mustRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func clean(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func realPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}
