package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

var spectrumEase = ui.CubicBezier(0, 0, .4, 1)
var feedbackMotion = ui.Animation{Duration: 130 * time.Millisecond, Ease: spectrumEase}
var contentMotion = ui.Animation{Duration: 190 * time.Millisecond, Ease: spectrumEase}
var hoverTransition = ui.ElementTransition{Colors: true, Duration: 130 * time.Millisecond, Ease: spectrumEase}
var pageTransition = ui.ElementTransition{Duration: 190 * time.Millisecond, Ease: spectrumEase, Position: true, Enter: &ui.Motion{Y: 6}, Exit: &ui.Motion{Y: -4}}
var popupTransition = ui.ElementTransition{Duration: 160 * time.Millisecond, Ease: spectrumEase, Enter: &ui.Motion{Y: 4, Scale: .985}, Exit: &ui.Motion{Y: 2, Scale: .99}}
var fadeTransition = ui.ElementTransition{Duration: 160 * time.Millisecond, Ease: spectrumEase, Colors: true, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
var rowTransition = ui.ElementTransition{Duration: 190 * time.Millisecond, Ease: spectrumEase, Position: true, Colors: true, Enter: &ui.Motion{Y: 4}, Exit: &ui.Motion{}}

func tactile(e ui.Element) ui.Element {
	e.Transition(hoverTransition)
	pose := ui.Pose{}
	if e.Pressed() {
		pose.Scale = .97
	}
	return e.AnimatePose(pose, feedbackMotion)
}
