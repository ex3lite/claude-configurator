package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	latestReleaseURL = "https://api.github.com/repos/ex3lite/claude-configurator/releases/latest"
	helperCommand    = "__claude_config_apply_update"
	helperCleanupEnv = "CLAUDE_CONFIG_UPDATE_HELPER"
	maxMetadataSize  = 2 << 20
	maxChecksumSize  = 1 << 20
	maxArchiveSize   = 128 << 20
	maxBinarySize    = 128 << 20
)

type Asset struct {
	Name   string
	URL    string
	Digest string
}

type Release struct {
	Version   string
	Tag       string
	PageURL   string
	Archive   Asset
	Checksums Asset
}

type Prepared struct {
	Version string
	Binary  string
	Helper  string
	Target  string
}

type Client struct {
	currentVersion string
	apiURL         string
	httpClient     *http.Client
	executable     func() (string, error)
	cacheDir       func() (string, error)
}

func New(currentVersion string) *Client {
	return &Client{
		currentVersion: currentVersion,
		apiURL:         latestReleaseURL,
		httpClient:     &http.Client{Timeout: 2 * time.Minute},
		executable:     os.Executable,
		cacheDir:       os.UserCacheDir,
	}
}

func Enabled(version string) bool {
	if os.Getenv("CLAUDE_CONFIG_NO_UPDATE") != "" {
		return false
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		return false
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return false
	}
	_, ok := parseVersion(version)
	return ok
}

func (c *Client) Check(ctx context.Context) (Release, bool, error) {
	current, ok := parseVersion(c.currentVersion)
	if !ok {
		return Release{}, false, nil
	}

	var response struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Digest             string `json:"digest"`
		} `json:"assets"`
	}
	if err := c.getJSON(ctx, c.apiURL, &response); err != nil {
		return Release{}, false, err
	}
	latest, ok := parseVersion(response.TagName)
	if !ok {
		return Release{}, false, fmt.Errorf("latest release has an invalid version %q", response.TagName)
	}
	if compareVersion(latest, current) <= 0 {
		return Release{}, false, nil
	}

	release := Release{
		Version: versionString(latest),
		Tag:     response.TagName,
		PageURL: response.HTMLURL,
	}
	archiveName := releaseArchiveName(release.Version)
	for _, asset := range response.Assets {
		switch asset.Name {
		case archiveName:
			release.Archive = Asset{
				Name:   asset.Name,
				URL:    asset.BrowserDownloadURL,
				Digest: asset.Digest,
			}
		case "checksums.txt":
			release.Checksums = Asset{Name: asset.Name, URL: asset.BrowserDownloadURL}
		}
	}
	if release.Archive.URL == "" {
		return Release{}, false, fmt.Errorf("release %s has no asset %s", release.Tag, archiveName)
	}
	if release.Checksums.URL == "" {
		return Release{}, false, fmt.Errorf("release %s has no checksums.txt", release.Tag)
	}
	return release, true, nil
}

func (c *Client) Prepare(ctx context.Context, release Release) (Prepared, error) {
	target, err := c.targetExecutable()
	if err != nil {
		return Prepared{}, err
	}
	if err := checkTargetDirectory(target); err != nil {
		return Prepared{}, err
	}

	checksums, err := c.downloadBytes(ctx, release.Checksums.URL, maxChecksumSize)
	if err != nil {
		return Prepared{}, fmt.Errorf("download checksums: %w", err)
	}
	expected, err := checksumFor(checksums, release.Archive.Name)
	if err != nil {
		return Prepared{}, err
	}
	if release.Archive.Digest != "" {
		digest := strings.TrimPrefix(strings.ToLower(release.Archive.Digest), "sha256:")
		if len(digest) == sha256.Size*2 && digest != expected {
			return Prepared{}, errors.New("release digest does not match checksums.txt")
		}
	}

	cacheRoot, err := c.cacheDir()
	if err != nil {
		return Prepared{}, fmt.Errorf("resolve update cache: %w", err)
	}
	updateDir := filepath.Join(cacheRoot, "claude-configurator", "updates", release.Version)
	if err := os.MkdirAll(updateDir, 0o700); err != nil {
		return Prepared{}, fmt.Errorf("create update cache: %w", err)
	}

	archive, err := os.CreateTemp(updateDir, ".archive-*")
	if err != nil {
		return Prepared{}, fmt.Errorf("create archive file: %w", err)
	}
	archivePath := archive.Name()
	archive.Close()
	defer os.Remove(archivePath)

	actual, err := c.downloadFile(ctx, release.Archive.URL, archivePath, maxArchiveSize)
	if err != nil {
		return Prepared{}, fmt.Errorf("download %s: %w", release.Archive.Name, err)
	}
	if actual != expected {
		return Prepared{}, fmt.Errorf("checksum mismatch for %s", release.Archive.Name)
	}

	pattern := ".claude-config-*"
	if runtime.GOOS == "windows" {
		pattern += ".exe"
	}
	binary, err := os.CreateTemp(updateDir, pattern)
	if err != nil {
		return Prepared{}, fmt.Errorf("create staged binary: %w", err)
	}
	binaryPath := binary.Name()
	binary.Close()
	if err := extractBinary(archivePath, release.Archive.Name, binaryPath); err != nil {
		os.Remove(binaryPath)
		return Prepared{}, err
	}
	if err := os.Chmod(binaryPath, 0o700); err != nil {
		os.Remove(binaryPath)
		return Prepared{}, fmt.Errorf("make staged binary executable: %w", err)
	}

	helperPattern := ".updater-*"
	if runtime.GOOS == "windows" {
		helperPattern += ".exe"
	}
	helper, err := os.CreateTemp(updateDir, helperPattern)
	if err != nil {
		os.Remove(binaryPath)
		return Prepared{}, fmt.Errorf("create update helper: %w", err)
	}
	helperPath := helper.Name()
	helper.Close()
	if err := copyExecutable(target, helperPath, 0o700); err != nil {
		os.Remove(binaryPath)
		os.Remove(helperPath)
		return Prepared{}, fmt.Errorf("stage update helper: %w", err)
	}
	return Prepared{
		Version: release.Version,
		Binary:  binaryPath,
		Helper:  helperPath,
		Target:  target,
	}, nil
}

