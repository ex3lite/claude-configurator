package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCheckFindsNewPlatformRelease(t *testing.T) {
	archiveName := releaseArchiveName("0.4.0")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") != "claude-configurator/0.3.0" {
			t.Errorf("User-Agent = %q", request.Header.Get("User-Agent"))
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"tag_name": "v0.4.0",
			"html_url": "https://example.test/releases/v0.4.0",
			"assets": []map[string]string{
				{"name": archiveName, "browser_download_url": serverURL(request) + "/archive"},
				{"name": "checksums.txt", "browser_download_url": serverURL(request) + "/checksums"},
			},
		})
	}))
	defer server.Close()

	client := New("0.3.0")
	client.apiURL = server.URL
	release, available, err := client.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !available || release.Version != "0.4.0" {
		t.Fatalf("available/version = %v/%q", available, release.Version)
	}
	if release.Archive.Name != archiveName || release.Checksums.Name != "checksums.txt" {
		t.Fatalf("release assets = %#v", release)
	}
}

func TestPrepareVerifiesAndExtractsRelease(t *testing.T) {
	binaryContents := []byte("new claude-config binary")
	archive := testArchive(t, binaryContents)
	sum := sha256.Sum256(archive)
	archiveName := releaseArchiveName("0.4.0")

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/archive":
			_, _ = writer.Write(archive)
		case "/checksums":
			fmt.Fprintf(writer, "%s  %s\n", hex.EncodeToString(sum[:]), archiveName)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), releaseBinaryName())
	if err := os.WriteFile(target, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	client := New("0.3.0")
	client.executable = func() (string, error) { return target, nil }
	client.cacheDir = func() (string, error) { return cache, nil }

	prepared, err := client.Prepare(context.Background(), Release{
		Version: "0.4.0",
		Archive: Asset{
			Name:   archiveName,
			URL:    server.URL + "/archive",
			Digest: "sha256:" + hex.EncodeToString(sum[:]),
		},
		Checksums: Asset{Name: "checksums.txt", URL: server.URL + "/checksums"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Remove(prepared.Binary)
		os.Remove(prepared.Helper)
	})
	got, err := os.ReadFile(prepared.Binary)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, binaryContents) {
		t.Fatalf("staged binary = %q", got)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Target != resolvedTarget {
		t.Fatalf("target = %q, want %q", prepared.Target, resolvedTarget)
	}
	helper, err := os.ReadFile(prepared.Helper)
	if err != nil {
		t.Fatal(err)
	}
	if string(helper) != "old binary" {
		t.Fatalf("update helper = %q", helper)
	}
	if current, _ := os.ReadFile(target); string(current) != "old binary" {
		t.Fatalf("target changed before consented install: %q", current)
	}
}

func TestReplaceExecutable(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	target := filepath.Join(directory, "target")
	if err := os.WriteFile(source, []byte("new"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceExecutable(source, target); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("target = %q", got)
	}
}

func TestPrepareRejectsChecksumMismatch(t *testing.T) {
	archiveName := releaseArchiveName("0.4.0")
	archive := testArchive(t, []byte("untrusted binary"))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/checksums" {
			fmt.Fprintf(writer, "%064d  %s\n", 0, archiveName)
			return
		}
		_, _ = writer.Write(archive)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), releaseBinaryName())
	if err := os.WriteFile(target, []byte("current binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	client := New("0.3.0")
	client.executable = func() (string, error) { return target, nil }
	client.cacheDir = func() (string, error) { return t.TempDir(), nil }

	_, err := client.Prepare(context.Background(), Release{
		Version:   "0.4.0",
		Archive:   Asset{Name: archiveName, URL: server.URL + "/archive"},
		Checksums: Asset{Name: "checksums.txt", URL: server.URL + "/checksums"},
	})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Prepare error = %v", err)
	}
	if current, _ := os.ReadFile(target); string(current) != "current binary" {
		t.Fatalf("target changed after checksum failure: %q", current)
	}
}

func TestVersionComparison(t *testing.T) {
	for _, test := range []struct {
		current string
		latest  string
		newer   bool
	}{
		{"0.3.0", "v0.4.0", true},
		{"1.2.3", "1.2.3", false},
		{"2.0.0", "1.99.99", false},
		{"dev", "1.0.0", false},
	} {
		current, currentOK := parseVersion(test.current)
		latest, latestOK := parseVersion(test.latest)
		newer := currentOK && latestOK && compareVersion(latest, current) > 0
		if newer != test.newer {
			t.Errorf("%s -> %s newer = %v", test.current, test.latest, newer)
		}
	}
}

func testArchive(t *testing.T, binary []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if runtime.GOOS == "windows" {
		writer := zip.NewWriter(&buffer)
		file, err := writer.Create(releaseBinaryName())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(binary); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}

	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	if err := writer.WriteHeader(&tar.Header{
		Name: releaseBinaryName(),
		Mode: 0o755,
		Size: int64(len(binary)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func serverURL(request *http.Request) string {
	return "http://" + request.Host
}
