//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const commandTimeout = 15 * time.Second

var (
	launcherPath   string
	repositoryRoot string
)

type commandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Err      error
}

func TestMain(m *testing.M) {
	var err error
	repositoryRoot, err = findRepositoryRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	buildDir, err := os.MkdirTemp("", "command-launcher-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create e2e build directory: %v\n", err)
		os.Exit(1)
	}

	launcherPath = filepath.Join(buildDir, "cl"+executableExtension())
	build := exec.Command("go", "build", "-o", launcherPath, ".")
	build.Dir = repositoryRoot
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build command launcher: %v\n%s", err, output)
		_ = os.RemoveAll(buildDir)
		os.Exit(1)
	}

	exitCode := m.Run()
	if err := os.RemoveAll(buildDir); err != nil {
		fmt.Fprintf(os.Stderr, "remove e2e build directory: %v\n", err)
		if exitCode == 0 {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func findRepositoryRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("locate e2e test source")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", fmt.Errorf("locate repository root: %w", err)
	}
	return root, nil
}

func executableExtension() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func runLauncher(t *testing.T, home string, environment map[string]string, args ...string) commandResult {
	t.Helper()
	return runExecutable(t, launcherPath, "CL", home, environment, args...)
}

func runExecutable(t *testing.T, executable, environmentPrefix, home string, environment map[string]string, args ...string) commandResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = filepath.Dir(home)
	command.Env = launcherEnvironment(environmentPrefix, home, environment)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	result := commandResult{
		ExitCode: 0,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Err:      err,
	}

	if ctx.Err() != nil {
		result.ExitCode = -1
		result.Err = fmt.Errorf("command timed out after %s: %w", commandTimeout, ctx.Err())
		return result
	}

	if err == nil {
		return result
	}

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		result.ExitCode = exitError.ExitCode()
	} else {
		result.ExitCode = -1
	}
	return result
}

func launcherEnvironment(environmentPrefix, home string, overrides map[string]string) []string {
	prefix := strings.ToUpper(environmentPrefix) + "_"
	replaced := map[string]string{
		prefix + "DEBUG_FLAGS":  "use_file_vault",
		prefix + "HOME":         home,
		prefix + "VAULT_SECRET": "e2e-test-secret",
		"NO_COLOR":              "1",
	}
	for key, value := range overrides {
		replaced[key] = value
	}

	environment := make([]string, 0, len(os.Environ())+len(replaced))
	for _, entry := range os.Environ() {
		key, _, found := strings.Cut(entry, "=")
		upperKey := strings.ToUpper(key)
		if !found || strings.HasPrefix(upperKey, "CL_") || strings.HasPrefix(upperKey, prefix) {
			continue
		}
		if _, overridden := replaced[key]; overridden {
			continue
		}
		environment = append(environment, entry)
	}
	for key, value := range replaced {
		environment = append(environment, key+"="+value)
	}
	return environment
}
