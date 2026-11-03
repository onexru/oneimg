package uploads

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/storage"

	"github.com/google/uuid"
)

// maxDuplicateSuffix 是同名文件自动编号的上限。超过后放弃可读名并回退到
// UUID 命名，避免在异常场景（例如存储里已经堆了几十个同名文件）里无限探测。
const maxDuplicateSuffix = 50

// errNameIndeterminate 表示无法判断目标 key 是否被占用（权限、网络等）。
// 此时既不能直接写入（可能覆盖既有图片），也不应让上传失败，调用方据此回退到
// UUID 命名。
var errNameIndeterminate = errors.New("artifact availability unknown")

func probeArtifact(ctx context.Context, driver storage.Driver, key, baseName, bucketType string) (bool, error) {
	if bucketType == "telegram" {
		// Telegram 的 key 只是逻辑路径，真实对象由 tg_file_id 定位，Stat 永远
		// 报 not found。而 image_tele_grams.file_name 是回查 TG 元数据的键，
		// 重名会让两张图互相读到对方的消息 ID，因此改用数据库判断。
		db := database.GetDB()
		if db == nil || db.DB == nil {
			return false, errNameIndeterminate
		}
		var count int64
		if err := db.DB.Model(&models.ImageTeleGram{}).Where("file_name = ?", baseName).Count(&count).Error; err != nil {
			return false, errNameIndeterminate
		}
		return count > 0, nil
	}
	_, err := driver.Stat(ctx, storage.Ref{Key: key})
	if err == nil {
		return true, nil
	}
	// 只有明确的 not found 才说明可以安全写入。
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	return false, errNameIndeterminate
}

// reserveArtifactKey 为「保存源文件名」挑选一个不覆盖既有对象的 key。
//
// 开这个开关意味着用户要拿回原始文件名，因此不再追加 UUID；代价是同一目录下
// 重名时必须自行编号（name.webp → name-2.webp → name-3.webp），否则第二次上传
// 会直接覆盖第一张图，而两条记录仍指向同一个 URL。
func reserveArtifactKey(ctx context.Context, driver storage.Driver, dir, name, bucketType string) (key, uniqueName string, err error) {
	ext := ""
	stem := name
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		ext, stem = name[i:], name[:i]
	}
	dir = strings.TrimRight(dir, "/")
	for n := 1; n <= maxDuplicateSuffix; n++ {
		candidate := name
		if n > 1 {
			candidate = fmt.Sprintf("%s-%d%s", stem, n, ext)
		}
		full, keyErr := storage.Key(dir + "/" + candidate)
		if keyErr != nil {
			return "", "", keyErr
		}
		taken, probeErr := probeArtifact(ctx, driver, full, candidate, bucketType)
		if probeErr != nil {
			return "", "", probeErr
		}
		if !taken {
			return full, candidate, nil
		}
	}
	return "", "", errNameIndeterminate
}

// uniqueArtifactName 在无法确认 key 是否被占用时生成带 UUID 的唯一名。
// 这是历史命名方式，能保证不覆盖任何既有对象。
func uniqueArtifactName(dir, name, uuidValue string) (key, uniqueName string, err error) {
	ext := ""
	stem := name
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		ext, stem = name[i:], name[:i]
	}
	if uuidValue == "" {
		uuidValue = uuid.NewString()
	}
	uniqueName = stem + "_" + uuidValue + ext
	key, err = storage.Key(strings.TrimRight(dir, "/") + "/" + uniqueName)
	return key, uniqueName, err
}
