package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/criteo/command-launcher/cmd/user"
	"github.com/criteo/command-launcher/internal/console"
	"github.com/criteo/command-launcher/internal/helper"
	"github.com/inconshreveable/go-update"
	log "github.com/sirupsen/logrus"
)

type LatestVersion struct {
	Version        string `json:"version"`
	ReleaseNotes   string `json:"releaseNotes"`
	StartPartition uint8  `json:"startPartition"`
	EndPartition   uint8  `json:"endPartition"`
}

type SelfUpdater struct {
	selfUpdateChan <-chan bool
	latestVersion  LatestVersion

	BinaryName        string
	LatestVersionUrl  string
	SelfUpdateRootUrl string
	User              user.User
	CurrentVersion    string
	Timeout           time.Duration
}

func (u *SelfUpdater) CheckUpdateAsync() {
	ch := make(chan bool, 1)
	u.selfUpdateChan = ch
	go func() {
		select {
		case value := <-u.checkSelfUpdate():
			ch <- value
		case <-time.After(u.Timeout):
			ch <- false
		}
	}()
}

func (u *SelfUpdater) Update() error {
	canBeSelfUpdated := <-u.selfUpdateChan || helper.LoadDebugFlags().ForceSelfUpdate
	if !canBeSelfUpdated {
		return nil
	}

	fmt.Println("\n-----------------------------------")
	fmt.Printf("🚀 %s version %s \n", u.BinaryName, u.CurrentVersion)
	fmt.Printf("\nan update of %s (%s) is available:\n\n", u.BinaryName, u.latestVersion.Version)
	fmt.Println(u.latestVersion.ReleaseNotes)
	fmt.Println()
	console.Reminder("do you want to update it? [yN]")
	var resp int
	if _, err := fmt.Scanf("%c", &resp); err != nil || (resp != 'y' && resp != 'Y') {
		fmt.Println("aborted by user")
		return fmt.Errorf("Aborted by user")
	}

	fmt.Printf("update and install the latest version of %s (%s)\n", u.BinaryName, u.latestVersion.Version)
	downloadUrl, err := u.latestDownloadUrl()
	if err != nil {
		console.Error("update failed: %s\n", err)
		return err
	}
	if err = u.doSelfUpdate(downloadUrl); err != nil {
		console.Error("update failed: %s\n", err)
		return err
	}

	return nil
}

func (u *SelfUpdater) checkSelfUpdate() <-chan bool {
	ch := make(chan bool, 1)
	go func() {
		data, err := helper.LoadFile(u.LatestVersionUrl)
		if err != nil {
			log.Infof(err.Error())
			ch <- false
			return
		}

		u.latestVersion = LatestVersion{}
		err = json.Unmarshal(data, &u.latestVersion)
		if err != nil {
			log.Errorf(err.Error())
			ch <- false
			return
		}

		versionParts := strings.Split(u.CurrentVersion, "-")
		suffixVersion := versionParts[len(versionParts)-1]

		ch <- u.latestVersion.Version != suffixVersion &&
			u.User.InPartition(u.latestVersion.StartPartition, u.latestVersion.EndPartition)
	}()
	return ch
}

func (u *SelfUpdater) doSelfUpdate(url string) error {
	resp, err := helper.HttpGetWrapper(url)
	if err != nil {
		return fmt.Errorf("cannot download the new version from %s: %v", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cannot download the new version from %s: code %d", url, resp.StatusCode)
	}

	defer resp.Body.Close()
	err = update.Apply(resp.Body, update.Options{})
	if err != nil {
		if err = update.RollbackError(err); err != nil {
			return fmt.Errorf("update failed, unfortunately, the rollback did not work neither: %v\nplease contact #build-services team", err)
		}
		console.Warn("update failed, rollback to previous version: %v\n", err)
	}

	return nil
}

func (u *SelfUpdater) latestDownloadUrl() (string, error) {
	updateUrl, err := url.Parse(u.SelfUpdateRootUrl)
	if err != nil {
		return "", err
	}

	updateUrl.Path = path.Join(updateUrl.Path, "current", runtime.GOOS, runtime.GOARCH, u.binaryFileName())
	return updateUrl.String(), nil
}

func (u *SelfUpdater) binaryFileName() string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("%s.exe", u.BinaryName)
	}
	return u.BinaryName
}
