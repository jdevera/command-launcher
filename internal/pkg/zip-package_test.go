package pkg

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/jdevera/command-launcher/internal/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPackageManifest = `pkgName: zip-test
version: 1.0.0
cmds: []
`

type testZipEntry struct {
	name     string
	contents string
	mode     fs.FileMode
}

func TestCreatePackage(t *testing.T) {
	pkg, err := CreateZipPackage("assets/fake-1.0.0.pkg")
	assert.Nil(t, err, "cannot create package")

	assert.Equal(t, "fake", pkg.Name())
	assert.Equal(t, "1.0.0", pkg.Version())
	assert.Equal(t, 2, len(pkg.Commands()))
}

func TestInstallPackage(t *testing.T) {
	pkg, err := CreateZipPackage("assets/fake-1.0.0.pkg")
	assert.Nil(t, err)

	target, err := os.MkdirTemp("", "cdt-package-test-*")
	assert.Nil(t, err)

	mf, err := pkg.InstallTo(target)
	assert.Nil(t, err)

	assert.Equal(t, "fake", mf.Name())
	assert.Equal(t, "1.0.0", mf.Version())
	assert.Equal(t, 2, len(mf.Commands()))
}

func TestInstallPackageCreatesMissingParentDirectories(t *testing.T) {
	archive := createTestZipPackage(t, testZipEntry{
		name:     "bin/tool.sh",
		contents: "#!/bin/sh\necho hello\n",
		mode:     0755,
	})
	pkg, err := CreateZipPackage(archive)
	require.NoError(t, err)

	target := filepath.Join(t.TempDir(), "package")
	_, err = pkg.InstallTo(target)
	require.NoError(t, err)

	contents, err := os.ReadFile(filepath.Join(target, "bin", "tool.sh"))
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\necho hello\n", string(contents))
}

func TestInstallPackageDoesNotDependOnDirectoryEntryOrder(t *testing.T) {
	archive := createTestZipPackage(t,
		testZipEntry{
			name:     "bin/tool.sh",
			contents: "#!/bin/sh\necho hello\n",
			mode:     0755,
		},
		testZipEntry{
			name: "bin/",
			mode: fs.ModeDir | 0755,
		},
	)
	pkg, err := CreateZipPackage(archive)
	require.NoError(t, err)

	target := filepath.Join(t.TempDir(), "package")
	_, err = pkg.InstallTo(target)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(target, "bin", "tool.sh"))
}

func TestInstallPackageRejectsPathTraversal(t *testing.T) {
	archive := createTestZipPackage(t, testZipEntry{
		name:     "../escaped.txt",
		contents: "outside the package",
		mode:     0644,
	})

	targetRoot := t.TempDir()
	target := filepath.Join(targetRoot, "package")
	pkg, err := CreateZipPackage(archive)
	if err == nil {
		_, err = pkg.InstallTo(target)
	}

	require.Error(t, err)
	assert.NoFileExists(t, filepath.Join(targetRoot, "escaped.txt"))
}

func TestZipEntryPathRejectsUnsafeNames(t *testing.T) {
	tests := []string{
		"",
		"../escaped.txt",
		"bin/../../escaped.txt",
		"/absolute/path",
		`..\escaped.txt`,
		`C:\absolute\path`,
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := zipEntryPath(t.TempDir(), name)
			assert.Error(t, err)
		})
	}
}

func TestInstallPackageWithSetupError(t *testing.T) {
	pkg, err := CreateZipPackage("assets/fake-wrong-setup-1.0.0.pkg")
	assert.Nil(t, err)

	target, err := os.MkdirTemp("", "cdt-package-test-*")
	assert.Nil(t, err)

	var previousSetupHook = viper.GetBool(config.ENABLE_PACKAGE_SETUP_HOOK_KEY)
	defer viper.Set(config.ENABLE_PACKAGE_SETUP_HOOK_KEY, previousSetupHook)
	viper.Set(config.ENABLE_PACKAGE_SETUP_HOOK_KEY, true)

	mf, err := pkg.InstallTo(target)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("setup hook of package %s failed to execute", pkg.Name()))
	assert.Nil(t, mf)
}

func TestVerifyChecksum(t *testing.T) {
	pkg, err := CreateZipPackage("assets/fake-1.0.0.pkg")
	assert.Nil(t, err)
	verified, err := pkg.VerifyChecksum("353b23600bd2c3a661c6b825b2a27f19ee14938903bac24290ec26a5c9fa5bb4")
	assert.Nil(t, err)
	assert.True(t, verified)
}

func createTestZipPackage(t *testing.T, entries ...testZipEntry) string {
	t.Helper()

	archivePath := filepath.Join(t.TempDir(), "test.pkg")
	archiveFile, err := os.Create(archivePath)
	require.NoError(t, err)

	zipWriter := zip.NewWriter(archiveFile)
	allEntries := append([]testZipEntry{{
		name:     "manifest.mf",
		contents: testPackageManifest,
		mode:     0644,
	}}, entries...)

	for _, entry := range allEntries {
		header := &zip.FileHeader{
			Name:   entry.name,
			Method: zip.Deflate,
		}
		header.SetMode(entry.mode)

		entryWriter, err := zipWriter.CreateHeader(header)
		require.NoError(t, err)
		_, err = io.WriteString(entryWriter, entry.contents)
		require.NoError(t, err)
	}

	require.NoError(t, zipWriter.Close())
	require.NoError(t, archiveFile.Close())
	return archivePath
}
