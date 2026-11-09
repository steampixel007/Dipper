package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/update"
)

type softwareClientFixture struct {
	check    func(context.Context) (update.Offer, error)
	download func(context.Context, update.Offer, string, func(int64, int64)) (update.Download, error)
}

func (client softwareClientFixture) Check(ctx context.Context, _, _, _ string) (update.Offer, error) {
	return client.check(ctx)
}

func (client softwareClientFixture) Download(ctx context.Context, offer update.Offer, dir string, progress func(int64, int64)) (update.Download, error) {
	return client.download(ctx, offer, dir, progress)
}

type softwareInstallerFixture struct {
	commits, aborts atomic.Int32
	err             error
}

func (installer *softwareInstallerFixture) Commit() error {
	installer.commits.Add(1)
	return installer.err
}

func (installer *softwareInstallerFixture) Abort() { installer.aborts.Add(1) }

func softwareFixture(t *testing.T) (*State, *ui.Tester, chan func()) {
	t.Helper()
	s := uiFixture(t, "settings")
	ctx, cancel := context.WithCancel(context.Background())
	s.Ctx = ctx
	s.Platform = "windows"
	queue := make(chan func(), 32)
	s.Update = func(fn func()) { queue <- fn }
	s.SoftwareExecutable = func() (string, error) { return filepath.Join(s.Dir, "Dipper.exe"), nil }
	tt := ui.NewTester(func(c *ui.Context) {
		if !s.SoftwareUpdate.Dialog {
			s.softwareUpdateSettings(c)
		}
		s.softwareUpdateDialog(c)
	}, 1320, 860)
	t.Cleanup(func() {
		s.abortSoftwareUpdate()
		cancel()
	})
	return s, tt, queue
}

func softwareApply(t *testing.T, tt *ui.Tester, queue chan func()) {
	t.Helper()
	select {
	case fn := <-queue:
		fn()
		tt.Frame()
	case <-time.After(5 * time.Second):
		t.Fatal("software operation did not post its UI result")
	}
}

func softwareRequireFailure(t *testing.T, tt *ui.Tester, update softwareUpdate) {
	t.Helper()
	if update.Error == "" || update.Status != "" {
		t.Fatalf("software failure must use only the error field: status=%q error=%q", update.Status, update.Error)
	}
	if count := strings.Count(strings.Join(tt.Texts(), "\n"), update.Error); count != 1 {
		t.Fatalf("software failure rendered %d times, want once: %q", count, update.Error)
	}
}

func softwareOffer() update.Offer {
	asset := update.Asset{
		Name: "dipper_9.0.0_windows_amd64.zip", Size: 100,
		URL: "https://github.com/steampixel007/Dipper/releases/download/v9.0.0/dipper_9.0.0_windows_amd64.zip",
	}
	return update.Offer{
		Available: true, Portable: true,
		Release: update.Release{Tag: "v9.0.0", Notes: "Release notes fixture", Assets: []update.Asset{asset}},
		Package: asset,
	}
}

