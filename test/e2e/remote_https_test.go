//go:build e2e && remote

package e2e_test

import (
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jdevera/command-launcher/test/remoteconfig"
	"github.com/stretchr/testify/require"
)

func TestRemoteHTTPSRegistry(t *testing.T) {
	config, err := remoteconfig.Load(filepath.Join(repositoryRoot, "test", "remote-fixture.json"))
	require.NoError(t, err)

	baseURL, err := remoteconfig.BaseURL(repositoryRoot, config)
	require.NoError(t, err)
	parsedBaseURL, err := url.Parse(baseURL)
	require.NoError(t, err)
	require.Equal(t, "https", parsedBaseURL.Scheme)
	hostname := parsedBaseURL.Hostname()
	require.NotEmpty(t, hostname)
	require.NotEqual(t, "localhost", strings.ToLower(hostname))
	if address := net.ParseIP(hostname); address != nil {
		require.False(t, address.IsLoopback(), "remote HTTPS smoke test cannot use a loopback address")
	}

	home := filepath.Join(t.TempDir(), "home")
	result := runLauncher(t, home, nil, "config", "command_repository_base_url", baseURL)
	requireSuccess(t, result)

	result = runLauncher(t, home, nil)
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "hello", commandFailure(result))
	require.FileExists(t, filepath.Join(home, "current", "command-launcher-demo", "manifest.mf"))

	result = runLauncher(t, home, nil, "hello")
	requireSuccess(t, result)
	require.Contains(t, result.Stdout, "Hello World!", commandFailure(result))
}
