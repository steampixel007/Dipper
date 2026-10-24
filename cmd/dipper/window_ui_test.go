package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo/ui"
)

func windowFixture(t *testing.T) (*State, *int, *int) {
	t.Helper()
	s := uiFixture(t, "browse")
	s.Platform = "windows"
	s.TrayAvailable = true
	s.CheckTray = func() bool { return s.TrayAvailable }
	hidden, quit := 0, 0
	s.HideWindow = func() { hidden++ }
	s.Quit = func() { quit++ }
	return s, &hidden, &quit
}

func TestWindowCloseAsksAndCancelsWithoutSideEffects(t *testing.T) {
	for _, cancel := range []string{"button", "escape", "backdrop"} {
		t.Run(cancel, func(t *testing.T) {
			s, hidden, quit := windowFixture(t)
			s.requestClose()
			tt := ui.NewTester(s.view, 1040, 720)
			uiRequireText(t, tt, windowText.question, windowText.background, windowText.quit, windowText.remember)
			if !s.ClosePrompt || s.RememberClose || *hidden != 0 || *quit != 0 {
				t.Fatal("closing must ask before hiding or quitting, with remember unchecked")
			}
			uiClick(t, tt, windowText.remember)
			s.requestClose()
			if !s.RememberClose {
				t.Fatal("a repeated close request reset the open dialog's choice")
			}
			switch cancel {
			case "button":
				uiClick(t, tt, windowText.cancel)
			case "escape":
				tt.Key(0, ui.KeyEscape)
			case "backdrop":
				tt.ClickAt(10, 100)
			}
			if s.ClosePrompt || *hidden != 0 || *quit != 0 || s.CloseBehavior != closeAsk {
				t.Fatal("cancelling the close changed the window or its preference")
			}
			if _, err := os.Stat(filepath.Join(s.Dir, "preferences.json")); !os.IsNotExist(err) {
				t.Fatal("cancelling must not write preferences")
			}
			s.requestClose()
			if s.RememberClose {
				t.Fatal("a fresh prompt must start with remember unchecked")
			}
		})
	}
}

func TestWindowCloseChoicesRememberOnlyWhenChecked(t *testing.T) {
	for _, behavior := range []string{closeBackground, closeQuit} {
		for _, remember := range []bool{false, true} {
			t.Run(behavior+map[bool]string{false: "/once", true: "/remember"}[remember], func(t *testing.T) {
				s, hidden, quit := windowFixture(t)
				s.requestClose()
				tt := ui.NewTester(s.view, 1040, 720)
				if remember {
					uiClick(t, tt, windowText.remember)
				}
				label := windowText.background
				if behavior == closeQuit {
					label = windowText.quit
				}
				uiClick(t, tt, label)
				wantHidden, wantQuit := 1, 0
				if behavior == closeQuit {
					wantHidden, wantQuit = 0, 1
				}
				if s.ClosePrompt || *hidden != wantHidden || *quit != wantQuit {
					t.Fatalf("choice %s: hidden=%d quit=%d prompt=%v", behavior, *hidden, *quit, s.ClosePrompt)
				}
				restarted := newState(s.Dir)
				restarted.Source = source.New("http://127.0.0.1")
				if err := restarted.loadPreferences(); err != nil {
					t.Fatal(err)
				}
				want := closeAsk
				if remember {
					want = behavior
				}
				if restarted.CloseBehavior != want {
					t.Fatalf("after restart close=%q, want %q", restarted.CloseBehavior, want)
				}
				if !remember {
					s.requestClose()
					if !s.ClosePrompt {
						t.Fatal("one-time action suppressed the next prompt")
					}
				} else {
					s.requestClose()
					if s.ClosePrompt || *hidden != wantHidden*2 || *quit != wantQuit*2 {
						t.Fatal("remembered action was not applied on the next close")
					}
				}
			})
		}
	}
}

func TestWindowCloseUnavailableTrayKeepsWindowAndAllowsQuit(t *testing.T) {
	for _, behavior := range []string{closeAsk, closeBackground} {
		t.Run(behavior, func(t *testing.T) {
			s, hidden, quit := windowFixture(t)
			s.CloseBehavior = behavior
			s.CheckTray = func() bool { return false }
			s.requestClose()
			tt := ui.NewTester(s.view, 1040, 720)
			uiRequireText(t, tt, windowText.unavailable)
			uiClick(t, tt, windowText.background)
			if !s.ClosePrompt || *hidden != 0 || *quit != 0 {
				t.Fatal("unavailable tray must never hide the window or force an exit")
			}
			uiClick(t, tt, windowText.quit)
			if *quit != 1 || *hidden != 0 || s.CloseBehavior != behavior {
				t.Fatal("explicit exit must remain available without overwriting the saved choice")
			}
		})
	}
	// Registration can disappear after the dialog was opened.
	s, hidden, _ := windowFixture(t)
	s.requestClose()
	tt := ui.NewTester(s.view, 1040, 720)
	s.CheckTray = func() bool { return false }
	uiClick(t, tt, windowText.background)
	uiRequireText(t, tt, windowText.unavailable)
	if *hidden != 0 || !s.ClosePrompt {
		t.Fatal("tray availability must be checked at the moment of hiding")
	}
}

