package updater

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/jdevera/command-launcher/internal/command"
	"github.com/jdevera/command-launcher/internal/remote"
	"github.com/jdevera/command-launcher/internal/repository"
	"github.com/jdevera/command-launcher/internal/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type updaterTestManifest struct {
	name    string
	version string
}

func (m updaterTestManifest) Name() string                { return m.name }
func (m updaterTestManifest) Version() string             { return m.version }
func (m updaterTestManifest) Commands() []command.Command { return nil }

type updaterTestPackage struct {
	updaterTestManifest
}

func (p updaterTestPackage) RepositoryID() string { return "test" }
func (p updaterTestPackage) VerifyChecksum(string) (bool, error) {
	return true, nil
}
func (p updaterTestPackage) VerifySignature(string) (bool, error) {
	return true, nil
}
func (p updaterTestPackage) InstallTo(string) (command.PackageManifest, error) {
	return p.updaterTestManifest, nil
}
func (p updaterTestPackage) RunSetup(string) error { return nil }

type updaterTestRepository struct {
	name        string
	folder      string
	packages    map[string]command.PackageManifest
	paused      map[string]bool
	installed   []string
	updated     []string
	uninstalled []string
}

func newUpdaterTestRepository(t *testing.T, manifests ...updaterTestManifest) *updaterTestRepository {
	t.Helper()

	packages := make(map[string]command.PackageManifest, len(manifests))
	for _, manifest := range manifests {
		packages[manifest.name] = manifest
	}

	return &updaterTestRepository{
		name:     "test-repository",
		folder:   t.TempDir(),
		packages: packages,
		paused:   map[string]bool{},
	}
}

func (r *updaterTestRepository) Name() string { return r.name }

func (r *updaterTestRepository) Install(pkg command.Package) error {
	r.installed = append(r.installed, pkg.Name())
	r.packages[pkg.Name()] = pkg
	return nil
}

func (r *updaterTestRepository) Uninstall(name string) error {
	r.uninstalled = append(r.uninstalled, name)
	delete(r.packages, name)
	return nil
}

func (r *updaterTestRepository) Update(pkg command.Package) error {
	r.updated = append(r.updated, pkg.Name())
	r.packages[pkg.Name()] = pkg
	return nil
}

func (r *updaterTestRepository) InstalledPackages() []command.PackageManifest {
	names := make([]string, 0, len(r.packages))
	for name := range r.packages {
		names = append(names, name)
	}
	sort.Strings(names)

	manifests := make([]command.PackageManifest, 0, len(names))
	for _, name := range names {
		manifests = append(manifests, r.packages[name])
	}
	return manifests
}

func (r *updaterTestRepository) InstalledCommands() []command.Command           { return nil }
func (r *updaterTestRepository) InstalledGroupCommands() []command.Command      { return nil }
func (r *updaterTestRepository) InstalledExecutableCommands() []command.Command { return nil }
func (r *updaterTestRepository) InstalledSystemCommands() repository.SystemCommands {
	return repository.SystemCommands{}
}

func (r *updaterTestRepository) Package(name string) (command.PackageManifest, error) {
	manifest, ok := r.packages[name]
	if !ok {
		return nil, fmt.Errorf("package %s not installed", name)
	}
	return manifest, nil
}

func (r *updaterTestRepository) IsPackageUpdatePaused(name string) (bool, error) {
	return r.paused[name], nil
}

func (r *updaterTestRepository) PausePackageUpdate(name string) error {
	r.paused[name] = true
	return nil
}

func (r *updaterTestRepository) Command(string, string, string) (command.Command, error) {
	return nil, errors.New("command not found")
}

func (r *updaterTestRepository) RepositoryFolder() (string, error) {
	return r.folder, nil
}

type updaterTestVerification struct {
	checksum  bool
	signature bool
}

type updaterTestRemote struct {
	infos         map[string][]remote.PackageInfo
	packages      map[string]command.Package
	verifyResult  bool
	verifyErr     error
	verifications []updaterTestVerification
}

