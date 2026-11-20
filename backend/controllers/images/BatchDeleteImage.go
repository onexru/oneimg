package images

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"time"

	"oneimg/backend/utils/result"

	"github.com/gin-gonic/gin"
)

// 单次批量删除的 ID 上限。图库一页 30 张（选中全部即 30 个 ID），留足余量；
// 设上限是避免一条请求触发无界的串行删除。
const maxBatchDeleteCount = 200

type batchDeleteRequest struct {
	IDs []int `json:"ids"`
}

type batchDeleteFailure struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
}

// BatchDeleteImages 一次请求删除多张图片。
//
// 之前前端对每张图各发一个 DELETE，30 张就是 30 个请求；而服务端删副本要拿同
// 一把全局锁（services.storageReplicaOperationMu），实际仍是串行执行。请求一多，
// 排在队尾的那些只会在反代里空等到超时被掐断，表现成「每次只删掉一部分」。
//
// 现在一次请求带上全部 ID，在服务端顺序删完再返回，逐张给出结果：
// 单张失败不影响其它张，前端也不必再猜哪张没删掉。
func BatchDeleteImages(c *gin.Context) {
	var req batchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, `请求参数格式不正确：需要 JSON 格式 {"ids":[1,2,3]}`))
		return
	}

	ids := normalizeDeleteIDs(req.IDs)
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "请选择要删除的图片"))
		return
	}
	if len(ids) > maxBatchDeleteCount {
		c.JSON(http.StatusBadRequest, result.Error(400, fmt.Sprintf("单次最多删除 %d 张图片", maxBatchDeleteCount)))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), batchDeleteTimeout(len(ids)))
	defer cancel()

	deleted := make([]int, 0, len(ids))
	failed := make([]batchDeleteFailure, 0)

	for _, id := range ids {
		if ctx.Err() != nil {
			// 预算用尽或客户端已断开：剩下的按未处理记失败，不再继续删。
			failed = append(failed, batchDeleteFailure{ID: id, Message: "删除超时，图片可能仍在，请重试"})
			continue
		}

		image, err := loadImageForDeletion(c, id)
		if err == nil {
			err = removeImageArtifacts(ctx, image)
		}
		if err != nil {
			// 已经不存在就按删除成功记：这样反代超时后重试是幂等的，
			// 不会把「上一次其实删掉了」的图报成失败。
			if isImageGoneError(err) {
				deleted = append(deleted, id)
				continue
			}
			var deleteErr *imageDeleteError
			message := "删除失败"
			if errors.As(err, &deleteErr) {
				message = deleteErr.message
			}
			log.Printf("批量删除图片 %d 失败：%v", id, err)
			failed = append(failed, batchDeleteFailure{ID: id, Message: message})
			continue
		}
		deleted = append(deleted, id)
	}

	data := gin.H{
		"requested":     len(ids),
		"deleted":       deleted,
		"failed":        failed,
		"deleted_count": len(deleted),
		"failed_count":  len(failed),
	}
	switch {
	case len(failed) == 0:
		c.JSON(http.StatusOK, result.Success(fmt.Sprintf("成功删除 %d 张图片", len(deleted)), data))
	case len(deleted) == 0:
		c.JSON(http.StatusOK, result.Success(fmt.Sprintf("%d 张图片均未删除", len(failed)), data))
	default:
		c.JSON(http.StatusOK, result.Success(fmt.Sprintf("已删除 %d 张，%d 张未完成", len(deleted), len(failed)), data))
	}
}

// batchDeleteTimeout 按张数给整批一个预算：单张约 20s 的余量（远程存储慢删），
// 最少 1 分钟、最多 10 分钟。单条删除的 5 分钟上限放到批量里没有意义 ——
// 30 张就是 2.5 小时，远早于反代读超时就被掐断。
func batchDeleteTimeout(count int) time.Duration {
	budget := time.Duration(count) * 20 * time.Second
	if budget < time.Minute {
		return time.Minute
	}
	if budget > 10*time.Minute {
		return 10 * time.Minute
	}
	return budget
}

func isImageGoneError(err error) bool {
	var deleteErr *imageDeleteError
	return errors.As(err, &deleteErr) && deleteErr.status == http.StatusNotFound
}

// normalizeDeleteIDs 去重、丢掉非法 ID，并排序保证输出稳定（重试时顺序一致）。
func normalizeDeleteIDs(ids []int) []int {
	seen := make(map[int]struct{}, len(ids))
	normalized := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	slices.Sort(normalized)
	return normalized
}
