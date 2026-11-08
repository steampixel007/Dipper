package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/steampixel007/Dipper/internal/update"
)

type softwarePending struct {
	Offer    update.Offer
	Download update.Download
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

func (s *State) softwarePortable(offer update.Offer) bool {
	return s.Platform == "windows" && offer.Portable && !offer.Development && !update.Development(version)
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
	return filepath.Join(config, "updates"), nil
}

func (s *State) softwarePendingDirectory(pending *softwarePending) bool {
	root, err := s.softwareUpdateRoot()
	if err != nil {
		return false
	}
	directory, err := softwareCanonicalPath(pending.Download.Directory)
	if err != nil {
		return false
	}
	return softwarePathEqual(filepath.Dir(directory), root) &&
		strings.HasPrefix(filepath.Base(directory), "dipper-update-download-")
}

func (s *State) validateSoftwarePending(pending *softwarePending) error {
	comparison, err := update.Compare(pending.Offer.Release.Tag, version)
	if err != nil || comparison <= 0 || !s.softwarePortable(pending.Offer) || !pending.Offer.Available {
		return fmt.Errorf("pending update is not a newer portable release")
	}
	download := pending.Download
	if !s.softwarePendingDirectory(pending) {
		return fmt.Errorf("pending update is outside the managed download directory")
	}
	directory, err := softwareCanonicalPath(download.Directory)
	if err != nil {
		return err
	}
	packagePath, err := softwareCanonicalPath(download.Package)
	if err != nil {
		return err
	}
	executablePath, err := softwareCanonicalPath(download.Executable)
	if err != nil {
		return err
	}
	if !softwarePathEqual(filepath.Dir(packagePath), directory) ||
		!softwarePathEqual(executablePath, filepath.Join(directory, "Dipper.exe")) ||
		softwarePathEqual(packagePath, executablePath) {
		return fmt.Errorf("pending update is outside the managed download directory")
	}
	return update.VerifyPortableDownload(download)
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
	executable := s.SoftwareExecutable
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
					offer, err = update.WindowsInstallationOffer(pending.Offer, target)
					if err == nil && !offer.Portable {
						err = fmt.Errorf("running copy is no longer portable")
					}
				}
			}
		}
		s.Update(func() {
			if revision != s.SoftwareUpdate.revision || s.Ctx.Err() != nil {
				return
			}
			s.SoftwareUpdate.Busy = false
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
