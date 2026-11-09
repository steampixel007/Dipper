package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/steampixel007/Dipper/internal/update"
)

type softwarePending struct {
	Offer    update.Offer
	Download update.Download
}

func (s *State) softwareInstallationOffer() func(context.Context, update.Offer, string, string) (update.Offer, error) {
	if s.SoftwareInstallationOffer != nil {
		return s.SoftwareInstallationOffer
	}
	return update.InstallationOffer
}

func (s *State) setSoftwareAutoDownload(value bool) {
	previous := s.SoftwareAutoDownload
	s.SoftwareAutoDownload = value
	if err := s.savePreferences(); err != nil {
		s.SoftwareAutoDownload = previous
		s.SaveError = softwareText.downloadPreferenceError + err.Error()
	} else {
		s.SaveError = ""
	}
}

func (s *State) softwareSelfUpdating(offer update.Offer) bool {
	if !update.SelfUpdating(offer) {
		return false
	}
	platform := s.Platform
	if platform == "" {
		platform = runtime.GOOS
	}
	switch offer.Mode {
	case update.ModeMacOS:
		return platform == "darwin"
	case update.ModeLinux, update.ModeDebian:
		return platform == "linux"
	default:
		return platform == "windows"
	}
}

func softwareOfferMode(offer update.Offer) string {
	if offer.Mode == "" && offer.Portable {
		return update.ModeWindows
	}
	return offer.Mode
}

func (s *State) softwarePersistent(offer update.Offer) bool {
	return s.softwareSelfUpdating(offer) && !offer.Development && !update.Development(version)
}

func (s *State) softwareRestartReady() bool {
	download := s.SoftwareUpdate.Downloaded
	mode := softwareOfferMode(s.SoftwareUpdate.Offer)
	downloadMode := download.Mode
	if downloadMode == "" && mode == update.ModeWindows {
		downloadMode = update.ModeWindows
	}
	return s.softwareSelfUpdating(s.SoftwareUpdate.Offer) && download.Package != "" &&
		mode == downloadMode &&
		(download.Executable != "" || download.Mode == update.ModeDebian)
}

func softwareCanonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}

func softwarePathEqual(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func (s *State) softwareUpdateRoot() (string, error) {
	config, err := softwareCanonicalPath(s.ConfigDir)
	if err != nil {
		return "", err
	}
	root := filepath.Join(config, "updates")
	if info, err := os.Lstat(root); err == nil {
		resolved, resolveErr := softwareCanonicalPath(root)
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || resolveErr != nil || !softwarePathEqual(root, resolved) {
			return "", fmt.Errorf("update storage is redirected")
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return root, nil
}

func (s *State) softwarePendingDirectory(pending *softwarePending) bool {
	if pending == nil || !filepath.IsAbs(pending.Download.Directory) {
		return false
	}
	root, err := s.softwareUpdateRoot()
	if err != nil {
		return false
	}
	directory, err := softwareCanonicalPath(pending.Download.Directory)
	if err != nil {
		return false
	}
	info, err := os.Lstat(pending.Download.Directory)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 &&
		softwarePathEqual(directory, pending.Download.Directory) &&
		softwarePathEqual(filepath.Dir(directory), root) &&
		strings.HasPrefix(filepath.Base(directory), "dipper-update-download-")
}

func softwarePendingPath(directory, path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("pending payload path is not absolute")
	}
	relative, err := filepath.Rel(directory, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("pending payload is outside the managed download directory")
	}
	current := directory
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("pending payload contains a symbolic link")
		}
	}
	resolved, err := softwareCanonicalPath(path)
	if err != nil {
		return err
	}
	if !softwarePathEqual(path, resolved) {
		return fmt.Errorf("pending payload is redirected")
	}
	return nil
}

