package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
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

func softwareModeFixture(t *testing.T, mode string) (*State, *ui.Tester, chan func()) {
	t.Helper()
	s, tt, queue := softwareAutoFixture(t)
	if mode == update.ModeMacOS {
		s.Platform = "darwin"
	} else if mode != update.ModeWindows {
		s.Platform = "linux"
	}
	s.SoftwareInstallationOffer = func(ctx context.Context, offer update.Offer, target, platform string) (update.Offer, error) {
		if target != filepath.Join(s.Dir, "Dipper.exe") || platform != s.Platform {
			return update.Offer{}, fmt.Errorf("lost installation context")
		}
		if err := ctx.Err(); err != nil {
			return update.Offer{}, err
		}
		return offer, nil
	}
	return s, tt, queue
}

func softwareModeOffer(mode string) update.Offer {
	offer := softwareOffer()
	platform, suffix := "windows", ".zip"
	switch mode {
	case update.ModeMacOS:
		platform, suffix = "darwin", ".tar.gz"
	case update.ModeLinux:
		platform, suffix = "linux", ".tar.gz"
	case update.ModeDebian:
		platform, suffix = "linux", ".deb"
	}
	name := "dipper_9.0.0_" + platform + "_amd64" + suffix
	offer.Mode, offer.Portable = mode, mode == update.ModeWindows
	offer.Package = update.Asset{
		Name: name, Size: 100,
		URL: "https://github.com/steampixel007/Dipper/releases/download/v9.0.0/" + name,
	}
	offer.Release.Assets = []update.Asset{offer.Package}
	return offer
}

func softwareVerifiedModeDownload(ctx context.Context, offer update.Offer, root string, progress func(int64, int64)) (update.Download, error) {
	if softwareOfferMode(offer) == update.ModeWindows {
		download, err := softwareVerifiedDownload(ctx, offer, root, progress)
		download.Mode = offer.Mode
		return download, err
	}
	directory, err := os.MkdirTemp(root, "dipper-update-download-")
	if err != nil {
		return update.Download{}, err
	}
	download := update.Download{Directory: directory, Package: filepath.Join(directory, offer.Package.Name), Mode: offer.Mode}
	files := map[string]string{"dipper": "verified Linux executable fixture"}
	switch offer.Mode {
	case update.ModeDebian:
		files = nil
	case update.ModeMacOS:
		download.Bundle = filepath.Join(directory, "Dipper.app")
		files = map[string]string{
			"Dipper.app/Contents/MacOS/Dipper": "verified macOS executable fixture",
			"Dipper.app/Contents/Info.plist":   "verified application metadata fixture",
			"Dipper.app/Contents/Resources/ui": "verified application resource fixture",
		}
	}
	var archive bytes.Buffer
	compressor := gzip.NewWriter(&archive)
	writer := tar.NewWriter(compressor)
	for name, body := range files {
		target := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return update.Download{}, err
		}
		if err := os.WriteFile(target, []byte(body), 0700); err != nil {
			return update.Download{}, err
		}
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0700, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			return update.Download{}, err
		}
		if _, err := writer.Write([]byte(body)); err != nil {
			return update.Download{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return update.Download{}, err
	}
	if err := compressor.Close(); err != nil {
		return update.Download{}, err
	}
	body := archive.Bytes()
	if offer.Mode == update.ModeDebian {
		body = []byte("!<arch>\nverified Debian package fixture")
	} else if offer.Mode == update.ModeMacOS {
		download.Executable = filepath.Join(download.Bundle, "Contents", "MacOS", "Dipper")
		download.BundleSHA256, err = update.BundleDigest(download.Bundle)
		if err != nil {
			return update.Download{}, err
		}
	} else {
		download.Executable = filepath.Join(directory, "dipper")
	}
	if err := os.WriteFile(download.Package, body, 0600); err != nil {
		return update.Download{}, err
	}
	download.PackageSHA256, err = update.Digest(download.Package)
	if err == nil && download.Executable != "" {
		download.SHA256, err = update.Digest(download.Executable)
	}
	return download, err
}