func Launch(prepared Prepared, currentVersion string, originalArgs []string) error {
	if prepared.Binary == "" || prepared.Helper == "" || prepared.Target == "" {
		return errors.New("update is not prepared")
	}
	arguments := []string{
		helperCommand,
		prepared.Binary,
		prepared.Target,
		strconv.Itoa(os.Getpid()),
		currentVersion,
		"--",
	}
	arguments = append(arguments, originalArgs...)
	command := exec.Command(prepared.Helper, arguments...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Start()
}

func HandleHelper(args []string) (bool, error) {
	if len(args) == 0 || args[0] != helperCommand {
		return false, nil
	}
	if len(args) < 6 || args[5] != "--" {
		return true, errors.New("invalid internal update arguments")
	}
	parentPID, err := strconv.Atoi(args[3])
	if err != nil || parentPID <= 0 {
		return true, errors.New("invalid parent process id")
	}
	if err := waitForProcessExit(parentPID, 30*time.Second); err != nil {
		return true, fmt.Errorf("wait for current version to exit: %w", err)
	}

	if err := replaceExecutable(args[1], args[2]); err != nil {
		return true, fmt.Errorf("install update: %w", err)
	}
	_ = os.Remove(args[1])

	command := exec.Command(args[2], args[6:]...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	environment := withEnvironment(
		os.Environ(),
		"CLAUDE_CONFIG_UPDATED_FROM",
		args[4],
	)
	helperPath, _ := os.Executable()
	command.Env = withEnvironment(environment, helperCleanupEnv, helperPath)
	if err := command.Start(); err != nil {
		return true, fmt.Errorf("restart updated version: %w", err)
	}
	if err := os.Remove(helperPath); err == nil {
		_ = os.Remove(filepath.Dir(helperPath))
	}
	return true, nil
}

func CleanupStagedHelper() {
	path := os.Getenv(helperCleanupEnv)
	_ = os.Unsetenv(helperCleanupEnv)
	if !validHelperPath(path) {
		return
	}
	go func() {
		for range 50 {
			err := os.Remove(path)
			if err == nil || errors.Is(err, os.ErrNotExist) {
				_ = os.Remove(filepath.Dir(path))
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

func (c *Client) targetExecutable() (string, error) {
	target, err := c.executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve current executable path: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(target); resolveErr == nil {
		target = resolved
	}
	if _, err := os.Stat(target); err != nil {
		return "", fmt.Errorf("inspect current executable: %w", err)
	}
	return target, nil
}

func checkTargetDirectory(target string) error {
	probe, err := os.CreateTemp(filepath.Dir(target), ".claude-config-update-*")
	if err != nil {
		return fmt.Errorf("the executable directory is not writable: %w", err)
	}
	name := probe.Name()
	if closeErr := probe.Close(); closeErr != nil {
		os.Remove(name)
		return fmt.Errorf("test executable directory: %w", closeErr)
	}
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("clean executable directory test: %w", err)
	}
	return nil
}

func (c *Client) getJSON(ctx context.Context, url string, value any) error {
	data, err := c.downloadBytes(ctx, url, maxMetadataSize)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("decode release metadata: %w", err)
	}
	return nil
}

func (c *Client) downloadBytes(ctx context.Context, url string, limit int64) ([]byte, error) {
	request, err := c.request(ctx, url)
	if err != nil {
		return nil, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", response.Status)
	}
	if response.ContentLength > limit {
		return nil, errors.New("response is too large")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("response is too large")
	}
	return data, nil
}

func (c *Client) downloadFile(ctx context.Context, url, path string, limit int64) (string, error) {
	request, err := c.request(ctx, url)
	if err != nil {
		return "", err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %s", response.Status)
	}
	if response.ContentLength > limit {
		return "", errors.New("archive is too large")
	}

	output, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(output, hash), io.LimitReader(response.Body, limit+1))
	closeErr := output.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if written > limit {
		return "", errors.New("archive is too large")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (c *Client) request(ctx context.Context, url string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	request.Header.Set("User-Agent", "claude-configurator/"+c.currentVersion)
	return request, nil
}

func checksumFor(data []byte, name string) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == name {
			value := strings.ToLower(fields[0])
			if len(value) != sha256.Size*2 {
				break
			}
			if _, err := hex.DecodeString(value); err != nil {
				break
			}
			return value, nil
		}
	}
	return "", fmt.Errorf("checksum for %s was not found", name)
}

func extractBinary(archivePath, archiveName, outputPath string) error {
	if strings.HasSuffix(archiveName, ".zip") {
		return extractZip(archivePath, outputPath)
	}
	return extractTarGz(archivePath, outputPath)
}

func extractTarGz(archivePath, outputPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open release archive: %w", err)
	}
	defer compressed.Close()

	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read release archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != releaseBinaryName() {
			continue
		}
		if header.Size < 1 || header.Size > maxBinarySize {
			return errors.New("release binary has an invalid size")
		}
		return writeBinary(outputPath, io.LimitReader(reader, maxBinarySize+1))
	}
	return errors.New("release archive does not contain claude-config")
}

func extractZip(archivePath, outputPath string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open release archive: %w", err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if file.FileInfo().IsDir() || filepath.Base(file.Name) != releaseBinaryName() {
			continue
		}
		if file.UncompressedSize64 < 1 || file.UncompressedSize64 > maxBinarySize {
			return errors.New("release binary has an invalid size")
		}
		input, err := file.Open()
		if err != nil {
			return fmt.Errorf("open release binary: %w", err)
		}
		err = writeBinary(outputPath, io.LimitReader(input, maxBinarySize+1))
		input.Close()
		return err
	}
	return errors.New("release archive does not contain claude-config.exe")
}