func TestWindowCloseSaveFailureKeepsWindowAndPreference(t *testing.T) {
	for _, behavior := range []string{closeBackground, closeQuit} {
		t.Run(behavior, func(t *testing.T) {
			s, hidden, quit := windowFixture(t)
			if err := os.Mkdir(filepath.Join(s.Dir, "preferences.json.tmp"), 0755); err != nil {
				t.Fatal(err)
			}
			s.requestClose()
			s.RememberClose = true
			if s.finishClose(behavior, true) {
				t.Fatal("a failed write must not complete the close")
			}
			if *hidden != 0 || *quit != 0 || !s.ClosePrompt || s.CloseBehavior != closeAsk || s.SaveError == "" {
				t.Fatal("failed preference write hid/quit the app or changed its remembered behavior")
			}
			tt := ui.NewTester(s.view, 1040, 720)
			uiRequireText(t, tt, s.SaveError)
		})
	}
}

func TestWindowSettingsCanChangeAndResetRememberedBehavior(t *testing.T) {
	s, _, quit := windowFixture(t)
	s.Page = "settings"
	tt := ui.NewTester(s.view, 1040, 720)
	for range 6 {
		tt.Wheel(800, 500, 0, 160)
		tt.Advance(100 * time.Millisecond)
	}
	uiRequireText(t, tt, windowText.closeLabel, windowText.ask, windowText.background, windowText.exitChoice)
	for _, choice := range []struct{ label, value string }{{windowText.background, closeBackground}, {windowText.exitChoice, closeQuit}, {windowText.ask, closeAsk}} {
		uiClick(t, tt, choice.label)
		pref, err := readPreferences(s.Dir)
		if err != nil || pref.CloseBehavior != choice.value || s.CloseBehavior != choice.value {
			t.Fatalf("settings choice %s was not saved: %+v %v", choice.value, pref, err)
		}
	}
	uiClick(t, tt, windowText.quit)
	if *quit != 1 {
		t.Fatal("settings exit must quit instead of selecting a close preference")
	}
}

func TestWindowSettingsFailureAndBusyKeepRememberedBehavior(t *testing.T) {
	s, hidden, quit := windowFixture(t)
	s.CloseBehavior = closeBackground
	if err := os.Mkdir(filepath.Join(s.Dir, "preferences.json.tmp"), 0755); err != nil {
		t.Fatal(err)
	}
	s.setCloseBehavior(closeQuit)
	if s.CloseBehavior != closeBackground || s.SaveError == "" {
		t.Fatal("failed setting write must restore the previous selection")
	}
	s.Busy = true
	s.setCloseBehavior(closeAsk)
	s.requestClose()
	s.exitApp()
	if s.finishClose(closeQuit, true) || s.CloseBehavior != closeBackground || *hidden != 0 || *quit != 0 || s.ClosePrompt {
		t.Fatal("busy operations must prevent close, quit and preference changes")
	}
}

func TestWindowCloseMacKeepsAppWithoutRequiringTray(t *testing.T) {
	s, hidden, quit := windowFixture(t)
	s.Platform = "darwin"
	s.CloseBehavior = closeQuit
	s.CheckTray = func() bool { t.Fatal("macOS must not depend on the menu bar extra to close"); return false }
	s.requestClose()
	if *hidden != 1 || *quit != 0 || s.ClosePrompt {
		t.Fatal("the macOS close button must close the window and keep the app running")
	}
	s.Page = "settings"
	tt := ui.NewTester(s.view, 1040, 720)
	uiRequireText(t, tt, windowText.macDetail)
	if tt.HasText(windowText.ask) {
		t.Fatal("macOS should explain its native close/quit convention")
	}
	s.exitApp()
	if *quit != 1 {
		t.Fatal("macOS still needs an explicit quit action")
	}
}

func TestWindowClosePreferenceMissingLegacyAndUnknownAsk(t *testing.T) {
	for _, content := range []string{"", `{"IntroHidden":true}`, `{"CloseBehavior":"future-value"}`} {
		t.Run(content, func(t *testing.T) {
			s, _, _ := windowFixture(t)
			if content != "" {
				if err := os.WriteFile(filepath.Join(s.Dir, "preferences.json"), []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			s.Source = source.New("http://127.0.0.1")
			if err := s.loadPreferences(); err != nil {
				t.Fatal(err)
			}
			if s.CloseBehavior != closeAsk {
				t.Fatalf("unspecified/unknown close behavior should ask, got %q", s.CloseBehavior)
			}
		})
	}
}

func TestWindowCloseChoicePreservesOtherPreferences(t *testing.T) {
	s, _, _ := windowFixture(t)
	s.IntroHidden = true
	s.ReduceEffects = true
	s.SourceRoute = s.SourceRoutes[1].Label
	s.setCloseBehavior(closeBackground)
	restarted := newState(s.Dir)
	restarted.Source = source.New("http://127.0.0.1")
	if err := restarted.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if restarted.CloseBehavior != closeBackground || !restarted.IntroHidden || !restarted.ReduceEffects || restarted.Source.Origin() != s.SourceRoutes[1].Origin {
		t.Fatal("changing the close action must preserve appearance, intro and source preferences")
	}
}