func (s *State) validateSoftwarePending(pending *softwarePending) error {
	if pending == nil {
		return fmt.Errorf("pending update is missing")
	}
	comparison, err := update.Compare(pending.Offer.Release.Tag, version)
	if err != nil || comparison <= 0 || !s.softwarePersistent(pending.Offer) || !pending.Offer.Available {
		return fmt.Errorf("pending update is not a newer supported release")
	}
	download := pending.Download
	mode := softwareOfferMode(pending.Offer)
	downloadMode := download.Mode
	if downloadMode == "" && mode == update.ModeWindows {
		downloadMode = update.ModeWindows
	}
	if mode != downloadMode || filepath.Base(download.Package) != pending.Offer.Package.Name {
		return fmt.Errorf("pending payload does not match its offer")
	}
	if !s.softwarePendingDirectory(pending) {
		return fmt.Errorf("pending update is outside the managed download directory")
	}
	if err := softwarePendingPath(download.Directory, download.Package); err != nil {
		return err
	}
	if !softwarePathEqual(filepath.Dir(download.Package), download.Directory) {
		return fmt.Errorf("pending update is outside the managed download directory")
	}
	if mode != update.ModeDebian {
		if err := softwarePendingPath(download.Directory, download.Executable); err != nil {
			return err
		}
		if softwarePathEqual(download.Package, download.Executable) {
			return fmt.Errorf("pending package and executable overlap")
		}
	}
	if mode == update.ModeMacOS {
		if err := softwarePendingPath(download.Directory, download.Bundle); err != nil {
			return err
		}
	}
	return update.VerifyUpdateDownload(download)
}

func (s *State) removeSoftwarePending(pending *softwarePending) {
	if s.softwarePendingDirectory(pending) {
		os.RemoveAll(pending.Download.Directory)
	}
}

func (s *State) discardInvalidSoftwarePending(pending *softwarePending) {
	s.SoftwarePending = nil
	s.SoftwareUpdate.Downloaded = update.Download{}
	if err := s.savePreferences(); err != nil {
		s.SaveError = softwareText.downloadPreferenceError + err.Error()
		return
	}
	s.removeSoftwarePending(pending)
}

func (s *State) restoreSoftwarePending() {
	pending := s.SoftwarePending
	s.SoftwareUpdate.revision++
	revision := s.SoftwareUpdate.revision
	s.SoftwareUpdate.Busy = true
	ctx, cancel := context.WithTimeout(s.Ctx, 2*time.Minute)
	s.SoftwareUpdate.cancel = cancel
	executable := s.SoftwareExecutable
	installationOffer := s.softwareInstallationOffer()
	platform := s.Platform
	if platform == "" {
		platform = runtime.GOOS
	}
	go func() {
		err := s.validateSoftwarePending(pending)
		if err == nil {
			if executable == nil {
				err = fmt.Errorf("running executable is unavailable")
			} else {
				var target string
				target, err = executable()
				if err == nil {
					var offer update.Offer
					offer, err = installationOffer(ctx, pending.Offer, target, platform)
					if err == nil && (!update.SelfUpdating(offer) || softwareOfferMode(offer) != softwareOfferMode(pending.Offer) || offer.Package.Name != pending.Offer.Package.Name) {
						err = fmt.Errorf("running installation no longer matches the pending update")
					}
				}
			}
			if ctx.Err() != nil {
				err = ctx.Err()
			}
		}
		s.Update(func() {
			cancel()
			if revision != s.SoftwareUpdate.revision || s.Ctx.Err() != nil {
				return
			}
			s.SoftwareUpdate.Busy = false
			s.SoftwareUpdate.cancel = nil
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				s.setSoftwareStatus(softwareText.cancelled)
				return
			}
			if err != nil {
				s.SoftwarePending = nil
				if saveErr := s.savePreferences(); saveErr != nil {
					s.SaveError = softwareText.downloadPreferenceError + saveErr.Error()
				} else {
					s.removeSoftwarePending(pending)
				}
				s.startAutomaticSoftwareCheck()
				return
			}
			s.SoftwareUpdate.Offer = pending.Offer
			s.SoftwareUpdate.Available = true
			s.SoftwareUpdate.Downloaded = pending.Download
			s.setSoftwareStatus(softwareText.ready)
		})
	}()
}