func TestSoftwarePendingModesRejectTampering(t *testing.T) {
	for _, mode := range []string{update.ModeWindows, update.ModeMacOS, update.ModeLinux, update.ModeDebian} {
		t.Run(mode, func(t *testing.T) {
			for _, payload := range []string{"package", "executable", "bundle"} {
				t.Run(payload, func(t *testing.T) {
					if payload == "executable" && mode == update.ModeDebian || payload == "bundle" && mode != update.ModeMacOS {
						return
					}
					s, _, _ := softwareAutoFixture(t)
					if mode == update.ModeMacOS {
						s.Platform = "darwin"
					} else if mode != update.ModeWindows {
						s.Platform = "linux"
					}
					root := filepath.Join(s.ConfigDir, "updates")
					if err := os.MkdirAll(root, 0700); err != nil {
						t.Fatal(err)
					}
					offer := softwareModeOffer(mode)
					download, err := softwareVerifiedModeDownload(s.Ctx, offer, root, nil)
					if err != nil {
						t.Fatal(err)
					}
					pending := &softwarePending{Offer: offer, Download: download}
					if err := s.validateSoftwarePending(pending); err != nil {
						t.Fatalf("untampered %s payload was rejected: %v", mode, err)
					}
					target := download.Package
					if payload == "executable" {
						target = download.Executable
					} else if payload == "bundle" {
						target = filepath.Join(download.Bundle, "Contents", "Resources", "ui")
					}
					if err := os.WriteFile(target, []byte("tampered payload"), 0700); err != nil {
						t.Fatal(err)
					}
					if err := s.validateSoftwarePending(pending); err == nil {
						t.Fatalf("tampered %s %s was accepted", mode, payload)
					}
				})
			}
		})
	}
}

func TestSoftwareAllModesBackgroundPersistRestart(t *testing.T) {
	for _, mode := range []string{update.ModeWindows, update.ModeMacOS, update.ModeLinux, update.ModeDebian} {
		t.Run(mode, func(t *testing.T) {
			s, tt, queue := softwareModeFixture(t, mode)
			var checks, downloads atomic.Int32
			s.SoftwareClient = softwareClientFixture{
				check: func(context.Context) (update.Offer, error) {
					checks.Add(1)
					return softwareModeOffer(mode), nil
				},
				download: func(ctx context.Context, offer update.Offer, root string, progress func(int64, int64)) (update.Download, error) {
					downloads.Add(1)
					return softwareVerifiedModeDownload(ctx, offer, root, progress)
				},
			}
			s.startAutomaticSoftwareCheck()
			softwareApply(t, tt, queue)
			if !s.SoftwareUpdate.Busy || s.SoftwareUpdate.Dialog {
				t.Fatal("supported release did not start a nonmodal background download")
			}
			softwareApply(t, tt, queue)
			pending := s.SoftwarePending
			if checks.Load() != 1 || downloads.Load() != 1 || pending == nil || !s.softwareRestartReady() ||
				s.SoftwareUpdate.Busy || s.SoftwareUpdate.Dialog {
				t.Fatalf("background download did not persist a restartable %s update: %+v", mode, s.SoftwareUpdate)
			}
			if mode == update.ModeDebian && pending.Download.Executable != "" {
				t.Fatal("DEB fixture invented an extracted executable")
			}
			s.SoftwareAuto, s.SoftwareAutoDownload = false, false
			if err := s.savePreferences(); err != nil {
				t.Fatal(err)
			}
			restarted, rt, results := softwareModeFixture(t, mode)
			restarted.ConfigDir = s.ConfigDir
			restarted.Source = source.New("http://127.0.0.1")
			if err := restarted.loadPreferences(); err != nil {
				t.Fatal(err)
			}
			restarted.startAutomaticSoftwareCheck()
			softwareApply(t, rt, results)
			if restarted.SoftwareAuto || restarted.SoftwareAutoDownload || restarted.SoftwareUpdate.Downloaded != pending.Download ||
				!restarted.softwareRestartReady() || restarted.SoftwareUpdate.Busy || restarted.SoftwareUpdate.Dialog {
				t.Fatalf("startup lost pending payload or changed saved preferences: %+v", restarted.SoftwareUpdate)
			}
			installer := &softwareInstallerFixture{}
			var prepares atomic.Int32
			quits, opens := 0, 0
			restarted.OpenSoftwareFile = func(string) error { opens++; return nil }
			restarted.PrepareSoftware = func(_ context.Context, _ string, downloaded update.Download, _ []string) (softwareInstaller, error) {
				prepares.Add(1)
				if downloaded != pending.Download {
					return nil, fmt.Errorf("restart dispatch changed the persisted payload")
				}
				return installer, nil
			}
			restarted.Quit = func() {
				if restarted.prepareAppQuit() {
					quits++
				}
			}
			uiRequireText(t, rt, "重启更新")
			uiClick(t, rt, "新版本 v9.0.0 已下载，点击重启更新")
			softwareApply(t, rt, results)
			if prepares.Load() != 1 || quits != 1 || installer.commits.Load() != 1 || opens != 0 || !restarted.SoftwareUpdate.Committed {
				t.Fatalf("restart did not reach graceful commit: prepares=%d quits=%d commits=%d opens=%d",
					prepares.Load(), quits, installer.commits.Load(), opens)
			}
		})
	}
}