func TestSoftwareSettingsManualCheckAndDownloadClicks(t *testing.T) {
	s, tt, queue := softwareFixture(t)
	var checks, downloads atomic.Int32
	dir := filepath.Join(s.Dir, "download")
	s.SoftwareClient = softwareClientFixture{
		check: func(context.Context) (update.Offer, error) {
			checks.Add(1)
			return softwareOffer(), nil
		},
		download: func(_ context.Context, offer update.Offer, _ string, progress func(int64, int64)) (update.Download, error) {
			downloads.Add(1)
			os.Mkdir(dir, 0700)
			progress(50, 100)
			return update.Download{Directory: dir, Package: filepath.Join(dir, offer.Package.Name), Executable: filepath.Join(dir, "Dipper.exe")}, nil
		},
	}
	uiRequireText(t, tt, softwareText.title, softwareText.current+"："+updateVersionLabel(version), softwareText.check)
	uiClick(t, tt, softwareText.check)
	if !s.SoftwareUpdate.Busy || !s.SoftwareUpdate.Dialog {
		t.Fatal("manual check click did not show pending operation")
	}
	softwareApply(t, tt, queue)
	uiRequireText(t, tt, "Release notes fixture", softwareText.download)
	if checks.Load() != 1 || !s.SoftwareUpdate.Available {
		t.Fatal("manual check did not invoke software release client")
	}
	uiClick(t, tt, softwareText.download)
	softwareApply(t, tt, queue)
	if s.SoftwareUpdate.Progress != 50 {
		t.Fatal("download progress not rendered through update loop")
	}
	softwareApply(t, tt, queue)
	uiRequireText(t, tt, softwareText.install)
	if downloads.Load() != 1 || s.SoftwareUpdate.Installer != nil || s.SoftwareUpdate.Installing || s.SoftwareUpdate.Downloaded.Executable == "" {
		t.Fatal("download must retain verified bytes without starting helper or quitting")
	}
	uiClick(t, tt, softwareText.installLater)
	if s.SoftwareUpdate.Downloaded.Executable == "" || s.SoftwareUpdate.Dialog {
		t.Fatal("postponing installation discarded downloaded update")
	}
}

func TestSoftwareAutomaticDailyAndPreferencePersistence(t *testing.T) {
	oldVersion := version
	version = "0.1.2"
	defer func() { version = oldVersion }()
	s, tt, queue := softwareFixture(t)
	var calls atomic.Int32
	s.SoftwareAuto = true
	s.SoftwareClient = softwareClientFixture{check: func(context.Context) (update.Offer, error) {
		calls.Add(1)
		return update.Offer{}, errors.New("offline fixture")
	}}
	s.startAutomaticSoftwareCheck()
	softwareApply(t, tt, queue)
	if s.SoftwareUpdate.Error != "" || s.SoftwareUpdate.Status != "" || s.SoftwareUpdate.Dialog {
		t.Fatal("automatic check failure interrupted the user")
	}
	s.startAutomaticSoftwareCheck()
	pref, err := readPreferences(s.Dir)
	if calls.Load() != 1 || err != nil || pref.SoftwareCheckedAt.IsZero() {
		t.Fatalf("automatic attempt not limited/persisted daily: calls=%d pref=%+v %v", calls.Load(), pref, err)
	}
	uiClick(t, tt, softwareText.automatic)
	pref, err = readPreferences(s.Dir)
	if s.SoftwareAuto || err != nil || pref.SoftwareAuto == nil || *pref.SoftwareAuto {
		t.Fatal("automatic check switch did not persist disabled choice")
	}
	s.SoftwareCheckedAt = time.Now().Add(-48 * time.Hour)
	s.startAutomaticSoftwareCheck()
	if calls.Load() != 1 {
		t.Fatal("disabled automatic checks still contacted releases")
	}
	uiClick(t, tt, softwareText.check)
	softwareApply(t, tt, queue)
	if !strings.Contains(s.SoftwareUpdate.Error, softwareText.networkAdvice) || !s.SoftwareUpdate.Dialog {
		t.Fatal("manual failure omitted actionable explanation")
	}
	uiRequireText(t, tt, softwareText.retry)
	uiClick(t, tt, softwareText.retry)
	softwareApply(t, tt, queue)
	if calls.Load() != 3 {
		t.Fatal("manual retry did not retry release lookup")
	}
	s.SoftwareUpdate.Dialog = false
	tt.Frame()
	os.Mkdir(filepath.Join(s.Dir, "preferences.json.tmp"), 0700)
	uiClick(t, tt, softwareText.automatic)
	if s.SoftwareAuto || s.SaveError == "" {
		t.Fatal("failed switch write changed persisted automatic preference")
	}
}

