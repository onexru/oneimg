package controllers

import (
	"log"

	"oneimg/backend/models"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/telegram"
)

// sendTGNotification 发送上传成功 Telegram 通知。TGBotToken 在数据库中
// 以 enc:v1: 密文存储，必须经 settings.GetSecret 解密后使用——直接使用
// settings 行的原始值会把密文当 token 发给 Telegram API，必然 401 且
// 通知静默失败。
func sendTGNotification(settingRow models.Settings, data telegram.PlaceholderData) {
	if !settingRow.TGNotice {
		return
	}
	token, err := settings.GetSecret("tg_bot_token")
	if err != nil {
		log.Printf("读取 Telegram 通知密钥失败: %v", err)
		return
	}
	if token == "" {
		return
	}
	if err := telegram.SendSimpleMsg(token, settingRow.TGReceivers, settingRow.TGNoticeText, data); err != nil {
		log.Println(err)
	}
}
