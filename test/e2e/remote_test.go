//go:build e2e

package e2e_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type packageIndexEntry struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	URL            string `json:"url,omitempty"`
	Checksum       string `json:"checksum"`
	StartPartition int    `json:"startPartition"`
	EndPartition   int    `json:"endPartition"`
}

type packageFixture struct {
	contents []byte
	checksum string
}

func TestRemotePackagesInstallAndUpdateFromConfiguredHTTPRegistry(t *testing.T) {
	server := newRemoteFixtureServer(t)
	home := filepath.Join(t.TempDir(), "home")

	result := runLauncher(t, home, nil,
		"config", "command_repository_base_url", server.URL+"/initial",
	)
	requireSuccess(t, result)

	result = runLauncher(t, home, nil)
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "hello", commandFailure(result))
	require.FileExists(t, filepath.Join(home, "current", "command-launcher-demo", "manifest.mf"))

	result = runLauncher(t, home, nil, "hello")
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "Hello World!", commandFailure(result))

	remoteEnvironment := map[string]string{
		"CL_REMOTE_CONFIG_URL": server.URL + "/config/remote_config.json",
	}
	result = runLauncher(t, home, remoteEnvironment, "config")
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, server.URL+"/updated", commandFailure(result))

	result = runLauncher(t, home, remoteEnvironment, "update", "--package")
	requireSuccess(t, result)
	require.Contains(t, result.Stdout,
		"upgrade package 'command-launcher-demo' from version 1.0.0 to version 2.0.0",
		commandFailure(result),
	)

	result = runLauncher(t, home, remoteEnvironment)
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "bonjour", commandFailure(result))
	require.FileExists(t, filepath.Join(home, "current", "bonjour", "manifest.mf"))

	result = runLauncher(t, home, remoteEnvironment, "bonjour")
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "bonjour!", commandFailure(result))

	result = runLauncher(t, home, remoteEnvironment, "hello")
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "Hello World v2!", commandFailure(result))
}

func newRemoteFixtureServer(t *testing.T) *httptest.Server {
	t.Helper()

	initialPackage := readPackageFixture(t,
		filepath.Join(repositoryRoot, "examples", "remote-repo", "command-launcher-demo-1.0.0.pkg"),
	)
	updatedPackage := readPackageFixture(t,
		filepath.Join(repositoryRoot, "test", "remote-repo", "command-launcher-demo-2.0.0.pkg"),
	)
	bonjourPackage := readPackageFixture(t,
		filepath.Join(repositoryRoot, "test", "packages-src", "bonjour", "bonjour-v1.pkg"),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /initial/index.json", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, []packageIndexEntry{
			{
				Name:           "command-launcher-demo",
				Version:        "1.0.0",
				Checksum:       initialPackage.checksum,
				StartPartition: 0,
				EndPartition:   9,
			},
		})
	})
	mux.HandleFunc("GET /initial/command-launcher-demo-1.0.0.pkg", servePackage(initialPackage))

	mux.HandleFunc("GET /updated/index.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []packageIndexEntry{
			{
				Name:           "command-launcher-demo",
				Version:        "2.0.0",
				Checksum:       updatedPackage.checksum,
				StartPartition: 0,
				EndPartition:   9,
			},
			{
				Name:           "bonjour",
				Version:        "1.0.0",
				URL:            "http://" + r.Host + "/packages/bonjour-v1.pkg",
				Checksum:       bonjourPackage.checksum,
				StartPartition: 0,
				EndPartition:   9,
			},
		})
	})
	mux.HandleFunc("GET /updated/command-launcher-demo-2.0.0.pkg", servePackage(updatedPackage))
	mux.HandleFunc("GET /packages/bonjour-v1.pkg", servePackage(bonjourPackage))

	mux.HandleFunc("GET /config/remote_config.json", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"ci_enabled":                  true,
			"command_repository_base_url": "http://" + r.Host + "/updated",
		})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func readPackageFixture(t *testing.T, path string) packageFixture {
	t.Helper()

	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	checksum := sha256.Sum256(contents)
	return packageFixture{
		contents: contents,
		checksum: hex.EncodeToString(checksum[:]),
	}
}

func servePackage(fixture packageFixture) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(fixture.contents)
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func requireSuccess(t *testing.T, result commandResult) {
	t.Helper()

	require.NoError(t, result.Err, commandFailure(result))
	require.Equal(t, 0, result.ExitCode, commandFailure(result))
}

func commandFailure(result commandResult) string {
	return fmt.Sprintf("exit code: %d\nstdout:\n%s\nstderr:\n%s", result.ExitCode, result.Stdout, result.Stderr)
}
