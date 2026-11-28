package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/core"
)

type limitInput struct {
	value     string
	unlimited bool
}

type transferLimits struct {
	upload, duration, ratio                limitInput
	savedUpload, savedDuration, savedRatio limitInput
	message                                string
	failed                                 bool
}

func (draft transferLimits) dirty() bool {
	return draft.upload != draft.savedUpload || draft.duration != draft.savedDuration || draft.ratio != draft.savedRatio
}

func (s *State) loadTransferLimits(settings core.Settings) {
	minutes := settings.SeedMinutes
	if minutes <= 0 {
		minutes = 60
	}
	ratio := strings.TrimSuffix(settings.SeedRatio, ":1")
	if ratio == "" || ratio == "不限" {
		ratio = "1"
	}
	upload := limitInput{strconv.FormatFloat(float64(settings.UploadLimitBytes)/(1024*1024), 'f', -1, 64), settings.UploadUnlimited}
	duration := limitInput{strconv.FormatInt(minutes, 10), settings.EffectiveSeedDuration() == -1}
	ratioInput := limitInput{ratio, settings.EffectiveSeedRatio() == -1}
	s.TransferLimits = transferLimits{
		upload:        upload,
		duration:      duration,
		ratio:         ratioInput,
		savedUpload:   upload,
		savedDuration: duration,
		savedRatio:    ratioInput,
	}
	s.Upload = uploadLabel(settings.EffectiveUploadLimit())
	s.Seed = seedLabel(settings.SeedMinutes)
	if s.TransferLimits.duration.unlimited {
		s.Seed = "不限时长"
	}
	s.SeedRatio = ratio + ":1"
	if s.TransferLimits.ratio.unlimited {
		s.SeedRatio = "不限"
	}
}

func (draft transferLimits) apply(settings *core.Settings) error {
	upload := strings.TrimSpace(draft.upload.value)
	savedUpload := strconv.FormatFloat(float64(settings.UploadLimitBytes)/(1024*1024), 'f', -1, 64)
	if rate, ok := parseByteRate(upload + " MiB/s"); ok && !strings.ContainsAny(upload, " \t\r\nxXpP_") {
		if upload != savedUpload {
			settings.UploadLimitBytes = rate
		}
	} else if !draft.upload.unlimited && !(upload == "0" && settings.UploadLimitBytes == 0) {
		return fmt.Errorf("全局上传上限请输入大于 0 的数字，或勾选「不限」")
	}
	minutes, err := strconv.ParseInt(strings.TrimSpace(draft.duration.value), 10, 64)
	if err == nil && minutes > 0 && minutes <= int64(math.MaxInt64)/int64(time.Minute) {
		settings.SeedMinutes = minutes
	} else if !draft.duration.unlimited {
		return fmt.Errorf("分享时长请输入有效的正整数分钟，或勾选「不限」")
	} else if settings.SeedMinutes == -1 {
		settings.SeedMinutes = 60
	}
	ratio := strings.TrimSpace(draft.ratio.value) + ":1"
	if core.ValidateSeedRatio(ratio) == nil {
		settings.SeedRatio = ratio
	} else if !draft.ratio.unlimited {
		return fmt.Errorf("分享率上限请输入大于 0 的数字，或勾选「不限」")
	} else if settings.SeedRatio == "不限" {
		settings.SeedRatio = "1:1"
	}
	settings.UploadUnlimited = draft.upload.unlimited
	settings.SeedTimeUnlimited = draft.duration.unlimited
	settings.SeedRatioUnlimited = draft.ratio.unlimited
	return nil
}

func (s *State) saveTransferLimits() {
	settings := core.Settings{UploadLimitBytes: uploadLimit(s.Upload)}
	var err error
	if s.Core != nil {
		settings, err = s.Core.GetSettings()
	}
	if err == nil {
		err = s.TransferLimits.apply(&settings)
	}
	if err == nil && s.Core != nil {
		if err = s.Core.SetSettings(settings); err != nil {
			err = fmt.Errorf("无法保存上传与分享设置：%w", err)
		}
	}
	if err != nil {
		s.TransferLimits.message, s.TransferLimits.failed = err.Error(), true
		return
	}
	s.loadTransferLimits(settings)
	if s.Engine != nil {
		s.Engine.SetSeedTarget(settings.EffectiveSeedDuration())
		s.Engine.SetRatioTarget(settings.EffectiveSeedRatio())
		if err := s.Engine.SetUploadLimit(settings.EffectiveUploadLimit()); err != nil {
			s.TransferLimits.message, s.TransferLimits.failed = "设置已保存，重新启动后应用上传限速。", true
			return
		}
	}
	s.TransferLimits.message = "上传与分享设置已保存。"
}

func (s *State) transferSettings(c *ui.Context) {
	draft := &s.TransferLimits
	s.limitRow(c, "upload", "全局上传上限", "MiB/s", &draft.upload)
	s.limitRow(c, "duration", "分享时长", "分钟", &draft.duration)
	s.limitRow(c, "ratio", "分享率上限", ":1", &draft.ratio)
	ui.Row(c).Wrap().AlignItems(ui.Center).Gap(16).Children(func() {
		caption(c, "下载完成后，"+s.seedStopRule()+"。").Grow(1).MinWidth(0)
		secondary(c, "save-transfer-limits", "保存限额", s.saveTransferLimits).Disabled(s.Busy || !draft.dirty())
	})
	if draft.failed {
		ui.Text(c, draft.message).FontSize(textSize(c, textMeta)).TextColor(colors(c).errorInk).LineHeight(1.5)
	} else if draft.message != "" {
		caption(c, draft.message)
	}
}

func (s *State) limitRow(c *ui.Context, key, label, unit string, input *limitInput) {
	pal := colors(c)
	ui.Row(c).Wrap().AlignItems(ui.Center).Gap(16).Children(func() {
		fieldLabel := ui.Text(c, label).FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink).LineHeight(1.5).Grow(1).MinWidth(textSize(c, textBody) * 10)
		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.TextInput(c.Key(key+"-limit"), &input.value).LabelledBy(fieldLabel).Width(textSize(c, textControl) * 6).
				MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface).
				Disabled(input.unlimited || s.Busy).OnSubmit(s.saveTransferLimits)
			ui.Text(c, unit).Width(textSize(c, textControl) * 4).FontSize(textSize(c, textMeta)).TextColor(pal.muted)
			ui.Checkbox(c.Key(key+"-unlimited"), &input.unlimited, "不限").Label(label + "不限").Disabled(s.Busy)
		})
	})
}
