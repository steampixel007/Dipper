package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/source"
	"github.com/steampixel007/Dipper/internal/update"
)

func softwareAutoFixture(t *testing.T) (*State, *ui.Tester, chan func()) {
	t.Helper()
	previous := version
	version = "v0.1.2"
	t.Cleanup(func() { version = previous })
	s, _, queue := softwareFixture(t)
	s.Platform = "windows"
	s.SoftwareAuto = true
	s.SoftwareAutoDownload = true
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() {
			s.softwareNotice(c)
			s.softwareUpdateSettings(c)
		})
		s.softwareUpdateDialog(c)
	}, 1320, 860)
	return s, tt, queue
}

func softwareVerifiedDownload(_ context.Context, offer update.Offer, root string, _ func(int64, int64)) (update.Download, error) {
	directory, err := os.MkdirTemp(root, "dipper-update-download-")
	if err != nil {
		return update.Download{}, err
	}
	body := []byte("verified Dipper executable fixture")
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entry, err := writer.Create("Dipper.exe")
	if err != nil {
		return update.Download{}, err
	}
	if _, err := entry.Write(body); err != nil {
		return update.Download{}, err
	}
	if err := writer.Close(); err != nil {
		return update.Download{}, err
	}
	download := update.Download{
		Directory: directory, Package: filepath.Join(directory, offer.Package.Name),
		Executable: filepath.Join(directory, "Dipper.exe"),
	}
	if err := os.WriteFile(download.Package, archive.Bytes(), 0600); err != nil {
		return update.Download{}, err
	}
	if err := os.WriteFile(download.Executable, body, 0700); err != nil {
		return update.Download{}, err
	}
	download.PackageSHA256, err = update.Digest(download.Package)
	if err != nil {
		return update.Download{}, err
	}
	download.SHA256, err = update.Digest(download.Executable)
	return download, err
}

func softwareStartAutoDownload(t *testing.T, s *State, tt *ui.Tester, queue chan func()) {
	t.Helper()
	s.startAutomaticSoftwareCheck()
	softwareApply(t, tt, queue)
	if !s.SoftwareUpdate.Busy || s.SoftwareUpdate.Dialog {
		t.Fatalf("new portable release did not start a nonmodal background download: %+v", s.SoftwareUpdate)
	}
	softwareApply(t, tt, queue)
	if s.SoftwareUpdate.Busy || s.SoftwareUpdate.Downloaded.Executable == "" || s.SoftwareUpdate.Dialog {
		t.Fatalf("automatic download did not finish without opening a dialog: %+v", s.SoftwareUpdate)
	}
}

func TestSoftwareAutoDownloadPortableAndNoticeRestart(t *testing.T) {
	s, tt, queue := softwareAutoFixture(t)
	var checks, downloads, prepares atomic.Int32
	quits := 0
	installer := &softwareInstallerFixture{}
	s.SoftwareClient = softwareClientFixture{
		check: func(context.Context) (update.Offer, error) {
			checks.Add(1)
			return softwareOffer(), nil
		},
		download: func(ctx context.Context, offer update.Offer, root string, progress func(int64, int64)) (update.Download, error) {
			downloads.Add(1)
			config, err := softwareCanonicalPath(s.ConfigDir)
			if err != nil {
				return update.Download{}, err
			}
			if root != filepath.Join(config, "updates") {
				return update.Download{}, fmt.Errorf("download is not in persistent storage: %s", root)
			}
			return softwareVerifiedDownload(ctx, offer, root, progress)
		},
	}
	s.PrepareSoftware = func(_ context.Context, target string, downloaded update.Download, _ []string) (softwareInstaller, error) {
		prepares.Add(1)
		if target != filepath.Join(s.Dir, "Dipper.exe") || update.VerifyPortableDownload(downloaded) != nil {
			return nil, errors.New("install lost the verified portable download")
		}
		return installer, nil
	}
	s.Quit = func() {
		if s.prepareAppQuit() {
			quits++
		}
	}
	softwareStartAutoDownload(t, s, tt, queue)
	message := "新版本 v9.0.0 已下载，点击重启更新"
	uiRequireText(t, tt, message)
	if checks.Load() != 1 || downloads.Load() != 1 || prepares.Load() != 0 || quits != 0 {
		t.Fatal("background download contacted releases more than once or installed before consent")
	}
	pref, err := readPreferences(s.ConfigDir)
	if err != nil || pref.SoftwarePending == nil || !pref.SoftwareAutoDownload {
		t.Fatalf("verified pending download was not persisted: %+v %v", pref, err)
	}
	s.checkSoftware(true)
	s.SoftwareUpdate.Dialog = false
	s.startAutomaticSoftwareCheck()
	tt.Frame()
	if checks.Load() != 1 || s.SoftwareUpdate.Downloaded.Executable == "" {
		t.Fatal("a repeated check discarded or redownloaded an installable update")
	}
	s.Page = "following"
	tt = ui.NewTester(s.view, 1040, 720)
	uiRequireText(t, tt, message)
	uiClick(t, tt, message)
	if !s.SoftwareUpdate.Busy || !s.SoftwareUpdate.Installing {
		t.Fatal("notice click did not start portable installer preparation")
	}
	softwareApply(t, tt, queue)
	if prepares.Load() != 1 || quits != 1 || installer.commits.Load() != 1 || !s.SoftwareUpdate.Committed {
		t.Fatalf("notice click did not prepare and commit through graceful quit: prepares=%d quits=%d commits=%d", prepares.Load(), quits, installer.commits.Load())
	}
}

