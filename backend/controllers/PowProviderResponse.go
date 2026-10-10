package controllers

import (
	"oneimg/backend/models"
	"strings"
)

func effectivePowScriptURL(s models.Settings) string {
	if strings.TrimSpace(s.PowScriptURL) == "" {
		return "https://cha.eta.im/static/js/pow.min.js"
	}
	return strings.TrimSpace(s.PowScriptURL)
}
func effectivePowWidgetURL(s models.Settings) string {
	if strings.TrimSpace(s.PowWidgetURL) == "" {
		return "https://cha.eta.im/"
	}
	return strings.TrimSpace(s.PowWidgetURL)
}
