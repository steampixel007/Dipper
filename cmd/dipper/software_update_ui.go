package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/update"
)

type softwareClient interface {
	Check(context.Context, string, string, string) (update.Offer, error)
	Download(context.Context, update.Offer, string, func(int64, int64)) (update.Download, error)
}

type softwareInstaller interface {
	Commit() error
	Abort()
}

type softwareUpdate struct {
	Busy         bool
	Available    bool
	Development  bool
	Manual       bool
	Installing   bool
	Committed    bool
	Offer        update.Offer
	Downloaded   update.Download
	Installer    softwareInstaller
	Status       string
	Error        string
	StartupError string
	Progress     int64
	Total        int64
	Dialog       bool
	revision     uint64
	cancel       context.CancelFunc
}

var softwareText = struct {
	title, current, automatic, automaticDetail, check, checking, latest, available    string
	failed, download, downloading, cancel, cancelled, ready, install, installing      string
	retry, notes, installLater, manualInstall, manualDetail, disabled                 string
	development, unavailable, preferenceError, networkAdvice, downloadError           string
	openError, installError, unchanged, noNotes, portableDetail, startupError         string
	preparing, startBlocked, handshakeRejected, manualWindows, manualMac, manualLinux string
	autoDownload, autoDownloadDetail, downloadedRestart, downloadPreferenceError      string
}{
	title: "软件更新", current: "当前版本", automatic: "启动后自动检查",
	automaticDetail: "每天最多检查一次，只使用稳定版本；自动检查失败不会打扰你。",
	check:           "立即检查", checking: "正在检查…", latest: "已是最新版本。",
	available: "发现新版本 ", failed: "检查失败", download: "下载更新",
	downloading: "正在下载并校验…", cancel: "取消下载", cancelled: "已取消，旧版本未改动。",
	ready: "下载完成，校验通过。安装前不会改动当前版本。", install: "安装并重启",
	installing: "正在退出并安装更新…", retry: "重试", notes: "更新说明",
	installLater: "稍后", manualInstall: "打开安装包",
	manualDetail: "下载已校验。请按安装包提示完成更新；Dipper 不会静默执行安装器。",
	disabled:     "开发版不自动检查；手动检查可选择切换到官方稳定版，无法比较新旧。",
	development:  "开发版", unavailable: "更新检查暂不可用。",
	preferenceError: "自动检查设置保存失败：", networkAdvice: "。请检查网络和代理设置后重试。",
	downloadError: "下载或校验失败：", openError: "无法打开安装包：", installError: "无法开始安装：",
	unchanged:               "。请检查目录写入权限或关闭占用程序后重试，旧版本未被改动。",
	noNotes:                 "此版本没有附加说明。",
	portableDetail:          "确认后会退出并重启，只替换 Dipper.exe；追番、下载和设置保留。启动失败时可能保留旧程序备份，但不会回退用户数据。请先阅读版本说明。",
	startupError:            "软件更新启动确认失败，请保留旧程序备份并检查日志。",
	preparing:               "正在准备更新助手…",
	startBlocked:            "软件更新仍在进行，请稍后重新打开 Dipper：",
	handshakeRejected:       "软件更新交接验证失败：",
	manualWindows:           "请保存工作并退出 Dipper，再按官方安装包提示完成安装。不会自动运行安装器。",
	manualMac:               "请退出 Dipper，打开磁盘映像或解压归档，将 Dipper.app 放到应用程序目录后重新打开。",
	manualLinux:             "请退出 Dipper，使用系统包管理器安装 DEB，或解压归档替换应用程序，再重新打开。",
	autoDownload:            "自动下载更新",
	autoDownloadDetail:      "仅 Windows 便携版：发现新版本后后台下载并校验，完成后提示重启。",
	downloadedRestart:       "新版本 %s 已下载，点击重启更新",
	downloadPreferenceError: "自动下载设置保存失败：",
}

func updateVersionLabel(value string) string {
	if update.Development(value) {
		return softwareText.development
	}
	return value
}

func (s *State) softwareAutomatic() bool {
	return s.SoftwareAuto
}

func (s *State) setSoftwareAutomatic(value bool) {
	previous := s.SoftwareAuto
	s.SoftwareAuto = value
	if err := s.savePreferences(); err != nil {
		s.SoftwareAuto = previous
		s.SaveError = softwareText.preferenceError + err.Error()
	} else {
		s.SaveError = ""
	}
}