func TestSoftwareAutomaticCheckTreatsFutureTimestampAsDue(t *testing.T) {
	oldVersion := version
	version = "0.1.2"
	defer func() { version = oldVersion }()
	s, tt, queue := softwareFixture(t)
	var calls atomic.Int32
	s.SoftwareAuto = true
	s.SoftwareCheckedAt = time.Now().Add(48 * time.Hour)
	s.SoftwareClient = softwareClientFixture{check: func(context.Context) (update.Offer, error) {
		calls.Add(1)
		return update.Offer{}, nil
	}}
	s.startAutomaticSoftwareCheck()
	softwareApply(t, tt, queue)
	if calls.Load() != 1 {
		t.Fatal("a future last-check timestamp suppressed the daily automatic check")
	}
	if !s.SoftwareCheckedAt.Before(time.Now().Add(time.Hour)) {
		t.Fatalf("automatic check did not replace the invalid future timestamp: %v", s.SoftwareCheckedAt)
	}
}

func TestSoftwareReadinessErrorSurvivesAutomaticCheck(t *testing.T) {
	oldVersion := version
	version = "0.1.2"
	defer func() { version = oldVersion }()
	s, tt, queue := softwareFixture(t)
	s.SoftwareAuto = true
	s.SoftwareUpdate.StartupError = softwareText.startupError
	s.SoftwareClient = softwareClientFixture{check: func(context.Context) (update.Offer, error) {
		return softwareOffer(), nil
	}}
	tt.Frame()
	uiRequireText(t, tt, softwareText.startupError)
	s.startAutomaticSoftwareCheck()
	softwareApply(t, tt, queue)
	if s.SoftwareUpdate.StartupError != softwareText.startupError {
		t.Fatal("automatic check cleared the post-update readiness failure")
	}
	uiRequireText(t, tt, softwareText.startupError)
}

func TestSoftwareManualFailureDoesNotBecomeGlobalNotice(t *testing.T) {
	oldVersion := version
	version = "0.1.2"
	defer func() { version = oldVersion }()
	s := uiFixture(t, "settings")
	ctx, cancel := context.WithCancel(context.Background())
	s.Ctx = ctx
	queue := make(chan func(), 8)
	s.Update = func(fn func()) { queue <- fn }
	s.SoftwareExecutable = func() (string, error) { return filepath.Join(s.Dir, "Dipper.exe"), nil }
	s.SoftwareClient = softwareClientFixture{check: func(context.Context) (update.Offer, error) {
		return update.Offer{}, errors.New("offline fixture")
	}}
	tt := ui.NewTester(s.view, 1040, 720)
	t.Cleanup(func() {
		s.abortSoftwareUpdate()
		cancel()
	})

	uiScrollTo(t, tt, softwareText.check)
	uiClick(t, tt, softwareText.check)
	softwareApply(t, tt, queue)
	if s.SoftwareUpdate.Error == "" || !s.SoftwareUpdate.Dialog {
		t.Fatal("manual release failure was not kept in the software dialog")
	}
	notice := ui.NewTester(func(c *ui.Context) { s.softwareNotice(c) }, 1040, 720)
	notice.Frame()
	if notice.HasText(s.SoftwareUpdate.Error) {
		t.Fatal("manual release failure leaked into the global software notice")
	}
	s.SoftwareUpdate.StartupError = "update readiness failed fixture"
	notice.Frame()
	uiRequireText(t, notice, "update readiness failed fixture")
}

