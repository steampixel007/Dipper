package main

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/proxy"
)

func (s *State) describeAutoProxy() string {
	address, source := proxy.Describe()
	if source == "environment" || source == "system" {
		return "跟随系统代理：" + address
	}
	return "跟随系统代理：未检测到代理，将直接连接。"
}

func (s *State) proxyDescription() string {
	switch proxyModes[s.proxyActiveMode] {
	case proxy.ModeDirect:
		return "不使用代理，直接连接蜜柑计划与下载源。"
	case proxy.ModeCustom:
		return "自定义代理：" + s.proxyActiveAddress
	}
	return s.ProxyAuto
}

func (s *State) proxySettings(c *ui.Context) {
	pal := colors(c)
	custom := proxyModes[s.ProxyMode] == proxy.ModeCustom
	disabled := s.Busy || s.ProxyTesting
	clearFeedback := func() { s.ProxyFeedback = "" }
	ui.Column(c).Gap(12).Children(func() {
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(16).Children(func() {
			ui.Column(c).Grow(1).MinWidth(textSize(c, textBody) * 10).Gap(4).Children(func() {
				ui.Text(c, "代理").FontSize(textSize(c, textBody)).FontWeight(500).TextColor(pal.ink).LineHeight(1.5)
				caption(c, s.proxyDescription())
			})
			ui.Row(c).Wrap().AlignItems(ui.Center).Gap(8).Children(func() {
				if !custom {
					compactSecondary(c, "test-proxy", "测试", s.testProxy).
						Disabled(disabled || s.Source == nil)
				}
				glidingSegmented(c, "proxy-mode", "代理模式", s.ProxyMode, []string{"自动", "关闭", "自定义"}, proxyModeMatch{}, disabled, func(index int) {
					s.ProxyMode = index
					clearFeedback()
					if proxyModes[s.ProxyMode] != proxy.ModeCustom {
						s.saveProxy()
					}
				})
			})
		})
		if custom {
			ui.Column(c).Gap(12).Children(func() {
				ui.TextInput(c.Key("proxy-url"), &s.ProxyURL).Label("代理地址").Placeholder("http://127.0.0.1:7890").
					FillWidth().MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface).
					OnChange(clearFeedback).OnSubmit(s.saveProxy).Disabled(disabled)
				ui.Row(c).Wrap().AlignItems(ui.Center).Gap(8).Children(func() {
					ui.TextInput(c.Key("proxy-user"), &s.proxyUsername).Label("代理用户名").Placeholder("用户名（可选）").
						Grow(1).MinWidth(120).MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface).
						OnChange(clearFeedback).OnSubmit(s.saveProxy).Disabled(disabled)
					ui.TextInput(c.Key("proxy-password"), &s.proxyPassword).Label("代理密码").Placeholder("密码（可选）").Password().
						Grow(1).MinWidth(120).MinHeight(inputHeight(c)).Radius(radiusSmall).Background(pal.surface).
						OnChange(clearFeedback).OnSubmit(s.saveProxy).Disabled(disabled)
					compactSecondary(c, "test-proxy", "测试", s.testProxy).
						Disabled(disabled || s.Source == nil || strings.TrimSpace(s.ProxyURL) == "")
					secondary(c, "save-proxy", "保存", s.saveProxy).Label("保存代理").Disabled(disabled)
				})
			})
		}
		if s.ProxyFeedback != "" {
			caption(c, s.ProxyFeedback)
		}
	})
}

func (s *State) proxySelection() (mode, address string, ok bool) {
	mode = proxyModes[s.ProxyMode]
	if mode != proxy.ModeCustom {
		return mode, "", true
	}
	if strings.TrimSpace(s.ProxyURL) == "" {
		s.ProxyFeedback = "请填写代理地址。"
		return mode, "", false
	}
	address, err := proxy.AddressWithCredentials(s.ProxyURL, s.proxyUsername, s.proxyPassword)
	if err != nil {
		s.ProxyFeedback = "代理地址格式不正确，请填写主机和端口，用户名和密码请分别填写。"
		return mode, "", false
	}
	return mode, address, true
}

func (s *State) saveProxy() {
	if s.Busy || s.ProxyTesting {
		return
	}
	mode, address, ok := s.proxySelection()
	if !ok {
		return
	}
	if s.Core != nil {
		settings, err := s.Core.GetSettings()
		if err == nil {
			settings.ProxyMode, settings.ProxyURL = mode, address
			err = s.Core.SetSettings(settings)
		}
		if err != nil {
			s.ProxyFeedback = "无法保存代理设置，请重试。"
			if mode != proxy.ModeCustom {
				s.ProxyMode = s.proxyActiveMode
			}
			return
		}
		proxy.Configure(mode, address)
		s.ProxyAuto = s.describeAutoProxy()
	}
	s.proxyActiveMode = s.ProxyMode
	s.proxyActiveAddress, _, _ = proxy.SplitAddress(address)
	s.ProxyFeedback = "代理设置已保存。"
}

func (s *State) testProxy() {
	if s.Busy || s.ProxyTesting || s.Source == nil {
		return
	}
	mode, address, ok := s.proxySelection()
	if !ok {
		return
	}
	s.ProxyTesting = true
	s.ProxyFeedback = "正在测试连接…"
	go func() {
		ctx, cancel := context.WithTimeout(s.Ctx, 45*time.Second)
		defer cancel()
		err := s.Source.ProbeWithProxy(ctx, mode, address)
		s.Update(func() {
			s.ProxyTesting = false
			if err != nil {
				s.ProxyFeedback = "连接失败，请检查代理设置后重试。"
				return
			}
			s.ProxyFeedback = "连接成功。"
		})
	}()
}
