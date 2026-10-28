package main

import "github.com/egoist/mygo/ui"

const (
	closeAsk        = "ask"
	closeBackground = "background"
	closeQuit       = "quit"
)

var windowText = struct {
	title, question, ask, background, quit, exitChoice, cancel, remember string
	backgroundDetail, quitDetail, unavailable, overflow, busy            string
	macDetail, closeLabel, preferenceError                               string
}{
	title: "窗口与后台运行", question: "关闭 Dipper？", ask: "每次询问",
	background: "收起到托盘", quit: "退出 Dipper", exitChoice: "退出应用", cancel: "取消", remember: "记住我的选择",
	backgroundDetail: "收起到托盘后，继续追番和下载。",
	quitDetail:       "退出后停止追番和下载，下次打开时继续。",
	unavailable:      "系统托盘不可用，窗口将保持打开。你仍可选择退出 Dipper。",
	overflow:         "托盘图标可能位于任务栏右下角的“隐藏的图标”中，单击可重新打开窗口。",
	busy:             "操作正在进行，请稍后再关闭。",
	macDetail:        "关闭窗口后继续追番和下载，点击 Dock 图标可重新打开。退出 Dipper 或按 ⌘Q 才会停止后台活动。",
	closeLabel:       "关闭窗口时", preferenceError: "关闭方式保存失败：",
}

func normalizeCloseBehavior(value string) string {
	switch value {
	case closeBackground, closeQuit:
		return value
	default:
		return closeAsk
	}
}

func (s *State) backgroundAvailable() bool {
	if s.Platform == "darwin" {
		return true // The Dock can reopen the window without a menu bar extra.
	}
	if s.CheckTray != nil {
		s.TrayAvailable = s.CheckTray()
	}
	return s.TrayAvailable
}

func (s *State) requestClose() {
	if s.Busy {
		s.Notice = windowText.busy
		return
	}
	if s.ClosePrompt {
		return
	}
	if s.Platform == "darwin" {
		s.finishClose(closeBackground, false)
		return
	}
	switch normalizeCloseBehavior(s.CloseBehavior) {
	case closeQuit:
		s.exitApp()
	case closeBackground:
		if !s.finishClose(closeBackground, false) {
			s.ClosePrompt = true
			s.RememberClose = false
		}
	default:
		s.backgroundAvailable()
		s.RememberClose = false
		s.ClosePrompt = true
	}
}

func (s *State) saveBeforeClose() bool {
	if s.Busy {
		s.Notice = windowText.busy
		return false
	}
	if s.Core != nil {
		s.save()
	} else if err := s.savePreferences(); err != nil {
		s.SaveError = windowText.preferenceError + err.Error()
		return false
	} else {
		s.SaveError = ""
	}
	return s.SaveError == ""
}

func (s *State) exitApp() {
	if !s.saveBeforeClose() {
		s.restoreAfterQuitVeto()
		return
	}
	if s.Quit != nil {
		s.ClosePrompt = false
		s.Quit()
	}
}

func (s *State) restoreAfterQuitVeto() {
	s.Update(func() {
		if s.ShowWindow != nil {
			s.ShowWindow()
		}
	})
}

func (s *State) finishClose(behavior string, remember bool) bool {
	if s.Busy || (behavior != closeBackground && behavior != closeQuit) {
		return false
	}
	if behavior == closeBackground && !s.backgroundAvailable() {
		return false
	}
	previous := s.CloseBehavior
	if remember {
		s.CloseBehavior = behavior
	}
	if !s.saveBeforeClose() {
		s.CloseBehavior = previous
		return false
	}
	s.ClosePrompt = false
	if behavior == closeQuit {
		if s.Quit != nil {
			s.Quit()
		}
	} else if s.HideWindow != nil {
		s.HideWindow()
	}
	return true
}

func (s *State) setCloseBehavior(value string) {
	if s.Busy {
		return
	}
	previous := s.CloseBehavior
	s.CloseBehavior = normalizeCloseBehavior(value)
	if err := s.savePreferences(); err != nil {
		s.CloseBehavior = previous
		s.SaveError = windowText.preferenceError + err.Error()
	} else {
		s.SaveError = ""
	}
}

func (s *State) windowSettings(c *ui.Context) {
	ui.Column(c).Gap(12).Padding(22).Background(colors(c).paper).Border(1, colors(c).line).Radius(10).Children(func() {
		heading(c, windowText.title)
		if s.Platform == "darwin" {
			caption(c, windowText.macDetail)
		} else {
			caption(c, windowText.closeLabel)
			selected := normalizeCloseBehavior(s.CloseBehavior)
			ui.RadioGroup(c, func() {
				for _, choice := range []struct{ value, label string }{{closeAsk, windowText.ask}, {closeBackground, windowText.background}, {closeQuit, windowText.exitChoice}} {
					ui.Radio(c.Key("close-"+choice.value), &selected, choice.value, choice.label).OnChange(func() { s.setCloseBehavior(selected) }).Disabled(s.Busy)
				}
			})
			caption(c, windowText.backgroundDetail+windowText.quitDetail)
			if !s.TrayAvailable {
				caption(c, windowText.unavailable)
			}
		}
		secondary(c, "quit", windowText.quit, s.exitApp).AlignSelf(ui.Start).FontSize(12).Disabled(s.Busy)
	})
}

func (s *State) closeDialog(c *ui.Context) {
	ui.DialogBase(c, &s.ClosePrompt, func(backdrop, panel ui.Element) {
		backdrop.Background(colors(c).scrim).Transition(fadeTransition)
		panel.Width(510).Padding(28).Gap(18).Radius(12).Background(colors(c).paper)
		ui.Text(c, windowText.question).FontSize(23).FontWeight(700)
		caption(c, windowText.backgroundDetail)
		caption(c, windowText.quitDetail)
		if !s.TrayAvailable {
			ui.Text(c, windowText.unavailable).FontSize(12).TextColor(colors(c).amber).LineHeight(1.5)
		} else if s.Platform == "windows" {
			caption(c, windowText.overflow)
		}
		if s.SaveError != "" {
			ui.Text(c, s.SaveError).FontSize(12).TextColor(colors(c).errorInk).LineHeight(1.5)
		}
		ui.Checkbox(c.Key("remember-close"), &s.RememberClose, windowText.remember).Disabled(s.Busy)
		ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
			quiet(c, "cancel-close", windowText.cancel, func() { s.ClosePrompt = false })
			secondary(c, "close-quit", windowText.quit, func() { s.finishClose(closeQuit, s.RememberClose) }).Disabled(s.Busy)
			primary(c, "close-background", windowText.background, func() { s.finishClose(closeBackground, s.RememberClose) }).Disabled(s.Busy || !s.TrayAvailable)
		})
	})
}