func TestSoftwareCancelDropsStaleCheckAndDownload(t *testing.T) {
	s, tt, queue := softwareFixture(t)
	release := make(chan struct{})
	finished := make(chan struct{})
	s.SoftwareClient = softwareClientFixture{check: func(ctx context.Context) (update.Offer, error) {
		defer close(finished)
		<-release
		return softwareOffer(), nil
	}}
	uiClick(t, tt, softwareText.check)
	uiClick(t, tt, softwareText.cancel)
	close(release)
	<-finished
	softwareApply(t, tt, queue)
	if s.SoftwareUpdate.Available || s.SoftwareUpdate.Busy || s.SoftwareUpdate.Status != softwareText.cancelled {
		t.Fatal("stale canceled check resurrected an update notice")
	}
	downloadRelease := make(chan struct{})
	dir := filepath.Join(s.Dir, "late-download")
	s.SoftwareClient = softwareClientFixture{download: func(ctx context.Context, _ update.Offer, _ string, progress func(int64, int64)) (update.Download, error) {
		<-downloadRelease
		os.Mkdir(dir, 0700)
		return update.Download{Directory: dir}, nil
	}}
	s.SoftwareUpdate.Available, s.SoftwareUpdate.Offer = true, softwareOffer()
	tt.Frame()
	uiClick(t, tt, softwareText.download)
	uiClick(t, tt, softwareText.cancel)
	close(downloadRelease)
	softwareApply(t, tt, queue)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("stale canceled download retained files")
	}
	if s.SoftwareUpdate.Downloaded.Directory != "" || s.SoftwareUpdate.Busy {
		t.Fatal("stale download replaced canceled UI state")
	}
}

func TestSoftwareCancelDuringPreparationLeavesRetryableState(t *testing.T) {
	s, tt, queue := softwareFixture(t)
	first, second := &softwareInstallerFixture{}, &softwareInstallerFixture{}
	preparing := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	s.PrepareSoftware = func(context.Context, string, update.Download, []string) (softwareInstaller, error) {
		if calls.Add(1) == 1 {
			close(preparing)
			<-release
			return first, nil
		}
		return second, nil
	}
	dir := filepath.Join(s.Dir, "staged")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(dir, "Dipper.exe")
	if err := os.WriteFile(executable, []byte("fixture executable"), 0600); err != nil {
		t.Fatal(err)
	}
	s.SoftwareUpdate = softwareUpdate{
		Available: true,
		Offer:     softwareOffer(),
		Downloaded: update.Download{
			Directory:  dir,
			Executable: executable,
			Package:    filepath.Join(dir, "update.zip"),
		},
	}
	tt.Frame()
	uiClick(t, tt, softwareText.install)
	select {
	case <-preparing:
	case <-time.After(5 * time.Second):
		t.Fatal("installation preparation did not start")
	}
	if !s.SoftwareUpdate.Busy || !s.SoftwareUpdate.Installing {
		t.Fatal("installation preparation did not expose its pending state")
	}
	tt.Frame()
	uiClick(t, tt, softwareText.cancel)
	close(release)
	softwareApply(t, tt, queue)
	if s.SoftwareUpdate.Busy || s.SoftwareUpdate.Installing || s.SoftwareUpdate.Installer != nil {
		t.Fatalf("cancelling preparation left installation stuck: busy=%v installing=%v installer=%v", s.SoftwareUpdate.Busy, s.SoftwareUpdate.Installing, s.SoftwareUpdate.Installer)
	}
	if first.aborts.Load() != 1 || s.SoftwareUpdate.Status != softwareText.cancelled {
		t.Fatalf("cancelling preparation did not abort the helper: abort=%d status=%q", first.aborts.Load(), s.SoftwareUpdate.Status)
	}
	if s.SoftwareUpdate.Downloaded.Executable != executable {
		t.Fatal("cancelling preparation discarded the verified download")
	}
	if _, err := os.Stat(executable); err != nil {
		t.Fatalf("cancelling preparation removed downloaded bytes: %v", err)
	}
	uiRequireText(t, tt, softwareText.install)
	uiClick(t, tt, softwareText.install)
	softwareApply(t, tt, queue)
	if calls.Load() != 2 || s.SoftwareUpdate.Installer != second || !s.SoftwareUpdate.Installing {
		t.Fatalf("cancelled preparation was not retryable: calls=%d installer=%v installing=%v", calls.Load(), s.SoftwareUpdate.Installer, s.SoftwareUpdate.Installing)
	}
}

