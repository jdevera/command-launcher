package helper

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFileFromURLRejectsUntrustedCertificate(t *testing.T) {
	server := untrustedTLSServer(t)

	contents, err := LoadFileFromUrl(server.URL)

	require.ErrorContains(t, err, "certificate")
	require.Empty(t, contents)
}

func TestDownloadFileFromURLRejectsUntrustedCertificate(t *testing.T) {
	server := untrustedTLSServer(t)
	destination := filepath.Join(t.TempDir(), "download")

	err := DownloadFileFromUrl(server.URL, destination, false)

	require.ErrorContains(t, err, "certificate")
}

func untrustedTLSServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("untrusted content"))
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	t.Cleanup(server.Close)
	return server
}