func (s *State) startAutomaticSoftwareCheck() {
	if s.SoftwareUpdate.Busy {
		return
	}
	if s.SoftwareUpdate.Downloaded.Executable != "" {
		return
	}
	if s.SoftwarePending != nil {
		s.restoreSoftwarePending()
		return
	}
	if !s.softwareAutomatic() || update.Development(version) || s.SoftwareClient == nil {
		return
	}
	now := time.Now()
	if !s.SoftwareCheckedAt.IsZero() {
		elapsed := now.Sub(s.SoftwareCheckedAt)
		if elapsed >= 0 && elapsed < 24*time.Hour {
			return
		}
	}
	previous := s.SoftwareCheckedAt
	s.SoftwareCheckedAt = now
	if err := s.savePreferences(); err != nil {
		s.SoftwareCheckedAt = previous
		return
	}
	s.checkSoftware(false)
}

func (s *State) checkSoftware(manual bool) {
	if s.SoftwareUpdate.Busy {
		return
	}
	if s.SoftwareUpdate.Downloaded.Executable != "" && s.SoftwarePending != nil {
		if s.validateSoftwarePending(s.SoftwarePending) == nil {
			if manual {
				s.SoftwareUpdate.Dialog = true
			}
			return
		}
		s.discardInvalidSoftwarePending(s.SoftwarePending)
	}
	if s.SoftwareClient == nil {
		if manual {
			s.SoftwareUpdate.Available = false
			s.SoftwareUpdate.Offer = update.Offer{}
			s.setSoftwareError(softwareText.unavailable)
		}
		return
	}
	s.discardSoftwareDownload()
	startupError := s.SoftwareUpdate.StartupError
	revision := s.SoftwareUpdate.revision
	ctx, cancel := context.WithTimeout(s.Ctx, 2*time.Minute)
	s.SoftwareUpdate = softwareUpdate{Busy: true, Manual: manual, cancel: cancel, Status: softwareText.checking, revision: revision, StartupError: startupError}
	if !manual {
		s.SoftwareUpdate.Status = ""
	}
	if manual {
		s.SoftwareUpdate.Dialog = true
	}
	client := s.SoftwareClient
	executable := s.SoftwareExecutable
	go func() {
		offer, err := client.Check(ctx, version, runtime.GOOS, runtime.GOARCH)
		if err == nil && offer.Portable && executable != nil {
			path, pathErr := executable()
			if pathErr != nil {
				err = pathErr
			} else {
				offer, err = update.WindowsInstallationOffer(offer, path)
			}
		}
		s.Update(func() {
			if revision != s.SoftwareUpdate.revision || s.Ctx.Err() != nil {
				return
			}
			cancel()
			s.SoftwareUpdate.Busy = false
			s.SoftwareUpdate.cancel = nil
			s.SoftwareUpdate.Offer = offer
			if err != nil {
				s.softwareCheckFailed(manual, err)
				return
			}
			s.SoftwareUpdate.Development = offer.Development
			s.SoftwareUpdate.Available = offer.Available
			switch {
			case offer.Development:
				s.setSoftwareStatus(softwareText.disabled)
			case offer.Available:
				s.setSoftwareStatus(softwareText.available + offer.Release.Tag)
			case manual:
				s.setSoftwareStatus(softwareText.latest)
			default:
				s.setSoftwareStatus("")
			}
			if !manual && s.SoftwareAutoDownload && s.softwarePortable(offer) && offer.Available {
				s.startSoftwareDownload(false)
			}
		})
	}()
}

func (s *State) setSoftwareStatus(status string) {
	s.SoftwareUpdate.Status = status
	s.SoftwareUpdate.Error = ""
}

func (s *State) setSoftwareError(message string) {
	s.SoftwareUpdate.Status = ""
	s.SoftwareUpdate.Error = message
}

func (s *State) softwareCheckFailed(manual bool, err error) {
	s.SoftwareUpdate.Available = false
	s.SoftwareUpdate.Offer = update.Offer{}
	message := softwareText.failed + "：" + err.Error() + softwareText.networkAdvice
	if manual {
		s.setSoftwareError(message)
	} else {
		s.setSoftwareStatus("")
	}
}

func (s *State) downloadSoftware() {
	s.startSoftwareDownload(true)
}