func writeBinary(path string, input io.Reader) error {
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written < 1 || written > maxBinarySize {
		return errors.New("release binary has an invalid size")
	}
	return nil
}

func replaceExecutable(source, target string) error {
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	next, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".update-*")
	if err != nil {
		return err
	}
	nextPath := next.Name()
	defer os.Remove(nextPath)
	if _, err := io.Copy(next, input); err != nil {
		next.Close()
		return err
	}
	if err := next.Sync(); err != nil {
		next.Close()
		return err
	}
	if err := next.Close(); err != nil {
		return err
	}
	if err := os.Chmod(nextPath, info.Mode().Perm()); err != nil {
		return err
	}

	if err := os.Rename(nextPath, target); err == nil {
		return nil
	}

	previous := target + ".previous"
	if err := os.Remove(previous); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(target, previous); err != nil {
		return err
	}
	if err := os.Rename(nextPath, target); err != nil {
		_ = os.Rename(previous, target)
		return err
	}
	_ = os.Remove(previous)
	return nil
}

func copyExecutable(source, target string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		output.Close()
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	return os.Chmod(target, mode)
}

func releaseArchiveName(version string) string {
	extension := ".tar.gz"
	if runtime.GOOS == "windows" {
		extension = ".zip"
	}
	return fmt.Sprintf(
		"claude-configurator_%s_%s_%s%s",
		version,
		runtime.GOOS,
		runtime.GOARCH,
		extension,
	)
}

func releaseBinaryName() string {
	if runtime.GOOS == "windows" {
		return "claude-config.exe"
	}
	return "claude-config"
}

func parseVersion(value string) ([3]uint64, bool) {
	var version [3]uint64
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	if value == "" || strings.ContainsAny(value, "-+") {
		return version, false
	}
	parts := strings.Split(value, ".")
	if len(parts) != len(version) {
		return version, false
	}
	for index, part := range parts {
		number, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return [3]uint64{}, false
		}
		version[index] = number
	}
	return version, true
}

func compareVersion(left, right [3]uint64) int {
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}

func versionString(version [3]uint64) string {
	return fmt.Sprintf("%d.%d.%d", version[0], version[1], version[2])
}

func withEnvironment(environment []string, key, value string) []string {
	prefix := strings.ToUpper(key) + "="
	filtered := make([]string, 0, len(environment)+1)
	for _, item := range environment {
		if strings.HasPrefix(strings.ToUpper(item), prefix) {
			continue
		}
		filtered = append(filtered, item)
	}
	return append(filtered, key+"="+value)
}

func validHelperPath(path string) bool {
	if path == "" || !strings.HasPrefix(filepath.Base(path), ".updater-") {
		return false
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return false
	}
	root, err := filepath.Abs(filepath.Join(cache, "claude-configurator", "updates"))
	if err != nil {
		return false
	}
	candidate, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	if resolved, resolveErr := filepath.EvalSymlinks(candidate); resolveErr == nil {
		candidate = resolved
	}
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != "." && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
