package api

// ThemeSettings 返回 ink 主题所需的完整 theme_settings 默认值。
// 键名与 komari-theme-ink/komari-theme.json 对齐——缺失的键 ink 会回退到内置默认值，
// 但显式下发可保证「零配置即满血」。
func ThemeSettings() map[string]any {
	return map[string]any{
		// 01 基础与外观
		"themeMode":          "beijing",   // beijing=按北京时间自动亮暗
		"dataUpdateInterval": 3,           // 秒
		"rpcTransportMode":   "websocket", // sounding 原生支持 /rpc2 WS
		"nodeCardSize":       "compact",
		"defaultViewMode":    "card",
		// 02 首页布局
		"alertEnabled":       false,
		"alertTitle":         "",
		"alertContent":       "",
		"visitorInfoEnabled": true,
		"colorVisionMode":    "标准",
		"generalCardPreset":  "基础",
		"generalCardKeys":    "onlineNodes\nhighLoadNodes\ntotalTraffic\nnetSpeed",
		// 03 高级工具与隐私
		"homeToolsEnabled":            true,
		"hideAdminEntryWhenLoggedOut": false,
		"hidePriceWhenLoggedOut":      false,
		"providerAliases":             "",
		"exportSecondaryPassword":     "",
		"disablePageAnimation":        false,
		// 04 节点卡片、列表与快捷控制
		"homeQuickControlsEnabled":    true,
		"homeQuickControlPreset":      "完整",
		"homeQuickControlKeys":        "favorite\ntotalTraffic\npeak\noffline",
		"nodeListMetadataEnabled":     true,
		"nodeListMetadataFields":      "provider\nregion\nasn",
		"nodeListCustomTagsVisible":   true,
		"offlineNodesLast":            false,
		"homeHighLoadThreshold":       80,
		"homeTrafficWarningThreshold": 80,
		"homeExpiringDays":            30,
		"diskPredictionEnabled":       false,
		"diskPredictionThresholdDays": 30,
		// 05 节点详情概览卡片
		"nodeDetailSectionTabsEnabled": false,
		"detailMetricCardPreset":       "财务",
		"detailMetricCardKeys":         "nodePrice\nmonthlyCost\nremainingTime\nremainingValue\ntotalTraffic\ntrafficQuota\nuptime\nconnections",
		// 06 节点详情图表
		"gpuChartEnabled":        false,
		"chartDashboardPreset":   "默认",
		"chartDashboardTemplate": "cpu\nmemory\ndisk\nnetwork\ngpu\nconnections\nprocess",
		// 07 自定义背景
		"backgroundEnabled":  false,
		"backgroundType":     "image",
		"lightBackgroundUrl": "",
		"darkBackgroundUrl":  "",
		"backgroundBlur":     0,
		"backgroundOverlay":  0,
	}
}

// PublicSettings ink 契约：站点公开设置（REST /api/public 与 RPC getPublicInfo 共用）。
func PublicSettings() map[string]any {
	return map[string]any{
		"allow_cors":                false,
		"custom_body":               "",
		"custom_head":               "",
		"description":               "sounding · 分布式探针",
		"disable_password_login":    true,
		"oauth_enable":              false,
		"oauth_provider":            nil,
		"ping_record_preserve_time": 30 * 24 * 60,
		"private_site":              false,
		"record_enabled":            true,
		"record_preserve_time":      30 * 24 * 60,
		"sitename":                  "sounding",
		"theme":                     "ink",
		"theme_settings":            ThemeSettings(),
		"visitor_audit_enabled":     false,
	}
}

// PublicSettingsResponse 包装为 ink REST 契约 {status, data}。
func PublicSettingsResponse() map[string]any {
	return map[string]any{"status": "success", "data": PublicSettings()}
}