func newUpdaterTestRemote(infos ...remote.PackageInfo) *updaterTestRemote {
	result := &updaterTestRemote{
		infos:        map[string][]remote.PackageInfo{},
		packages:     map[string]command.Package{},
		verifyResult: true,
	}
	for _, info := range infos {
		result.infos[info.Name] = append(result.infos[info.Name], info)
		result.packages[info.Name+"@"+info.Version] = updaterTestPackage{
			updaterTestManifest: updaterTestManifest{name: info.Name, version: info.Version},
		}
	}
	return result
}

func (r *updaterTestRemote) Fetch() error { return nil }

func (r *updaterTestRemote) All() ([]remote.PackageInfo, error) {
	var result []remote.PackageInfo
	for _, infos := range r.infos {
		result = append(result, infos...)
	}
	return result, nil
}

func (r *updaterTestRemote) PackageNames() ([]string, error) {
	names := make([]string, 0, len(r.infos))
	for name := range r.infos {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (r *updaterTestRemote) Versions(name string) ([]string, error) {
	infos := r.infos[name]
	versions := make([]string, 0, len(infos))
	for _, info := range infos {
		versions = append(versions, info.Version)
	}
	return versions, nil
}

func (r *updaterTestRemote) LatestVersion(name string) (string, error) {
	info, err := r.LatestPackageInfo(name)
	if err != nil {
		return "", err
	}
	return info.Version, nil
}

func (r *updaterTestRemote) QueryLatestVersion(name string, filter remote.PackageInfoFilterFunc) (string, error) {
	info, err := r.QueryLatestPackageInfo(name, filter)
	if err != nil {
		return "", err
	}
	return info.Version, nil
}

func (r *updaterTestRemote) LatestPackageInfo(name string) (*remote.PackageInfo, error) {
	return r.QueryLatestPackageInfo(name, func(*remote.PackageInfo) bool { return true })
}

func (r *updaterTestRemote) QueryLatestPackageInfo(name string, filter remote.PackageInfoFilterFunc) (*remote.PackageInfo, error) {
	infos := r.infos[name]
	for i := len(infos) - 1; i >= 0; i-- {
		info := infos[i]
		if filter(&info) {
			return &info, nil
		}
	}
	return nil, fmt.Errorf("no matching package %s", name)
}

func (r *updaterTestRemote) Package(name, version string) (command.Package, error) {
	pkg, ok := r.packages[name+"@"+version]
	if !ok {
		return nil, fmt.Errorf("package %s@%s not found", name, version)
	}
	return pkg, nil
}

func (r *updaterTestRemote) PackageInfo(name, version string) (*remote.PackageInfo, error) {
	for _, info := range r.infos[name] {
		if info.Version == version {
			copy := info
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("package info %s@%s not found", name, version)
}

func (r *updaterTestRemote) Verify(_ command.Package, checksum, signature bool) (bool, error) {
	r.verifications = append(r.verifications, updaterTestVerification{
		checksum:  checksum,
		signature: signature,
	})
	return r.verifyResult, r.verifyErr
}

func updaterWithRemote(repo *updaterTestRepository, remoteRepo *updaterTestRemote) *CmdUpdater {
	updater := &CmdUpdater{
		CmdRepositoryBaseUrl: "test://remote",
		LocalRepo:            repo,
		User:                 user.User{Partition: 5},
		Timeout:              time.Second,
		SyncPolicy:           "always",
		remoteRepo:           remoteRepo,
	}
	updater.initRemoteRepoOnce.Do(func() {})
	return updater
}

func TestCheckUpdateCommandsBuildsInstallUpdateDeletePlan(t *testing.T) {
	repo := newUpdaterTestRepository(t,
		updaterTestManifest{name: "delete-me", version: "1.0.0"},
		updaterTestManifest{name: "update-me", version: "1.0.0"},
	)
	remoteRepo := newUpdaterTestRemote(
		remote.PackageInfo{Name: "install-me", Version: "1.0.0", StartPartition: 0, EndPartition: 9},
		remote.PackageInfo{Name: "update-me", Version: "2.0.0", StartPartition: 0, EndPartition: 9},
	)
	updater := updaterWithRemote(repo, remoteRepo)

	canUpdate := <-updater.checkUpdateCommands()

	assert.True(t, canUpdate)
	assert.Equal(t, map[string]string{"install-me": "1.0.0"}, updater.toBeInstalled)
	assert.Equal(t, map[string]string{"update-me": "2.0.0"}, updater.toBeUpdated)
	assert.Equal(t, map[string]string{"delete-me": "1.0.0"}, updater.toBeDeleted)
}

func TestCheckUpdateCommandsHonorsPartitionAndPause(t *testing.T) {
	repo := newUpdaterTestRepository(t,
		updaterTestManifest{name: "update-me", version: "1.0.0"},
	)
	repo.paused["update-me"] = true
	repo.paused["install-me"] = true
	remoteRepo := newUpdaterTestRemote(
		remote.PackageInfo{Name: "install-me", Version: "1.0.0", StartPartition: 0, EndPartition: 9},
		remote.PackageInfo{Name: "update-me", Version: "2.0.0", StartPartition: 0, EndPartition: 9},
		remote.PackageInfo{Name: "outside-partition", Version: "1.0.0", StartPartition: 6, EndPartition: 9},
	)
	updater := updaterWithRemote(repo, remoteRepo)

	canUpdate := <-updater.checkUpdateCommands()

	assert.False(t, canUpdate)
	assert.Empty(t, updater.toBeInstalled)
	assert.Empty(t, updater.toBeUpdated)
	assert.Empty(t, updater.toBeDeleted)

	updater = updaterWithRemote(repo, remoteRepo)
	updater.IgnoreUpdatePause = true

	canUpdate = <-updater.checkUpdateCommands()

	assert.True(t, canUpdate)
	assert.Equal(t, map[string]string{"install-me": "1.0.0"}, updater.toBeInstalled)
	assert.Equal(t, map[string]string{"update-me": "2.0.0"}, updater.toBeUpdated)
	assert.NotContains(t, updater.toBeInstalled, "outside-partition")
}

func TestCheckUpdateCommandsUsesCILockVersions(t *testing.T) {
	repo := newUpdaterTestRepository(t,
		updaterTestManifest{name: "update-me", version: "1.0.0"},
	)
	remoteRepo := newUpdaterTestRemote(
		remote.PackageInfo{Name: "locked-new", Version: "3.1.4", StartPartition: 0, EndPartition: 9},
		remote.PackageInfo{Name: "locked-new", Version: "4.0.0", StartPartition: 0, EndPartition: 9},
		remote.PackageInfo{Name: "update-me", Version: "1.5.0", StartPartition: 0, EndPartition: 9},
		remote.PackageInfo{Name: "update-me", Version: "2.0.0", StartPartition: 0, EndPartition: 9},
	)
	lockPath := filepath.Join(t.TempDir(), "lock.json")
	require.NoError(t, os.WriteFile(lockPath, []byte(`{"locked-new":"3.1.4","update-me":"1.5.0"}`), 0600))

	updater := updaterWithRemote(repo, remoteRepo)
	updater.EnableCI = true
	updater.PackageLockFile = lockPath

	canUpdate := <-updater.checkUpdateCommands()

	assert.True(t, canUpdate)
	assert.Equal(t, map[string]string{"locked-new": "3.1.4"}, updater.toBeInstalled)
	assert.Equal(t, map[string]string{"update-me": "1.5.0"}, updater.toBeUpdated)
}

func TestUpdateAppliesPlanAndForwardsVerificationPolicy(t *testing.T) {
	repo := newUpdaterTestRepository(t,
		updaterTestManifest{name: "delete-me", version: "1.0.0"},
		updaterTestManifest{name: "update-me", version: "1.0.0"},
	)
	remoteRepo := newUpdaterTestRemote(
		remote.PackageInfo{Name: "install-me", Version: "1.0.0", StartPartition: 0, EndPartition: 9},
		remote.PackageInfo{Name: "update-me", Version: "2.0.0", StartPartition: 0, EndPartition: 9},
	)
	updater := updaterWithRemote(repo, remoteRepo)
	updater.VerifyChecksum = true
	updater.VerifySignature = true
	updater.toBeDeleted = map[string]string{"delete-me": "1.0.0"}
	updater.toBeUpdated = map[string]string{"update-me": "2.0.0"}
	updater.toBeInstalled = map[string]string{"install-me": "1.0.0"}
	updateReady := make(chan bool, 1)
	updateReady <- true
	updater.cmdUpdateChan = updateReady

	require.NoError(t, updater.Update())

	assert.ElementsMatch(t, []string{"delete-me"}, repo.uninstalled)
	assert.ElementsMatch(t, []string{"update-me"}, repo.updated)
	assert.ElementsMatch(t, []string{"install-me"}, repo.installed)
	require.Len(t, remoteRepo.verifications, 2)
	for _, verification := range remoteRepo.verifications {
		assert.True(t, verification.checksum)
		assert.True(t, verification.signature)
	}
}

func TestUpdateDoesNotInstallArtifactRejectedByVerification(t *testing.T) {
	repo := newUpdaterTestRepository(t)
	remoteRepo := newUpdaterTestRemote(
		remote.PackageInfo{Name: "install-me", Version: "1.0.0", StartPartition: 0, EndPartition: 9},
	)
	remoteRepo.verifyResult = false
	remoteRepo.verifyErr = errors.New("verification rejected")
	updater := updaterWithRemote(repo, remoteRepo)
	updater.toBeInstalled = map[string]string{"install-me": "1.0.0"}
	updateReady := make(chan bool, 1)
	updateReady <- true
	updater.cmdUpdateChan = updateReady

	err := updater.Update()

	require.ErrorContains(t, err, "verification rejected")
	assert.Empty(t, repo.installed)
	assert.True(t, repo.paused["install-me"])
}

func TestReachSyncSchedule(t *testing.T) {
	repo := newUpdaterTestRepository(t)
	updater := updaterWithRemote(repo, newUpdaterTestRemote())

	updater.SyncPolicy = "never"
	require.ErrorContains(t, updater.reachSyncSchedule(), "set to never")

	updater.SyncPolicy = "always"
	require.NoError(t, updater.reachSyncSchedule())

	updater.SyncPolicy = "daily"
	require.NoError(t, updater.reachSyncSchedule(), "a missing timestamp allows synchronization")

	timestampPath := filepath.Join(repo.folder, "sync.timestamp")
	require.NoError(t, os.WriteFile(timestampPath, []byte(time.Now().Add(time.Hour).Format(time.RFC3339)), 0600))
	require.ErrorContains(t, updater.reachSyncSchedule(), "Not yet reach the sync time")

	require.NoError(t, os.WriteFile(timestampPath, []byte(time.Now().Add(-time.Hour).Format(time.RFC3339)), 0600))
	require.NoError(t, updater.reachSyncSchedule())
}

func TestUpdateSyncTimestampUsesPolicyDelay(t *testing.T) {
	repo := newUpdaterTestRepository(t)
	updater := updaterWithRemote(repo, newUpdaterTestRemote())
	updater.SyncPolicy = "weekly"
	before := time.Now().Add(7*24*time.Hour - time.Minute)

	require.NoError(t, updater.UpdateSyncTimestamp())

	data, err := os.ReadFile(filepath.Join(repo.folder, "sync.timestamp"))
	require.NoError(t, err)
	written, err := time.Parse(time.RFC3339, string(data))
	require.NoError(t, err)
	after := time.Now().Add(7*24*time.Hour + time.Minute)
	assert.True(t, written.After(before), "timestamp %s should be after %s", written, before)
	assert.True(t, written.Before(after), "timestamp %s should be before %s", written, after)
}
