package main

import (
	"fmt"
	"os"
	"path/filepath"
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

func (s *State) softwareUpdateRoot() (string, error) {
	config, err := filepath.Abs(s.ConfigDir)
	if err != nil {
		return "", err
	}
	config, err = filepath.EvalSymlinks(config)
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
	directory := pending.Download.Directory
	if filepath.Dir(directory) != root || !strings.HasPrefix(filepath.Base(directory), "dipper-update-download-") {
		return false
	}
	resolved, err := filepath.EvalSymlinks(directory)
	return err == nil && strings.EqualFold(resolved, directory)
}

func (s *State) validateSoftwarePending(pending *softwarePending) error {
	comparison, err := update.Compare(pending.Offer.Release.Tag, version)
	if err != nil || comparison <= 0 || !s.softwarePortable(pending.Offer) || !pending.Offer.Available {
		return fmt.Errorf("pending update is not a newer portable release")
	}
	download := pending.Download
	if !s.softwarePendingDirectory(pending) || filepath.Dir(download.Package) != download.Directory ||
		download.Executable != filepath.Join(download.Directory, "Dipper.exe") || download.Package == download.Executable {
		return fmt.Errorf("pending update is outside the managed download directory")
	}
	return update.VerifyPortableDownload(download)
}

func (s *State) removeSoftwarePending(pending *softwarePending) {
	if s.softwarePendingDirectory(pending) {
		os.RemoveAll(pending.Download.Directory)
	}
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