func TestSoftwareAutoDownloadPreferencePersistenceAndRollback(t *testing.T) {
	s, tt, _ := softwareAutoFixture(t)
	s.Source = source.New("http://127.0.0.1")
	if err := s.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if s.SoftwareAutoDownload {
		t.Fatal("missing automatic-download preference must default to disabled")
	}
	tt.Frame()
	uiClick(t, tt, softwareText.autoDownload)
	pref, err := readPreferences(s.ConfigDir)
	if err != nil || !pref.SoftwareAutoDownload || !s.SoftwareAutoDownload {
		t.Fatalf("automatic download switch did not persist enabled choice: %+v %v", pref, err)
	}
	s.SoftwareAutoDownload = false
	if err := s.loadPreferences(); err != nil || !s.SoftwareAutoDownload {
		t.Fatalf("saved automatic download switch was not restored: %v", err)
	}
	uiClick(t, tt, softwareText.autoDownload)
	pref, err = readPreferences(s.ConfigDir)
	if err != nil || pref.SoftwareAutoDownload || s.SoftwareAutoDownload {
		t.Fatal("automatic download switch did not persist disabled choice")
	}
	if err := os.Mkdir(filepath.Join(s.ConfigDir, "preferences.json.tmp"), 0700); err != nil {
		t.Fatal(err)
	}
	uiClick(t, tt, softwareText.autoDownload)
	if s.SoftwareAutoDownload || s.SaveError == "" {
		t.Fatal("failed switch write did not roll back and report the save error")
	}
}