func TestSoftwareInstallClickAndQuitVeto(t *testing.T) {
	for _, scenario := range []string{"success", "save", "busy", "commit", "prepare", "ordinary"} {
		t.Run(scenario, func(t *testing.T) {
			s, tt, queue := softwareFixture(t)
			installer := &softwareInstallerFixture{}
			prepares, quits, shutdowns, shows := 0, 0, 0, 0
			s.ShowWindow = func() { shows++ }
			s.PrepareSoftware = func(_ context.Context, target string, downloaded update.Download, args []string) (softwareInstaller, error) {
				if target != filepath.Join(s.Dir, "Dipper.exe") || downloaded.Executable == "" {
					return nil, errors.New("lost installation context")
				}
				prepares++
				if scenario == "prepare" {
					return nil, errors.New("fixture folder unwritable")
				}
				if scenario == "save" {
					os.Mkdir(filepath.Join(s.Dir, "preferences.json.tmp"), 0700)
				}
				return installer, nil
			}
			s.Quit = func() {
				quits++
				if s.prepareAppQuit() {
					shutdowns++
				}
			}
			dir := filepath.Join(s.Dir, "staged")
			os.Mkdir(dir, 0700)
			s.SoftwareUpdate = softwareUpdate{
				Available: true, Offer: softwareOffer(),
				Downloaded: update.Download{Directory: dir, Executable: filepath.Join(dir, "Dipper.exe"), Package: filepath.Join(dir, "update.zip")},
			}
			if scenario == "commit" {
				installer.err = errors.New("helper unavailable")
			}
			if scenario == "busy" {
				s.Busy = true
			}
			tt.Frame()
			if scenario == "ordinary" {
				s.SoftwareUpdate.Installer = installer
				if !s.prepareAppQuit() || installer.commits.Load() != 0 || installer.aborts.Load() != 1 {
					t.Fatal("ordinary quit committed a postponed update")
				}
				return
			}
			uiClick(t, tt, softwareText.install)
			if scenario == "busy" {
				if prepares != 0 || quits != 0 {
					t.Fatal("busy app started installation")
				}
				return
			}
			softwareApply(t, tt, queue)
			if scenario == "save" || scenario == "commit" {
				softwareApply(t, tt, queue)
				if shutdowns != 0 || installer.aborts.Load() == 0 || s.SoftwareUpdate.Committed || s.SoftwareUpdate.Installing || shows != 1 {
					t.Fatalf("quit veto did not abort replacement and restore window: shutdown=%d abort=%d shows=%d", shutdowns, installer.aborts.Load(), shows)
				}
			} else if scenario == "prepare" {
				if quits != 0 || s.SoftwareUpdate.Error == "" || s.SoftwareUpdate.Downloaded.Directory == "" || s.SoftwareUpdate.Busy {
					t.Fatal("prepare failure lost retryable download or quit")
				}
				softwareRequireFailure(t, tt, s.SoftwareUpdate)
			} else if shutdowns != 1 || installer.commits.Load() != 1 || !s.SoftwareUpdate.Committed || quits != 1 {
				t.Fatal("confirmed install failed to commit once before graceful shutdown")
			}
			if scenario == "commit" {
				softwareRequireFailure(t, tt, s.SoftwareUpdate)
			}
		})
	}
}

