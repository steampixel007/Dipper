package main

import "dipper.local/dipper/internal/proxy"

func (s *State) describeAutoProxy() string {
	address, source := proxy.Describe()
	if source == "environment" || source == "system" {
		return "跟随系统代理：" + address
	}
	return "跟随系统代理：未检测到代理，将直接连接。"
}

func (s *State) proxyDescription() string {
	switch proxyModes[s.ProxyMode] {
	case proxy.ModeDirect:
		return "不使用代理，直接连接蜜柑计划与下载源。"
	case proxy.ModeCustom:
		if s.ProxyURL == "" {
			return "自定义代理：还未填写地址。"
		}
		return "自定义代理：" + s.ProxyURL
	}
	return s.ProxyAuto
}