func TestSoftwareAutoDownloadEligibility(t *testing.T) {
	for _, scenario := range []string{"portable", "disabled", "nonportable", "linux", "darwin", "development", "development-offer", "installed", "manual"} {
		t.Run(scenario, func(t *testing.T) {
			s, tt, queue := softwareAutoFixture(t)
			offer := softwareOffer()
			switch scenario {
			case "disabled":
				s.SoftwareAutoDownload = false
			case "nonportable":
				offer.Portable = false
			case "linux", "darwin":
				s.Platform = scenario
				offer.Portable = false
			case "development":
				version = "dev"
			case "development-offer":
				offer.Development = true
			case "installed":
				if err := os.WriteFile(filepath.Join(s.Dir, "Uninstall.exe"), []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
				offer.Release.Assets = []update.Asset{{Name: "dipper_9.0.0_windows_amd64_setup.exe", Size: 100}}
			}
			var checks, downloads atomic.Int32
			s.SoftwareClient = softwareClientFixture{
				check: func(context.Context) (update.Offer, error) {
					checks.Add(1)
					return offer, nil
				},
				download: func(ctx context.Context, offer update.Offer, root string, progress func(int64, int64)) (update.Download, error) {
					downloads.Add(1)
					if scenario == "portable" {
						return softwareVerifiedDownload(ctx, offer, root, progress)
					}
					return update.Download{}, errors.New("must not download automatically")
				},
			}
			if scenario == "manual" {
				s.checkSoftware(true)
			} else {
				s.startAutomaticSoftwareCheck()
			}
			if scenario == "development" {
				if s.SoftwareUpdate.Busy || checks.Load() != 0 {
					t.Fatal("development build started automatic update work")
				}
				return
			}
			softwareApply(t, tt, queue)
			if scenario == "portable" {
				if !s.SoftwareUpdate.Busy || s.SoftwareUpdate.Dialog {
					t.Fatal("eligible Windows portable offer did not start a quiet download")
				}
				softwareApply(t, tt, queue)
				if downloads.Load() != 1 || s.SoftwareUpdate.Downloaded.Executable == "" {
					t.Fatal("eligible Windows portable offer was not downloaded")
				}
				return
			}
			if downloads.Load() != 0 || s.SoftwareUpdate.Busy || s.SoftwareUpdate.Dialog != (scenario == "manual") {
				t.Fatalf("ineligible check changed its existing behavior: downloads=%d state=%+v", downloads.Load(), s.SoftwareUpdate)
			}
			if scenario == "installed" && s.SoftwareUpdate.Offer.Portable {
				t.Fatal("installed Windows copy was treated as portable")
			}
		})
	}
}

func TestSoftwarePendingSurvivesRestartAndInstalls(t *testing.T) {
	s, tt, queue := softwareAutoFixture(t)
	ctx, cancel := context.WithCancel(s.Ctx)
	s.Ctx = ctx
	t.Cleanup(cancel)
	s.SoftwareClient = softwareClientFixture{
		check:    func(context.Context) (update.Offer, error) { return softwareOffer(), nil },
		download: softwareVerifiedDownload,
	}
	softwareStartAutoDownload(t, s, tt, queue)
	pending := s.SoftwarePending
	s.SoftwareAuto = false
	s.SoftwareAutoDownload = false
	if !s.prepareAppQuit() {
		t.Fatal("ordinary quit rejected a completed background download")
	}
	cancel()
	restarted, rt, results := softwareAutoFixture(t)
	restarted.ConfigDir = s.ConfigDir
	restarted.Source = source.New("http://127.0.0.1")
	if err := restarted.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	restarted.SoftwareUpdate.StartupError = softwareText.startupError
	restarted.startAutomaticSoftwareCheck()
	if !restarted.SoftwareUpdate.Busy {
		t.Fatal("restart did not schedule verification of the saved pending update")
	}
	softwareApply(t, rt, results)
	if restarted.SoftwareUpdate.Downloaded != pending.Download || restarted.SoftwareUpdate.Dialog ||
		restarted.SoftwareUpdate.StartupError != softwareText.startupError {
		t.Fatalf("restart did not recover the verified update quietly: %+v", restarted.SoftwareUpdate)
	}
	message := "新版本 v9.0.0 已下载，点击重启更新"
	uiRequireText(t, rt, message)
	installer := &softwareInstallerFixture{}
	var prepares atomic.Int32
	quits := 0
	restarted.PrepareSoftware = func(_ context.Context, _ string, downloaded update.Download, _ []string) (softwareInstaller, error) {
		prepares.Add(1)
		if downloaded != pending.Download {
			return nil, errors.New("restored download changed before installation")
		}
		return installer, nil
	}
	restarted.Quit = func() {
		if restarted.prepareAppQuit() {
			quits++
		}
	}
	restarted.startAutomaticSoftwareCheck()
	uiClick(t, rt, message)
	softwareApply(t, rt, results)
	if prepares.Load() != 1 || quits != 1 || installer.commits.Load() != 1 {
		t.Fatal("restored notice did not reach the existing prepare/quit/commit flow")
	}
}

func TestSoftwarePendingRejectsInvalidDownloads(t *testing.T) {
	for _, scenario := range []string{"package", "executable", "missing", "outdated", "outside", "installed"} {
		t.Run(scenario, func(t *testing.T) {
			s, tt, queue := softwareAutoFixture(t)
			s.SoftwareAuto = false
			root := filepath.Join(s.ConfigDir, "updates")
			if scenario == "outside" {
				root = t.TempDir()
			} else if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			root, err := softwareCanonicalPath(root)
			if err != nil {
				t.Fatal(err)
			}
			download, err := softwareVerifiedDownload(s.Ctx, softwareOffer(), root, nil)
			if err != nil {
				t.Fatal(err)
			}
			s.SoftwarePending = &softwarePending{Offer: softwareOffer(), Download: download}
			switch scenario {
			case "package":
				err = os.WriteFile(download.Package, []byte("tampered package"), 0600)
			case "executable":
				err = os.WriteFile(download.Executable, []byte("tampered executable"), 0700)
			case "missing":
				err = os.Remove(download.Executable)
			case "outdated":
				version = "v9.0.0"
			case "installed":
				err = os.WriteFile(filepath.Join(s.Dir, "Uninstall.exe"), []byte("fixture"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := s.savePreferences(); err != nil {
				t.Fatal(err)
			}
			s.startAutomaticSoftwareCheck()
			if !s.SoftwareUpdate.Busy {
				t.Fatal("startup ignored the pending update instead of verifying and rejecting it")
			}
			softwareApply(t, tt, queue)
			pref, err := readPreferences(s.ConfigDir)
			if s.SoftwareUpdate.Downloaded.Executable != "" || s.SoftwarePending != nil ||
				s.SoftwareUpdate.Available || s.SoftwareUpdate.Busy || err != nil || pref.SoftwarePending != nil {
				t.Fatalf("invalid pending update was offered or retained as installable: %+v %v", s.SoftwareUpdate, err)
			}
			if scenario == "outside" {
				if _, err := os.Stat(download.Executable); err != nil {
					t.Fatalf("rejecting untrusted metadata deleted files outside update storage: %v", err)
				}
			} else if _, err := os.Stat(download.Directory); !os.IsNotExist(err) {
				t.Fatalf("obsolete or invalid managed update was not reclaimed: %v", err)
			}
		})
	}
}

func TestSoftwareAutoDownloadFailureRemainsNonmodal(t *testing.T) {
	for _, scenario := range []string{"network", "checksum", "save"} {
		t.Run(scenario, func(t *testing.T) {
			s, tt, queue := softwareAutoFixture(t)
			var downloaded update.Download
			s.SoftwareClient = softwareClientFixture{
				check: func(context.Context) (update.Offer, error) { return softwareOffer(), nil },
				download: func(ctx context.Context, offer update.Offer, root string, progress func(int64, int64)) (update.Download, error) {
					if scenario == "network" {
						return update.Download{}, errors.New("offline fixture")
					}
					var err error
					downloaded, err = softwareVerifiedDownload(ctx, offer, root, progress)
					if err != nil {
						return downloaded, err
					}
					if scenario == "checksum" {
						err = os.WriteFile(downloaded.Executable, []byte("bad executable"), 0700)
					}
					if scenario == "save" {
						err = os.Mkdir(filepath.Join(s.ConfigDir, "preferences.json.tmp"), 0700)
					}
					return downloaded, err
				},
			}
			s.startAutomaticSoftwareCheck()
			softwareApply(t, tt, queue)
			if !s.SoftwareUpdate.Busy {
				t.Fatal("eligible release never reached automatic downloading")
			}
			softwareApply(t, tt, queue)
			if s.SoftwareUpdate.Busy || s.SoftwareUpdate.Dialog || s.SoftwareUpdate.Downloaded.Executable != "" ||
				s.SoftwarePending != nil || s.SoftwareUpdate.Error == "" {
				t.Fatalf("failed automatic download opened a dialog or exposed an install action: %+v", s.SoftwareUpdate)
			}
			if downloaded.Directory != "" {
				if _, err := os.Stat(downloaded.Directory); !os.IsNotExist(err) {
					t.Fatalf("failed download left an untracked package: %v", err)
				}
			}
		})
	}
}

func TestSoftwareAutoDownloadCancellationDropsStaleCompletion(t *testing.T) {
	s, tt, queue := softwareAutoFixture(t)
	release := make(chan struct{})
	started := make(chan struct{})
	var downloaded update.Download
	s.SoftwareClient = softwareClientFixture{
		check: func(context.Context) (update.Offer, error) { return softwareOffer(), nil },
		download: func(ctx context.Context, offer update.Offer, root string, progress func(int64, int64)) (update.Download, error) {
			close(started)
			<-release
			var err error
			downloaded, err = softwareVerifiedDownload(ctx, offer, root, progress)
			return downloaded, err
		},
	}
	s.startAutomaticSoftwareCheck()
	softwareApply(t, tt, queue)
	if !s.SoftwareUpdate.Busy {
		close(release)
		t.Fatal("automatic check did not start the cancellable download")
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("fixture downloader did not start")
	}
	s.cancelSoftwareDownload()
	close(release)
	softwareApply(t, tt, queue)
	if s.SoftwareUpdate.Downloaded.Executable != "" || s.SoftwarePending != nil || s.SoftwareUpdate.Busy {
		t.Fatal("cancelled automatic download was published by a stale completion")
	}
	if _, err := os.Stat(downloaded.Directory); !os.IsNotExist(err) {
		t.Fatalf("cancelled automatic download was not reclaimed: %v", err)
	}
	pref, err := readPreferences(s.ConfigDir)
	if err != nil || pref.SoftwarePending != nil {
		t.Fatal("cancelled automatic download was persisted")
	}
}

func TestSoftwarePendingPreferenceSurvivesOtherSettingSaves(t *testing.T) {
	s, tt, queue := softwareAutoFixture(t)
	s.SoftwareClient = softwareClientFixture{
		check:    func(context.Context) (update.Offer, error) { return softwareOffer(), nil },
		download: softwareVerifiedDownload,
	}
	softwareStartAutoDownload(t, s, tt, queue)
	s.SoftwareCheckedAt = time.Now().Add(-48 * time.Hour)
	s.setSoftwareAutomatic(false)
	s.setSoftwareAutoDownload(false)
	pref, err := readPreferences(s.ConfigDir)
	if err != nil || pref.SoftwarePending == nil || pref.SoftwarePending.Download != s.SoftwareUpdate.Downloaded {
		t.Fatalf("unrelated preference save erased the pending update: %+v %v", pref, err)
	}
}

func TestSoftwarePendingQuitVetoKeepsRetryableNotice(t *testing.T) {
	s, tt, queue := softwareAutoFixture(t)
	s.SoftwareClient = softwareClientFixture{
		check:    func(context.Context) (update.Offer, error) { return softwareOffer(), nil },
		download: softwareVerifiedDownload,
	}
	softwareStartAutoDownload(t, s, tt, queue)
	installer := &softwareInstallerFixture{err: errors.New("commit veto fixture")}
	s.PrepareSoftware = func(context.Context, string, update.Download, []string) (softwareInstaller, error) {
		return installer, nil
	}
	quits := 0
	s.Quit = func() {
		if s.prepareAppQuit() {
			quits++
		}
	}
	message := "新版本 v9.0.0 已下载，点击重启更新"
	uiClick(t, tt, message)
	softwareApply(t, tt, queue)
	softwareApply(t, tt, queue)
	if quits != 0 || installer.aborts.Load() != 1 || s.SoftwareUpdate.Busy || s.SoftwareUpdate.Installing {
		t.Fatal("quit veto did not abort preparation and keep the application usable")
	}
	if s.SoftwarePending == nil || update.VerifyPortableDownload(s.SoftwareUpdate.Downloaded) != nil {
		t.Fatal("quit veto destroyed the verified download")
	}
	uiRequireText(t, tt, message)
	installer.err = nil
	uiClick(t, tt, message)
	softwareApply(t, tt, queue)
	if quits != 1 || installer.commits.Load() != 2 {
		t.Fatal("notice could not retry the existing installation after a quit veto")
	}
}

func TestSoftwareInstallFailureDiscardsInvalidDownloadAndRecovers(t *testing.T) {
	s, tt, queue := softwareAutoFixture(t)
	s.SoftwareClient = softwareClientFixture{
		check:    func(context.Context) (update.Offer, error) { return softwareOffer(), nil },
		download: softwareVerifiedDownload,
	}
	softwareStartAutoDownload(t, s, tt, queue)
	invalid := s.SoftwarePending
	if invalid == nil {
		t.Fatal("automatic download did not persist a pending update")
	}
	if err := os.WriteFile(invalid.Download.Executable, []byte("tampered executable"), 0700); err != nil {
		t.Fatal(err)
	}
	var prepares atomic.Int32
	s.PrepareSoftware = func(context.Context, string, update.Download, []string) (softwareInstaller, error) {
		prepares.Add(1)
		return nil, errors.New("fixture helper rejected the tampered executable")
	}
	message := "新版本 v9.0.0 已下载，点击重启更新"
	uiClick(t, tt, message)
	softwareApply(t, tt, queue)
	if prepares.Load() != 1 || s.SoftwareUpdate.Error == "" || s.SoftwareUpdate.Installing {
		t.Fatalf("invalid download failure was not reported: %+v", s.SoftwareUpdate)
	}
	if s.SoftwarePending != nil || s.SoftwareUpdate.Downloaded.Executable != "" {
		t.Fatalf("invalid pending download stayed installable: %+v", s.SoftwareUpdate)
	}
	pref, err := readPreferences(s.ConfigDir)
	if err != nil || pref.SoftwarePending != nil {
		t.Fatalf("invalid pending download stayed persisted: %+v %v", pref, err)
	}
	if _, err := os.Stat(invalid.Download.Directory); !os.IsNotExist(err) {
		t.Fatalf("invalid pending download was not reclaimed: %v", err)
	}
	var checks, downloads atomic.Int32
	s.SoftwareClient = softwareClientFixture{
		check: func(context.Context) (update.Offer, error) {
			checks.Add(1)
			return softwareOffer(), nil
		},
		download: func(ctx context.Context, offer update.Offer, root string, progress func(int64, int64)) (update.Download, error) {
			downloads.Add(1)
			return softwareVerifiedDownload(ctx, offer, root, progress)
		},
	}
	s.SoftwareCheckedAt = time.Time{}
	s.startAutomaticSoftwareCheck()
	if !s.SoftwareUpdate.Busy {
		t.Fatal("recovery did not start a fresh release check")
	}
	softwareApply(t, tt, queue)
	softwareApply(t, tt, queue)
	if checks.Load() != 1 || downloads.Load() != 1 || s.SoftwarePending == nil ||
		s.SoftwareUpdate.Downloaded.Executable == "" || s.SoftwareUpdate.Dialog {
		t.Fatalf("discarded download could not be checked and downloaded again: checks=%d downloads=%d state=%+v", checks.Load(), downloads.Load(), s.SoftwareUpdate)
	}
}

func TestSoftwareTransientInstallFailureKeepsVerifiedDownload(t *testing.T) {
	s, tt, queue := softwareAutoFixture(t)
	s.SoftwareClient = softwareClientFixture{
		check:    func(context.Context) (update.Offer, error) { return softwareOffer(), nil },
		download: softwareVerifiedDownload,
	}
	softwareStartAutoDownload(t, s, tt, queue)
	pending := s.SoftwarePending
	if pending == nil {
		t.Fatal("automatic download did not persist a pending update")
	}
	s.PrepareSoftware = func(context.Context, string, update.Download, []string) (softwareInstaller, error) {
		return nil, errors.New("fixture folder temporarily unwritable")
	}
	message := "新版本 v9.0.0 已下载，点击重启更新"
	uiClick(t, tt, message)
	softwareApply(t, tt, queue)
	if s.SoftwarePending != pending || update.VerifyPortableDownload(s.SoftwareUpdate.Downloaded) != nil {
		t.Fatal("transient preparation failure discarded a still-verified download")
	}
	if _, err := os.Stat(pending.Download.Directory); err != nil {
		t.Fatalf("transient preparation failure removed the managed download: %v", err)
	}
	pref, err := readPreferences(s.ConfigDir)
	if err != nil || pref.SoftwarePending == nil {
		t.Fatalf("transient preparation failure erased the pending record: %+v %v", pref, err)
	}
	uiRequireText(t, tt, message)
}
