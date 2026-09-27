//go:build e2e

package e2e_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBasicHelpAndCommandDiscovery(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")

	result := runLauncher(t, home, nil)
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "Command Launcher - A command launcher", commandFailure(result))
	require.NotContains(t, result.Stdout, "  hello", commandFailure(result))
	require.DirExists(t, filepath.Join(home, "current"))

	installDropinFixture(t, home, "bonjour")
	result = runLauncher(t, home, nil)
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "Commands from 'dropin' registry", commandFailure(result))
	require.Contains(t, result.Stdout, "  bonjour", commandFailure(result))

	result = runLauncher(t, home, nil, "config", "group_help_by_registry", "false")
	requireSuccess(t, result)
	result = runLauncher(t, home, nil)
	requireSuccess(t, result)
	require.NotContains(t, result.Stdout, "Commands from 'dropin' registry", commandFailure(result))
	require.Contains(t, result.Stdout, "  bonjour", commandFailure(result))
}

func TestConfigurationOutput(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")

	result := runLauncher(t, home, nil, "config")
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "local_command_repository_dirname", commandFailure(result))
	require.Contains(t, result.Stdout, filepath.Join(home, "current"), commandFailure(result))

	result = runLauncher(t, home, nil, "config", "--json")
	requireSuccess(t, result)
	settings := decodeJSONObject(t, result.Stdout)
	require.Equal(t, false, settings["log_enabled"])
	require.Equal(t, true, settings["group_help_by_registry"])
	require.Equal(t, filepath.Join(home, "current"), settings["local_command_repository_dirname"])

	result = runLauncher(t, home, nil, "config", "log_enabled", "--json")
	requireSuccess(t, result)
	setting := decodeJSONObject(t, result.Stdout)
	require.Equal(t, map[string]any{"log_enabled": false}, setting)
}

func TestExternalCommandExitCodeIsPreserved(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	installDropinFixture(t, home, "exit-code")

	result := runLauncher(t, home, nil, "exit0")
	requireSuccess(t, result)

	result = runLauncher(t, home, nil, "exit1")
	require.Error(t, result.Err, commandFailure(result))
	require.Equal(t, 1, result.ExitCode, commandFailure(result))
}

func TestRuntimeNameAndConfigurationAreDerivedFromExecutable(t *testing.T) {
	originalHome := filepath.Join(t.TempDir(), "cl-home")
	result := runLauncher(t, originalHome, nil, "version")
	requireSuccess(t, result)
	require.Regexp(t, `^cl version`, result.Stdout, commandFailure(result))

	copyDirectory := t.TempDir()
	copiedPath := filepath.Join(copyDirectory, "myapp"+executableExtension())
	copyFile(t, launcherPath, copiedPath)
	copiedHome := filepath.Join(t.TempDir(), "myapp-home")

	result = runExecutable(t, copiedPath, "MYAPP", copiedHome, nil, "version")
	requireSuccess(t, result)
	require.Regexp(t, `^myapp version`, result.Stdout, commandFailure(result))

	result = runExecutable(t, copiedPath, "MYAPP", copiedHome, nil)
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "Command Launcher - A command launcher", commandFailure(result))

	result = runExecutable(t, copiedPath, "MYAPP", copiedHome, nil, "config", "app_long_name", "My Custom App")
	requireSuccess(t, result)
	result = runExecutable(t, copiedPath, "MYAPP", copiedHome, nil)
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "My Custom App - A command launcher", commandFailure(result))

	result = runLauncher(t, originalHome, nil)
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "Command Launcher - A command launcher", commandFailure(result))
}

func TestRuntimeNameResolvesExecutableSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating a real executable symlink is not reliable on Windows runners")
	}

	linkPath := filepath.Join(t.TempDir(), "myalias")
	require.NoError(t, os.Symlink(launcherPath, linkPath))
	home := filepath.Join(t.TempDir(), "cl-home")

	result := runExecutable(t, linkPath, "CL", home, nil, "version")
	requireSuccess(t, result)
	require.Regexp(t, `^cl version`, result.Stdout, commandFailure(result))
}

func TestYAMLManifestCommandsAndHelp(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	installDropinFixture(t, home, "yaml-manifest")

	result := runLauncher(t, home, nil, "bonjour1", "world")
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "bonjour! world", commandFailure(result))

	result = runLauncher(t, home, nil, "bonjour2")
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "bonjour! monde", commandFailure(result))

	result = runLauncher(t, home, nil, "help", "bonjour1")
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "This is another line", commandFailure(result))

	result = runLauncher(t, home, nil, "bonjour2", "-h")
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "bonjour2 name", commandFailure(result))
	require.Contains(t, result.Stdout, "# Print greeting message", commandFailure(result))
}

func installDropinFixture(t *testing.T, home, fixtureName string) {
	t.Helper()

	source := filepath.Join(repositoryRoot, "test", "packages-src", fixtureName)
	target := filepath.Join(home, "dropins", fixtureName)
	require.NoError(t, filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(target, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, contents, info.Mode().Perm())
	}))
}

func copyFile(t *testing.T, source, target string) {
	t.Helper()

	contents, err := os.ReadFile(source)
	require.NoError(t, err)
	info, err := os.Stat(source)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(target, contents, info.Mode().Perm()))
}

func decodeJSONObject(t *testing.T, output string) map[string]any {
	t.Helper()

	var value map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &value), output)
	return value
}