func TestSoftwareFailedCheckHidesUnavailableRelease(t *testing.T) {
	s, tt, queue := softwareFixture(t)
	s.SoftwareUpdate = softwareUpdate{
		Available: true,
		Offer:     softwareOffer(),
		Status:    softwareText.available + "v9.0.0",
		Dialog:    true,
	}
	tt.Frame()
	uiRequireText(t, tt, "Release notes fixture")
	s.SoftwareUpdate.Dialog = false
	tt.Frame()
	s.SoftwareClient = softwareClientFixture{check: func(context.Context) (update.Offer, error) {
		offer := softwareOffer()
		offer.Package = update.Asset{}
		return offer, errors.New("release asset fixture unavailable")
	}}
	uiClick(t, tt, softwareText.check)
	softwareApply(t, tt, queue)
	if s.SoftwareUpdate.Available || s.SoftwareUpdate.Offer.Release.Tag != "" {
		t.Fatal("failed check retained an uninstallable release offer")
	}
	if tt.HasText("Release notes fixture") {
		t.Fatalf("failed check did not replace stale release details: %q", tt.Texts())
	}
	softwareRequireFailure(t, tt, s.SoftwareUpdate)
	uiRequireText(t, tt, softwareText.retry)
}

func TestSoftwareDownloadFailureIsActionableAndRenderedOnce(t *testing.T) {
	s, tt, queue := softwareFixture(t)
	s.SoftwareUpdate = softwareUpdate{Available: true, Manual: true, Dialog: true, Offer: softwareOffer()}
	s.SoftwareClient = softwareClientFixture{download: func(context.Context, update.Offer, string, func(int64, int64)) (update.Download, error) {
		return update.Download{}, errors.New("checksum fixture mismatch")
	}}
	tt.Frame()
	uiClick(t, tt, softwareText.download)
	softwareApply(t, tt, queue)
	if !s.SoftwareUpdate.Available || s.SoftwareUpdate.Downloaded.Package != "" {
		t.Fatal("download failure discarded the release or invented a package")
	}
	softwareRequireFailure(t, tt, s.SoftwareUpdate)
	if !strings.Contains(s.SoftwareUpdate.Error, "checksum fixture mismatch") {
		t.Fatalf("download failure omitted its cause: %q", s.SoftwareUpdate.Error)
	}
	uiRequireText(t, tt, softwareText.retry)
}

func TestSoftwareCancellationClearsStaleFailure(t *testing.T) {
	s, tt, queue := softwareFixture(t)
	s.SoftwareUpdate = softwareUpdate{Available: true, Manual: true, Dialog: true, Offer: softwareOffer()}
	s.SoftwareClient = softwareClientFixture{download: func(context.Context, update.Offer, string, func(int64, int64)) (update.Download, error) {
		return update.Download{}, errors.New("offline fixture")
	}}
	tt.Frame()
	uiClick(t, tt, softwareText.download)
	softwareApply(t, tt, queue)
	softwareRequireFailure(t, tt, s.SoftwareUpdate)

	release := make(chan struct{})
	s.SoftwareClient = softwareClientFixture{download: func(context.Context, update.Offer, string, func(int64, int64)) (update.Download, error) {
		<-release
		return update.Download{}, context.Canceled
	}}
	uiClick(t, tt, softwareText.retry)
	if s.SoftwareUpdate.Error != "" || s.SoftwareUpdate.Status != softwareText.downloading {
		t.Fatalf("retry retained a stale failure: status=%q error=%q", s.SoftwareUpdate.Status, s.SoftwareUpdate.Error)
	}
	uiClick(t, tt, softwareText.cancel)
	close(release)
	softwareApply(t, tt, queue)
	if s.SoftwareUpdate.Busy || s.SoftwareUpdate.Error != "" || s.SoftwareUpdate.Status != softwareText.cancelled {
		t.Fatalf("cancellation retained stale failure: busy=%v status=%q error=%q", s.SoftwareUpdate.Busy, s.SoftwareUpdate.Status, s.SoftwareUpdate.Error)
	}
}