func (s *State) startSoftwareDownload(manual bool) {
	if !s.SoftwareUpdate.Available || s.SoftwareUpdate.Busy || s.SoftwareClient == nil {
		return
	}
	if s.SoftwarePending != nil && s.SoftwareUpdate.Downloaded.Executable != "" {
		if s.validateSoftwarePending(s.SoftwarePending) == nil {
			return
		}
		s.discardInvalidSoftwarePending(s.SoftwarePending)
	}
	offer := s.SoftwareUpdate.Offer
	s.discardSoftwareDownload()
	ctx, cancel := context.WithTimeout(s.Ctx, 45*time.Minute)
	s.SoftwareUpdate.Busy = true
	s.SoftwareUpdate.Manual = manual
	s.SoftwareUpdate.cancel = cancel
	s.setSoftwareStatus(softwareText.downloading)
	s.SoftwareUpdate.Progress = 0
	s.SoftwareUpdate.Total = offer.Package.Size
	s.SoftwareUpdate.revision++
	revision := s.SoftwareUpdate.revision
	client := s.SoftwareClient
	persistent := s.softwarePortable(offer)
	go func() {
		lastReport := time.Time{}
		directory := os.TempDir()
		if persistent {
			var err error
			directory, err = s.softwareUpdateRoot()
			if err != nil {
				s.Update(func() { s.softwareDownloadFailed(revision, err) })
				return
			}
		}
		if err := os.MkdirAll(directory, 0700); err != nil {
			s.Update(func() { s.softwareDownloadFailed(revision, err) })
			return
		}
		download, err := client.Download(ctx, offer, directory, func(done, total int64) {
			if time.Since(lastReport) < 200*time.Millisecond {
				return
			}
			lastReport = time.Now()
			s.Update(func() {
				if revision != s.SoftwareUpdate.revision {
					return
				}
				s.SoftwareUpdate.Progress, s.SoftwareUpdate.Total = done, total
			})
		})
		if err == nil && persistent {
			err = s.validateSoftwarePending(&softwarePending{Offer: offer, Download: download})
		}
		if err != nil && persistent {
			s.removeSoftwarePending(&softwarePending{Download: download})
			download = update.Download{}
		}
		if !persistent && download.Directory != "" {
			context.AfterFunc(s.Ctx, func() { os.RemoveAll(download.Directory) })
		}
		if ctx.Err() != nil {
			os.RemoveAll(download.Directory)
			download = update.Download{}
			err = ctx.Err()
		}
		s.Update(func() {
			if revision != s.SoftwareUpdate.revision || s.Ctx.Err() != nil {
				os.RemoveAll(download.Directory)
				return
			}
			cancel()
			s.SoftwareUpdate.Busy = false
			s.SoftwareUpdate.cancel = nil
			if err != nil {
				s.softwareDownloadFailed(revision, err)
				return
			}
			if persistent {
				pending := &softwarePending{Offer: offer, Download: download}
				s.SoftwarePending = pending
				if err := s.savePreferences(); err != nil {
					s.SoftwarePending = nil
					s.removeSoftwarePending(pending)
					s.softwareDownloadFailed(revision, err)
					return
				}
			}
			s.SoftwareUpdate.Downloaded = download
			s.setSoftwareStatus(softwareText.ready)
			s.SoftwareUpdate.Progress, s.SoftwareUpdate.Total = offer.Package.Size, offer.Package.Size
		})
	}()
}

func (s *State) softwareDownloadFailed(revision uint64, err error) {
	if revision != s.SoftwareUpdate.revision || s.Ctx.Err() != nil {
		return
	}
	s.SoftwareUpdate.Busy = false
	if s.SoftwareUpdate.cancel != nil {
		s.SoftwareUpdate.cancel()
		s.SoftwareUpdate.cancel = nil
	}
	if errors.Is(err, context.Canceled) {
		s.setSoftwareStatus(softwareText.cancelled)
		return
	}
	s.setSoftwareError(softwareText.downloadError + err.Error() + softwareText.unchanged)
}

func (s *State) cancelSoftwareOperation() {
	s.SoftwareUpdate.revision++
	if s.SoftwareUpdate.cancel != nil {
		s.SoftwareUpdate.cancel()
		s.SoftwareUpdate.cancel = nil
	}
	if s.SoftwareUpdate.Installer != nil && !s.SoftwareUpdate.Committed {
		s.SoftwareUpdate.Installer.Abort()
		s.SoftwareUpdate.Installer = nil
	}
}

func (s *State) discardSoftwareDownload() {
	s.cancelSoftwareOperation()
	if s.SoftwareUpdate.Downloaded.Directory != "" {
		os.RemoveAll(s.SoftwareUpdate.Downloaded.Directory)
		s.SoftwareUpdate.Downloaded = update.Download{}
	}
}

