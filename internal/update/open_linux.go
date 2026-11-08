//go:build linux

package update

import "os/exec"

func OpenDownloaded(path string) error {
	command := exec.Command("xdg-open", path)
	if err := command.Start(); err != nil {
		return err
	}
	go command.Wait()
	return nil
}