func TestSoftwareQuitVetoPreservesVerifiedDownload(t *testing.T) {
	s, tt, queue := softwareFixture(t)
	installer := &softwareInstallerFixture{}
	dir := filepath.Join(s.Dir, "verified-download")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "update.zip"), []byte("verified fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	shows := 0
	s.ShowWindow = func() { shows++ }
	s.SoftwareUpdate = softwareUpdate{
		Available:  true,
		Installing: true,
		Installer:  installer,
		Offer:      softwareOffer(),
		Downloaded: update.Download{Directory: dir, Package: filepath.Join(dir, "update.zip")},
	}
	s.restoreAfterQuitVeto()
	softwareApply(t, tt, queue)
	if installer.aborts.Load() != 1 || s.SoftwareUpdate.Installing || shows != 1 {
		t.Fatalf("quit veto did not cancel replacement and restore the window: abort=%d installing=%v shows=%d", installer.aborts.Load(), s.SoftwareUpdate.Installing, shows)
	}
	if s.SoftwareUpdate.Downloaded.Directory != dir {
		t.Fatal("quit veto discarded the verified download")
	}
	if _, err := os.Stat(filepath.Join(dir, "update.zip")); err != nil {
		t.Fatalf("verified download was deleted: %v", err)
	}
}

func TestSoftwareOfficialPackageOpensOnlyAfterExplicitClick(t *testing.T) {
	s, tt, _ := softwareFixture(t)
	path := filepath.Join(s.Dir, "official-installer.fixture")
	os.WriteFile(path, []byte("harmless installer fixture"), 0600)
	digest, _ := update.Digest(path)
	s.SoftwareUpdate.Downloaded = update.Download{Package: path, PackageSHA256: digest}
	opened := 0
	s.OpenSoftwareFile = func(actual string) error {
		if actual != path {
			t.Fatal("opened an unexpected package")
		}
		opened++
		return nil
	}
	tt.Frame()
	if opened != 0 {
		t.Fatal("installer opened automatically")
	}
	uiClick(t, tt, softwareText.manualInstall)
	if opened != 1 || s.SoftwareUpdate.Status != softwareText.manualDetail {
		t.Fatal("explicit official package click did not open verified file")
	}
	os.WriteFile(path, []byte("changed after download"), 0600)
	uiClick(t, tt, softwareText.manualInstall)
	if opened != 1 || s.SoftwareUpdate.Error == "" {
		t.Fatal("opened tampered downloaded installer")
	}
}

func TestSoftwareSettingsPageManualCheckAtMinimumWindowSize(t *testing.T) {
	oldVersion := version
	version = "0.1.2"
	defer func() { version = oldVersion }()
	s := uiFixture(t, "settings")
	ctx, cancel := context.WithCancel(context.Background())
	s.Ctx = ctx
	queue := make(chan func(), 8)
	s.Update = func(fn func()) { queue <- fn }
	s.SoftwareExecutable = func() (string, error) { return filepath.Join(s.Dir, "Dipper.exe"), nil }
	var checks atomic.Int32
	s.SoftwareClient = softwareClientFixture{check: func(context.Context) (update.Offer, error) {
		checks.Add(1)
		return softwareOffer(), nil
	}}
	tt := ui.NewTester(s.view, 1040, 720)
	t.Cleanup(func() {
		s.abortSoftwareUpdate()
		cancel()
	})
	uiRequireText(t, tt, softwareText.title, softwareText.current+"："+version)
	uiScrollTo(t, tt, softwareText.check)
	uiClick(t, tt, softwareText.check)
	if !s.SoftwareUpdate.Dialog || !s.SoftwareUpdate.Busy {
		t.Fatal("manual check on the full settings page did not open its pending dialog")
	}
	softwareApply(t, tt, queue)
	if checks.Load() != 1 || !s.SoftwareUpdate.Available || s.SoftwareUpdate.Offer.Release.Tag != "v9.0.0" {
		t.Fatalf("full settings page check did not apply the release offer: calls=%d available=%v offer=%+v", checks.Load(), s.SoftwareUpdate.Available, s.SoftwareUpdate.Offer)
	}
	uiRequireText(t, tt, "Release notes fixture", softwareText.download, softwareText.available+version+" → v9.0.0")
}