func (s *State) cancelSoftwareDownload() {
	if !s.SoftwareUpdate.Busy {
		return
	}
	if s.SoftwareUpdate.Installing {
		s.abortSoftwareUpdate()
		return
	}
	s.discardSoftwareDownload()
	s.SoftwareUpdate.Busy = false
	s.setSoftwareStatus(softwareText.cancelled)
}

func (s *State) installSoftware() {
	if s.SoftwareUpdate.Busy || s.SoftwareUpdate.Installing || s.Busy {
		return
	}
	switch {
	case s.SoftwareUpdate.Downloaded.Executable != "":
		if !s.saveBeforeClose() {
			s.restoreAfterQuitVeto()
			return
		}
		executable, err := s.SoftwareExecutable()
		if err != nil {
			s.setSoftwareError(softwareText.installError + err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(s.Ctx, 20*time.Second)
		s.SoftwareUpdate.revision++
		revision := s.SoftwareUpdate.revision
		s.SoftwareUpdate.cancel = cancel
		s.SoftwareUpdate.Busy = true
		s.SoftwareUpdate.Installing = true
		s.setSoftwareStatus(softwareText.preparing)
		downloaded := s.SoftwareUpdate.Downloaded
		pending := s.SoftwarePending
		prepare := s.PrepareSoftware
		if prepare == nil {
			prepare = func(ctx context.Context, target string, downloaded update.Download, args []string) (softwareInstaller, error) {
				return update.PreparePortable(ctx, target, downloaded, args)
			}
		}
		args := append([]string(nil), os.Args[1:]...)
		go func() {
			installer, err := prepare(ctx, executable, downloaded, args)
			invalidPending := false
			if err != nil && ctx.Err() == nil && pending != nil && downloaded.Executable != "" {
				invalidPending = update.VerifyPortableDownload(downloaded) != nil
			}
			if installer != nil {
				context.AfterFunc(s.Ctx, installer.Abort)
			}
			if ctx.Err() != nil {
				if installer != nil {
					installer.Abort()
				}
				installer = nil
				err = ctx.Err()
			}
			s.Update(func() {
				cancel()
				if revision != s.SoftwareUpdate.revision || s.Ctx.Err() != nil {
					if installer != nil {
						installer.Abort()
					}
					return
				}
				s.SoftwareUpdate.cancel = nil
				s.SoftwareUpdate.Busy = false
				if err != nil {
					s.SoftwareUpdate.Installing = false
					s.setSoftwareError(softwareText.installError + err.Error() + softwareText.unchanged)
					if invalidPending {
						s.discardInvalidSoftwarePending(pending)
					}
					return
				}
				s.SoftwareUpdate.Installer = installer
				s.setSoftwareStatus(softwareText.installing)
				if s.Quit != nil {
					s.Quit()
				}
			})
		}()
	case s.SoftwareUpdate.Downloaded.Package != "":
		if s.OpenSoftwareFile == nil {
			return
		}
		if err := update.VerifyDownload(s.SoftwareUpdate.Downloaded); err != nil {
			s.setSoftwareError(softwareText.openError + err.Error())
			return
		}
		if err := s.OpenSoftwareFile(s.SoftwareUpdate.Downloaded.Package); err != nil {
			s.setSoftwareError(softwareText.openError + err.Error())
			return
		}
		s.setSoftwareStatus(softwareText.manualDetail)
	}
}

func (s *State) commitSoftwareUpdate() bool {
	if s.SoftwareUpdate.Installer == nil || !s.SoftwareUpdate.Installing {
		s.abortSoftwareUpdate()
		return true
	}
	if err := s.SoftwareUpdate.Installer.Commit(); err != nil {
		s.SoftwareUpdate.Installer.Abort()
		s.SoftwareUpdate.Installer = nil
		s.SoftwareUpdate.Installing = false
		s.setSoftwareError(softwareText.installError + err.Error() + softwareText.unchanged)
		return false
	}
	s.SoftwareUpdate.Committed = true
	return true
}

func (s *State) abortSoftwareUpdate() {
	if s.SoftwareUpdate.Committed {
		return
	}
	wasBusy := s.SoftwareUpdate.Busy
	wasInstalling := s.SoftwareUpdate.Installing
	s.cancelSoftwareOperation()
	s.SoftwareUpdate.Busy = false
	if wasInstalling {
		s.SoftwareUpdate.Installing = false
	}
	if wasBusy || wasInstalling {
		s.setSoftwareStatus(softwareText.cancelled)
	}
}

func (s *State) softwareNotice(c *ui.Context) {
	if !s.SoftwareUpdate.Available && s.SoftwareUpdate.StartupError == "" {
		return
	}
	message := s.SoftwareUpdate.StartupError
	if s.SoftwareUpdate.Available {
		message = softwareText.available + s.SoftwareUpdate.Offer.Release.Tag
	}
	if message == "" {
		return
	}
	if s.SoftwarePending != nil && s.SoftwareUpdate.Downloaded.Executable != "" {
		ui.Row(c).Wrap().Gap(12).Padding(8, 24).Background(colors(c).blueWash).Children(func() {
			quiet(c, "restart-software-update", fmt.Sprintf(softwareText.downloadedRestart, s.SoftwareUpdate.Offer.Release.Tag), s.installSoftware).
				Disabled(s.Busy || s.SoftwareUpdate.Busy || s.SoftwareUpdate.Installing)
			quiet(c, "open-software-update", softwareText.notes, func() {
				s.Page = "settings"
				s.SoftwareUpdate.Dialog = true
			})
		})
		return
	}
	ui.Row(c).Wrap().Gap(12).Padding(8, 24).Background(colors(c).blueWash).Children(func() {
		ui.Text(c, message).FontSize(textSize(c, textCaption)).TextColor(colors(c).ink).Grow(1).MinWidth(0)
		quiet(c, "open-software-update", softwareText.notes, func() {
			s.Page = "settings"
			s.SoftwareUpdate.Dialog = true
		})
	})
}

func (s *State) softwareUpdateSettings(c *ui.Context) {
	ui.Column(c).Gap(12).Padding(22).Background(colors(c).paper).Border(1, colors(c).line).Radius(10).Children(func() {
		heading(c, softwareText.title)
		ui.Row(c).Wrap().Gap(16).Children(func() {
			ui.Column(c).Grow(1).MinWidth(textSize(c, textBody) * 12).Gap(4).Children(func() {
				ui.Text(c, softwareText.current+"："+updateVersionLabel(version)).FontSize(textSize(c, textBody)).FontWeight(600)
				caption(c, softwareText.automaticDetail)
			})
			automatic := s.SoftwareAuto
			ui.Checkbox(c.Key("software-auto"), &automatic, softwareText.automatic).OnChange(func() {
				s.setSoftwareAutomatic(automatic)
			}).Disabled(s.Busy || s.SoftwareUpdate.Busy)
			autoDownload := s.SoftwareAutoDownload
			ui.Checkbox(c.Key("software-auto-download"), &autoDownload, softwareText.autoDownload).OnChange(func() {
				s.setSoftwareAutoDownload(autoDownload)
			}).Disabled(s.Busy || s.SoftwareUpdate.Busy)
		})
		caption(c, softwareText.autoDownloadDetail)
		if update.Development(version) {
			caption(c, softwareText.disabled)
		}
		if s.SoftwareUpdate.Status != "" {
			ui.Text(c, s.SoftwareUpdate.Status).FontSize(textSize(c, textCaption)).TextColor(colors(c).ink).LineHeight(1.5)
		}
		if s.SoftwareUpdate.StartupError != "" {
			ui.Text(c, s.SoftwareUpdate.StartupError).FontSize(textSize(c, textCaption)).TextColor(colors(c).errorInk).LineHeight(1.5)
		}
		if s.SoftwareUpdate.Busy && s.SoftwareUpdate.Total > 0 {
			percent := float32(s.SoftwareUpdate.Progress) / float32(s.SoftwareUpdate.Total) * 100
			ui.Text(c, fmt.Sprintf("%.0f%%", percent)).FontSize(textSize(c, textCaption))
		}
		ui.Row(c).Wrap().Gap(controlGap).Children(func() {
			secondary(c, "software-check", softwareText.check, func() { s.checkSoftware(true) }).FontSize(textSize(c, textCaption)).Disabled(s.Busy || s.SoftwareUpdate.Busy)
			switch {
			case s.SoftwareUpdate.Busy:
				quiet(c, "software-cancel", softwareText.cancel, s.cancelSoftwareDownload)
			case s.SoftwareUpdate.Downloaded.Executable != "":
				primary(c, "software-install", softwareText.install, s.installSoftware).FontSize(textSize(c, textCaption)).Disabled(s.Busy || s.SoftwareUpdate.Installing)
			case s.SoftwareUpdate.Downloaded.Package != "":
				primary(c, "software-open-installer", softwareText.manualInstall, s.installSoftware).FontSize(textSize(c, textCaption)).Disabled(s.Busy)
			case s.SoftwareUpdate.Available && !s.SoftwareUpdate.Installing:
				primary(c, "software-download", softwareText.download, s.downloadSoftware).FontSize(textSize(c, textCaption)).Disabled(s.Busy)
			}
			if s.SoftwareUpdate.Offer.Release.Tag != "" {
				quiet(c, "software-notes", softwareText.notes, func() { s.SoftwareUpdate.Dialog = true })
			}
		})
		if s.SoftwareUpdate.Error != "" {
			ui.Text(c, s.SoftwareUpdate.Error).FontSize(textSize(c, textCaption)).TextColor(colors(c).errorInk).LineHeight(1.5)
		}
	})
}

func (s *State) softwareUpdateDialog(c *ui.Context) {
	if !s.SoftwareUpdate.Dialog {
		return
	}
	ui.DialogBase(c, &s.SoftwareUpdate.Dialog, func(backdrop, panel ui.Element) {
		backdrop.Background(colors(c).scrim)
		dialogContentWithFooter(c, panel, "software-update-content", 620, func() {
			heading(c, softwareText.title)
			offer := s.SoftwareUpdate.Offer
			if offer.Release.Tag != "" {
				ui.Text(c, offer.Release.Tag).FontSize(textSize(c, textSection)).FontWeight(700)
				if offer.Available {
					ui.Text(c, softwareText.available+updateVersionLabel(version)+" → "+offer.Release.Tag).FontSize(textSize(c, textBody))
				}
				notes := strings.TrimSpace(offer.Release.Notes)
				if notes == "" {
					notes = softwareText.noNotes
				}
				ui.Text(c, notes).FontSize(textSize(c, textBody)).LineHeight(1.6).Selectable()
			}
			if s.SoftwareUpdate.Status != "" {
				ui.Text(c, s.SoftwareUpdate.Status).FontSize(textSize(c, textBody)).LineHeight(1.6).Selectable()
			}
			if s.SoftwareUpdate.StartupError != "" {
				ui.Text(c, s.SoftwareUpdate.StartupError).FontSize(textSize(c, textCaption)).TextColor(colors(c).errorInk).LineHeight(1.5)
			}
			if s.SoftwareUpdate.Busy && s.SoftwareUpdate.Total > 0 {
				caption(c, fmt.Sprintf("%.0f%%", float64(s.SoftwareUpdate.Progress)*100/float64(s.SoftwareUpdate.Total)))
			}
			if offer.Portable {
				caption(c, softwareText.portableDetail)
			} else if offer.Available {
				detail := softwareText.manualWindows
				if s.Platform == "darwin" {
					detail = softwareText.manualMac
				} else if s.Platform == "linux" {
					detail = softwareText.manualLinux
				}
				caption(c, detail)
			}
			if s.SoftwareUpdate.Error != "" {
				ui.Text(c, s.SoftwareUpdate.Error).FontSize(textSize(c, textCaption)).TextColor(colors(c).errorInk).LineHeight(1.5)
			}
		}, func() {
			quiet(c, "software-close", softwareText.installLater, func() { s.SoftwareUpdate.Dialog = false })
			if s.SoftwareUpdate.Busy {
				quiet(c, "software-dialog-cancel", softwareText.cancel, s.cancelSoftwareDownload)
			}
			if s.SoftwareUpdate.Available && !s.SoftwareUpdate.Busy && s.SoftwareUpdate.Downloaded.Package == "" {
				secondary(c, "software-dialog-download", softwareText.download, s.downloadSoftware).Disabled(s.Busy)
			}
			if s.SoftwareUpdate.Downloaded.Executable != "" {
				primary(c, "software-dialog-install", softwareText.install, s.installSoftware).Disabled(s.Busy || s.SoftwareUpdate.Installing)
			} else if s.SoftwareUpdate.Downloaded.Package != "" {
				primary(c, "software-dialog-open", softwareText.manualInstall, s.installSoftware).Disabled(s.Busy)
			}
			if s.SoftwareUpdate.Error != "" {
				quiet(c, "software-retry", softwareText.retry, func() {
					if s.SoftwareUpdate.Downloaded.Package != "" {
						s.installSoftware()
					} else if s.SoftwareUpdate.Available {
						s.downloadSoftware()
					} else {
						s.checkSoftware(true)
					}
				}).Disabled(s.SoftwareUpdate.Busy)
			}
		})
	})
}
