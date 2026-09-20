package remoteconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var testConfig = Config{
	DefaultRepository: "fallback/project",
	Tag:               "remote-fixtures-v1",
	RegistryPath:      "test/fixtures/remote",
}

func TestBaseURLUsesExplicitOverride(t *testing.T) {
	clearRemoteEnvironment(t)
	t.Setenv(baseURLOverrideEnvironment, "https://fixtures.example.test/registry/")

	baseURL, err := BaseURL(t.TempDir(), testConfig)

	require.NoError(t, err)
	require.Equal(t, "https://fixtures.example.test/registry", baseURL)
}

func TestBaseURLUsesGitHubActionsRepository(t *testing.T) {
	clearRemoteEnvironment(t)
	t.Setenv(githubServerURLEnvironment, "https://github.example.test")
	t.Setenv(githubRepositoryEnvironment, "fork/project")

	baseURL, err := BaseURL(t.TempDir(), testConfig)

	require.NoError(t, err)
	require.Equal(t, "https://github.example.test/fork/project/raw/remote-fixtures-v1/test/fixtures/remote", baseURL)
}

func TestBaseURLUsesGitOrigin(t *testing.T) {
	clearRemoteEnvironment(t)
	repositoryRoot := initializeGitRepository(t, "git@github.com:fork/project.git")

	baseURL, err := BaseURL(repositoryRoot, testConfig)

	require.NoError(t, err)
	require.Equal(t, "https://github.com/fork/project/raw/remote-fixtures-v1/test/fixtures/remote", baseURL)
}

func TestBaseURLFallsBackWithoutGitMetadata(t *testing.T) {
	clearRemoteEnvironment(t)

	baseURL, err := BaseURL(t.TempDir(), testConfig)

	require.NoError(t, err)
	require.Equal(t, "https://github.com/fallback/project/raw/remote-fixtures-v1/test/fixtures/remote", baseURL)
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "remote-fixture.json")
	require.NoError(t, os.WriteFile(filename, []byte(`{
  "default_repository": "missing-owner",
  "tag": "",
  "registry_path": "../outside"
}`), 0o600))

	_, err := Load(filename)

	require.Error(t, err)
}

func TestParseGitOrigin(t *testing.T) {
	tests := []struct {
		name       string
		origin     string
		serverURL  string
		repository string
		valid      bool
	}{
		{name: "SSH", origin: "git@github.com:owner/project.git", serverURL: "https://github.com", repository: "owner/project", valid: true},
		{name: "HTTPS", origin: "https://github.com/owner/project.git", serverURL: "https://github.com", repository: "owner/project", valid: true},
		{name: "SSH URL", origin: "ssh://git@github.example.test/owner/project.git", serverURL: "https://github.example.test", repository: "owner/project", valid: true},
		{name: "SSH URL with port", origin: "ssh://git@github.example.test:8443/owner/project.git", serverURL: "https://github.example.test:8443", repository: "owner/project", valid: true},
		{name: "local path", origin: "/tmp/project", valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			serverURL, repository, valid := parseGitOrigin(test.origin)
			require.Equal(t, test.valid, valid)
			require.Equal(t, test.serverURL, serverURL)
			require.Equal(t, test.repository, repository)
		})
	}
}

func clearRemoteEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv(baseURLOverrideEnvironment, "")
	t.Setenv(githubRepositoryEnvironment, "")
	t.Setenv(githubServerURLEnvironment, "")
}

func initializeGitRepository(t *testing.T, origin string) string {
	t.Helper()

	repositoryRoot := t.TempDir()
	runGit(t, repositoryRoot, "init")
	runGit(t, repositoryRoot, "remote", "add", "origin", origin)
	return repositoryRoot
}

func runGit(t *testing.T, directory string, arguments ...string) {
	t.Helper()

	command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
}
