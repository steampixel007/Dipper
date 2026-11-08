//go:build windows

package update

import "os/exec"

func OpenDownloaded(path string) error {
	command := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", path)
	hideHelper(command)
	if err := command.Start(); err != nil {
		return err
	}
	go command.Wait()
	return nil
}
