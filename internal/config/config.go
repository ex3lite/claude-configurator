package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const schemaURL = "https://json.schemastore.org/claude-code-settings.json"

type Scope string

const (
	Global  Scope = "global"
	Project Scope = "project"
	Local   Scope = "local"
)

var ErrConflict = errors.New("settings file changed since it was loaded")

type Paths struct {
	Start       string
	ProjectRoot string
	LocalRoot   string
	Global      string
	Project     string
	Local       string
	InGit       bool
}

func (p Paths) For(scope Scope) string {
	switch scope {
	case Global:
		return p.Global
	case Project:
		return p.Project
	case Local:
		return p.Local
	default:
		return ""
	}
}

func ResolvePaths(home, start string) (Paths, error) {
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return Paths{}, err
		}
	}
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return Paths{}, err
		}
	}
	home, err := filepath.Abs(home)
	if err != nil {
		return Paths{}, err
	}
	start, err = filepath.Abs(start)
	if err != nil {
		return Paths{}, err
	}

	p := Paths{Start: start, ProjectRoot: start, LocalRoot: start}
	if root, ok := gitOutput(start, "rev-parse", "--show-toplevel"); ok {
		p.InGit = true
		p.ProjectRoot = filepath.Clean(root)
		p.LocalRoot = p.ProjectRoot

		if samePath(p.ProjectRoot, home) {
			p.LocalRoot = start
		} else if common, ok := gitOutput(start, "rev-parse", "--path-format=absolute", "--git-common-dir"); ok {
			common = filepath.Clean(common)
			if filepath.Base(common) == ".git" {
				p.LocalRoot = filepath.Dir(common)
			}
		}
	}

	p.Global = filepath.Join(home, ".claude", "settings.json")
	p.Project = filepath.Join(p.ProjectRoot, ".claude", "settings.json")
	p.Local = filepath.Join(p.LocalRoot, ".claude", "settings.local.json")
	return p, nil
}

func gitOutput(dir string, args ...string) (string, bool) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

type ParseError struct {
	Path   string
	Line   int
	Column int
	Err    error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s:%d:%d: %v", e.Path, e.Line, e.Column, e.Err)
}

func (e *ParseError) Unwrap() error { return e.Err }

type Document struct {
	Path         string
	Data         map[string]any
	Exists       bool
	Original     []byte
	OriginalHash [32]byte
	Mode         fs.FileMode
}

func Load(path string) (*Document, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Document{
			Path: path,
			Data: map[string]any{"$schema": schemaURL},
			Mode: 0o600,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	data, err := decode(path, raw)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &Document{
		Path:         path,
		Data:         data,
		Exists:       true,
		Original:     append([]byte(nil), raw...),
		OriginalHash: sha256.Sum256(raw),
		Mode:         info.Mode().Perm(),
	}, nil
}

func decode(path string, raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var data map[string]any
	if err := decoder.Decode(&data); err != nil {
		return nil, parseError(path, raw, err)
	}
	if data == nil {
		return nil, &ParseError{Path: path, Line: 1, Column: 1, Err: errors.New("root must be a JSON object")}
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return nil, &ParseError{Path: path, Line: 1, Column: 1, Err: errors.New("multiple JSON values")}
	} else if !errors.Is(err, io.EOF) {
		return nil, parseError(path, raw, err)
	}
	return data, nil
}

func parseError(path string, raw []byte, err error) error {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		line, column := lineColumn(raw, syntax.Offset)
		return &ParseError{Path: path, Line: line, Column: column, Err: err}
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		line, column := lineColumn(raw, typeErr.Offset)
		return &ParseError{Path: path, Line: line, Column: column, Err: err}
	}
	return &ParseError{Path: path, Line: 1, Column: 1, Err: err}
}

func lineColumn(raw []byte, offset int64) (int, int) {
	if offset < 1 {
		return 1, 1
	}
	i := int(offset - 1)
	if i > len(raw) {
		i = len(raw)
	}
	line := bytes.Count(raw[:i], []byte{'\n'}) + 1
	last := bytes.LastIndexByte(raw[:i], '\n')
	return line, i - last
}

func Clone(data map[string]any) map[string]any {
	raw, _ := json.Marshal(data)
	var clone map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&clone)
	return clone
}