func TestSoftwareAllModesManualDownloadWithoutOptIn(t *testing.T) {
	for _, mode := range []string{update.ModeWindows, update.ModeMacOS, update.ModeLinux, update.ModeDebian} {
		t.Run(mode, func(t *testing.T) {
			s, tt, queue := softwareModeFixture(t, mode)
			s.SoftwareAutoDownload = false
			var downloads atomic.Int32
			s.SoftwareClient = softwareClientFixture{
				check: func(context.Context) (update.Offer, error) { return softwareModeOffer(mode), nil },
				download: func(ctx context.Context, offer update.Offer, root string, progress func(int64, int64)) (update.Download, error) {
					downloads.Add(1)
					return softwareVerifiedModeDownload(ctx, offer, root, progress)
				},
			}
			s.startAutomaticSoftwareCheck()
			softwareApply(t, tt, queue)
			if downloads.Load() != 0 || s.SoftwareUpdate.Busy || s.SoftwarePending != nil {
				t.Fatal("disabled automatic download still fetched a package")
			}
			uiClick(t, tt, softwareText.download)
			softwareApply(t, tt, queue)
			if downloads.Load() != 1 || s.SoftwareAutoDownload || !s.softwareRestartReady() || s.SoftwarePending == nil {
				t.Fatal("manual download did not become restartable while preserving opt-out")
			}
			uiRequireText(t, tt, "重启更新")
			detail := softwareText.archiveDetail
			if mode == update.ModeDebian {
				detail = softwareText.debianDetail
			}
			s.SoftwareUpdate.Dialog = true
			tt.Frame()
			uiRequireText(t, tt, detail)
		})
	}
}

