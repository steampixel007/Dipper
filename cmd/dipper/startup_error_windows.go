package main

import "golang.org/x/sys/windows"

func showStartupError(detail string) {
	message, _ := windows.UTF16PtrFromString("Dipper 无法启动。\n\n" + detail)
	title, _ := windows.UTF16PtrFromString("Dipper 启动失败")
	_, _ = windows.MessageBox(0, message, title, windows.MB_OK|windows.MB_ICONERROR)
}