func Get(data map[string]any, path string) (any, bool) {
	var current any = data
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func Set(data map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func Unset(data map[string]any, path string) {
	unset(data, strings.Split(path, "."))
}

func unset(data map[string]any, parts []string) bool {
	if len(parts) == 1 {
		delete(data, parts[0])
		return len(data) == 0
	}
	child, ok := data[parts[0]].(map[string]any)
	if !ok {
		return len(data) == 0
	}
	if unset(child, parts[1:]) {
		delete(data, parts[0])
	}
	return len(data) == 0
}

func Equal(a, b any) bool {
	return bytes.Equal(mustJSON(a), mustJSON(b))
}

func mustJSON(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

type Workspace struct {
	Paths Paths
	Docs  map[Scope]*Document
}

func LoadWorkspace(home, start string) (*Workspace, error) {
	paths, err := ResolvePaths(home, start)
	if err != nil {
		return nil, err
	}
	workspace := &Workspace{Paths: paths, Docs: make(map[Scope]*Document, 3)}
	for _, scope := range []Scope{Global, Project, Local} {
		doc, err := Load(paths.For(scope))
		if err != nil {
			return nil, err
		}
		workspace.Docs[scope] = doc
	}
	return workspace, nil
}

func (w *Workspace) Reload() error {
	loaded, err := LoadWorkspace(filepath.Dir(filepath.Dir(w.Paths.Global)), w.Paths.Start)
	if err != nil {
		return err
	}
	*w = *loaded
	return nil
}

func (w *Workspace) Effective(scope Scope, path string, merge bool) (any, string, bool) {
	scopes := []Scope{Global}
	if scope == Project || scope == Local {
		scopes = append(scopes, Project)
	}
	if scope == Local {
		scopes = append(scopes, Local)
	}
	if merge {
		var combined []any
		var sources []string
		seen := make(map[string]bool)
		for _, candidate := range scopes {
			value, ok := Get(w.Docs[candidate].Data, path)
			if !ok {
				continue
			}
			items, ok := value.([]any)
			if !ok {
				continue
			}
			for _, item := range items {
				key := string(mustJSON(item))
				if !seen[key] {
					combined = append(combined, item)
					seen[key] = true
				}
			}
			sources = append(sources, string(candidate))
		}
		if len(sources) > 0 {
			return combined, strings.Join(sources, "+"), true
		}
		return nil, "", false
	}
	for i := len(scopes) - 1; i >= 0; i-- {
		if value, ok := Get(w.Docs[scopes[i]].Data, path); ok {
			return value, string(scopes[i]), true
		}
	}
	return nil, "", false
}

func (w *Workspace) Save(scope Scope, data map[string]any, backupRoot string) error {
	doc := w.Docs[scope]
	if scope == Local {
		if err := ensureLocalIgnored(w.Paths); err != nil {
			return fmt.Errorf("prepare Git exclude: %w", err)
		}
	}
	if err := doc.Save(data, backupRoot, scope); err != nil {
		return err
	}
	return nil
}

func (d *Document) Save(data map[string]any, backupRoot string, scope Scope) error {
	if d.Exists {
		current, err := os.ReadFile(d.Path)
		if err != nil {
			return err
		}
		if sha256.Sum256(current) != d.OriginalHash {
			return ErrConflict
		}
		info, err := os.Stat(d.Path)
		if err != nil {
			return err
		}
		if info.Mode().Perm()&0o200 == 0 {
			return fmt.Errorf("%s is read-only", d.Path)
		}
	} else if _, err := os.Stat(d.Path); err == nil {
		return ErrConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if d.Exists {
		if err := backup(d.Path, d.Original, backupRoot); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
	}
	if err := atomicWrite(d.Path, raw, fileMode(d, scope)); err != nil {
		return err
	}
	info, err := os.Stat(d.Path)
	if err != nil {
		return err
	}
	d.Data = Clone(data)
	d.Exists = true
	d.Original = append(d.Original[:0], raw...)
	d.OriginalHash = sha256.Sum256(raw)
	d.Mode = info.Mode().Perm()
	return nil
}

func fileMode(d *Document, scope Scope) fs.FileMode {
	if d.Exists && d.Mode != 0 {
		return d.Mode
	}
	if scope == Project {
		return 0o644
	}
	return 0o600
}

func atomicWrite(path string, raw []byte, mode fs.FileMode) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer func() {
		_ = temp.Close()
		if err != nil {
			_ = os.Remove(name)
		}
	}()
	if err = temp.Chmod(mode); err != nil {
		return err
	}
	if _, err = temp.Write(raw); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func backup(path string, raw []byte, root string) error {
	if root == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		root = filepath.Join(cache, "claude-configurator", "backups")
	}
	sum := sha256.Sum256([]byte(filepath.Clean(path)))
	dir := filepath.Join(root, hex.EncodeToString(sum[:8]))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	name := time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + filepath.Base(path)
	if err := os.WriteFile(filepath.Join(dir, name), raw, 0o600); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
	if len(entries) > 10 {
		for _, entry := range entries[10:] {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
	return nil
}

func ensureLocalIgnored(paths Paths) error {
	if !paths.InGit {
		return nil
	}
	relative, err := filepath.Rel(paths.ProjectRoot, paths.Local)
	if err != nil || strings.HasPrefix(relative, "..") {
		relative = ".claude/settings.local.json"
	}
	relative = filepath.ToSlash(relative)
	check := exec.Command("git", "-C", paths.ProjectRoot, "check-ignore", "-q", "--", relative)
	if check.Run() == nil {
		return nil
	}
	exclude, ok := gitOutput(paths.ProjectRoot, "rev-parse", "--path-format=absolute", "--git-path", "info/exclude")
	if !ok {
		return errors.New("cannot resolve Git exclude file")
	}
	raw, err := os.ReadFile(exclude)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	line := []byte(relative + "\n")
	if bytes.Contains(append([]byte{'\n'}, raw...), append([]byte{'\n'}, line...)) {
		return nil
	}
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		raw = append(raw, '\n')
	}
	raw = append(raw, line...)
	if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		return err
	}
	return os.WriteFile(exclude, raw, 0o644)
}
