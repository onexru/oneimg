package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"oneimg/backend/models"
	"strconv"
)

func MetadataInt(metadata map[string]any, key string) int {
	value := metadata[key]
	switch v := value.(type) {
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case float64:
		return int(v)
	default:
		n, _ := strconv.Atoi(fmt.Sprint(value))
		return n
	}
}
func MetadataString(metadata map[string]any, key string) string {
	if v, ok := metadata[key].(string); ok {
		return v
	}
	return ""
}
func TelegramMetadata(row models.ImageTeleGram) map[string]any {
	return map[string]any{"tg_file_id": row.TGFileId, "tg_thumbnail_file_id": row.TGThumbnailFileId, "tg_message_id": row.TGMessageId, "tg_thumbnail_message_id": row.TGThumbnailMessageId}
}

// ResolveTelegramMetadata merges old image_tele_grams rows field-by-field.
// A partially populated replica must not accidentally select its main file when
// the thumbnail ID is still stored only in the legacy table.
func ResolveTelegramMetadata(db *gorm.DB, filename string, metadata map[string]any) (map[string]any, error) {
	result := map[string]any{}
	for k, v := range metadata {
		result[k] = v
	}
	if MetadataString(result, "tg_file_id") != "" && MetadataString(result, "tg_thumbnail_file_id") != "" && MetadataInt(result, "tg_message_id") > 0 && MetadataInt(result, "tg_thumbnail_message_id") > 0 {
		return result, nil
	}
	if db == nil {
		return result, nil
	}
	var row models.ImageTeleGram
	err := db.Where("file_name = ?", filename).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for k, v := range TelegramMetadata(row) {
		if MetadataString(result, k) == "" && MetadataInt(result, k) == 0 {
			result[k] = v
		}
	}
	return result, nil
}
func ArtifactRef(key string, metadata map[string]any, thumbnail bool) Ref {
	prefix := "tg_"
	if thumbnail {
		prefix = "tg_thumbnail_"
	}
	return Ref{Key: key, Metadata: map[string]any{"tg_file_id": metadata[prefix+"file_id"], "tg_message_id": metadata[prefix+"message_id"], "tg_chat_id": metadata["tg_chat_id"]}}
}
func MergeArtifactMetadata(target map[string]any, info Info, thumbnail bool) {
	prefix := "tg_"
	if thumbnail {
		prefix = "tg_thumbnail_"
	}
	for _, field := range []string{"file_id", "message_id"} {
		if v, ok := info.Metadata["tg_"+field]; ok {
			target[prefix+field] = v
		}
	}
	if v, ok := info.Metadata["tg_chat_id"]; ok {
		target["tg_chat_id"] = v
	}
}