func TestSoftwareAllModesCancelStaleDownloadAndPreparation(t *testing.T) {
	for _, mode := range []string{update.ModeWindows, update.ModeMacOS, update.ModeLinux, update.ModeDebian} {
		t.Run(mode, func(t *testing.T) {
			s, tt, queue := softwareModeFixture(t, mode)
			entered, release := make(chan struct{}), make(chan struct{})
			var stale update.Download
			s.SoftwareClient = softwareClientFixture{
				check: func(context.Context) (update.Offer, error) { return softwareModeOffer(mode), nil },
				download: func(ctx context.Context, offer update.Offer, root string, progress func(int64, int64)) (update.Download, error) {
					var err error
					stale, err = softwareVerifiedModeDownload(ctx, offer, root, progress)
					close(entered)
					<-release
					return stale, err
				},
			}
			s.startAutomaticSoftwareCheck()
			softwareApply(t, tt, queue)
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("download did not start")
			}
			s.cancelSoftwareDownload()
			close(release)
			softwareApply(t, tt, queue)
			if s.SoftwarePending != nil || s.SoftwareUpdate.Downloaded.Package != "" || s.SoftwareUpdate.Busy {
				t.Fatal("cancelled download published or persisted stale bytes")
			}
			if _, err := os.Stat(stale.Directory); !os.IsNotExist(err) {
				t.Fatalf("cancelled managed download was not reclaimed: %v", err)
			}
			s.SoftwareClient = softwareClientFixture{download: softwareVerifiedModeDownload}
			s.startSoftwareDownload(false)
			softwareApply(t, tt, queue)
			pending := s.SoftwarePending
			if pending == nil {
				t.Fatal("retry did not persist a verified download")
			}
			installer := &softwareInstallerFixture{}
			preparing, finish := make(chan struct{}), make(chan struct{})
			s.PrepareSoftware = func(context.Context, string, update.Download, []string) (softwareInstaller, error) {
				close(preparing)
				<-finish
				return installer, nil
			}
			quits := 0
			s.Quit = func() { quits++ }
			uiClick(t, tt, "新版本 v9.0.0 已下载，点击重启更新")
			select {
			case <-preparing:
			case <-time.After(5 * time.Second):
				t.Fatal("restart preparation did not start")
			}
			s.cancelSoftwareDownload()
			close(finish)
			softwareApply(t, tt, queue)
			if installer.aborts.Load() == 0 || installer.commits.Load() != 0 || quits != 0 ||
				s.SoftwarePending != pending || s.SoftwareUpdate.Busy || s.SoftwareUpdate.Installing || !s.softwareRestartReady() {
				t.Fatal("cancelled preparation committed, quit, or lost its retryable payload")
			}
			pref, err := readPreferences(s.ConfigDir)
			if err != nil || pref.SoftwarePending == nil || pref.SoftwarePending.Download != pending.Download {
				t.Fatalf("cancelled preparation lost the saved update: %v", err)
			}
		})
	}
}

func TestSoftwarePendingRejectsPathEscapeAndLinks(t *testing.T) {
	for _, scenario := range []string{"escape", "package-link", "executable-link", "directory-link", "root-link"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, _ := softwareAutoFixture(t)
			root := filepath.Join(s.ConfigDir, "updates")
			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			offer := softwareModeOffer(update.ModeWindows)
			download, err := softwareVerifiedModeDownload(s.Ctx, offer, root, nil)
			if err != nil {
				t.Fatal(err)
			}
			pending := &softwarePending{Offer: offer, Download: download}
			outside := t.TempDir()
			external := filepath.Join(outside, "payload")
			switch scenario {
			case "escape":
				body, err := os.ReadFile(download.Executable)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(external, body, 0700); err != nil {
					t.Fatal(err)
				}
				pending.Download.Executable = external
			case "package-link", "executable-link":
				path := download.Executable
				if scenario == "package-link" {
					path = download.Package
				}
				if err := os.Rename(path, external); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(external, path); err != nil {
					t.Skipf("host does not permit symbolic links: %v", err)
				}
			case "directory-link", "root-link":
				path := download.Directory
				if scenario == "root-link" {
					path = root
				}
				if err := os.Rename(path, external); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(external, path); err != nil {
					t.Skipf("host does not permit symbolic links: %v", err)
				}
			}
			if err := s.validateSoftwarePending(pending); err == nil {
				t.Fatal("pending update accepted a redirected or escaped payload")
			}
			s.removeSoftwarePending(pending)
			if _, err := os.Stat(external); err != nil {
				t.Fatalf("rejecting untrusted pending paths removed external data: %v", err)
			}
		})
	}
}
