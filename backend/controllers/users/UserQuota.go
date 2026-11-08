package users

import (
	"errors"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	shared "oneimg/backend/controllers/shared"
	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/uploads"
)

// ErrQuotaExceeded 表示本次上传会超出账号配额。
var ErrQuotaExceeded = errors.New("账号存储配额已用完")

// quotaBytesForUser 解析某个用户实际生效的配额。
//
// 优先级（用户明确要求的三段式）：
//  1. users.quota_bytes 非 nil（管理员给这个账号单独设过）→ 就用它，**不再与默认值比较**；
//  2. 否则用 settings.default_user_quota（超管/有权限者可改，默认 10 GiB）；
//  3. 任一层为 0 表示不限。
//
// 注意「单独设过」和「设成 0」是两件事：0 是有效值（不限），
// 所以字段必须是 *int64 而不是用 0 当哨兵。
func quotaBytesForUser(user models.User, s models.Settings) int64 {
	if user.QuotaBytes != nil {
		return *user.QuotaBytes
	}
	return s.DefaultUserQuota
}

// usedBytesForUser 统计该账号当前占用的空间。
//
// 口径是 images 表里属于该用户、且未处于删除中的原图大小之和。
// 不含缩略图：缩略图是系统的派生物，无法被用户选择放弃，计入会让
// 「配额已满」的原因变得难以解释。也不含 sync 副本 —— 同一张图在多个
// 存储源上占的物理空间不该按份数惩罚用户。
func usedBytesForUser(db *gorm.DB, userID int) (int64, error) {
	var used int64
	err := db.Model(&models.Image{}).
		Where("user_id = ? AND deleting = ?", userID, false).
		Select("COALESCE(SUM(file_size), 0)").
		Scan(&used).Error
	return used, err
}

// EnforceUserQuota 在写入任何文件之前做一次粗检。
//
// 语义是「只拦确定超限的情况」，不是精确判断 —— 调用方传入的
// incomingBytes 来自 multipart 的声明大小，而实际落盘的往往是压缩后的
// webp（实测 721KB 的 PNG 会存成 268KB）。用声明大小做精确判断会把
// 「配额还剩 500KB、传一张会压缩到 268KB 的图」误判为超限，
// 用户看着配额没用满却传不进去。
//
// 因此这里只挡「已经完全没额度」这一种情况，真正的精确复核在
// EnforceQuotaInTx 里，用落盘后的真实 FileSize 在事务中判断。
//
// 返回值语义：
//   - nil：可以继续（不代表一定放得下）；
//   - ErrQuotaExceeded：已无额度，调用方应回 403；
//   - 其他 error：查询失败等基础设施问题，调用方应回 500。
func EnforceUserQuota(c *gin.Context, db *gorm.DB, userID int, incomingBytes int64) error {
	if userID <= 0 {
		// 游客（user_id 为负）与未登录不参与账号配额：前者没有稳定身份，
		// 后者在更早的中间件就被拦下了。
		return nil
	}
	setting, err := settings.GetSettings()
	if err != nil {
		return err
	}

	user, err := loadQuotaUser(db, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	// 管理员不受账号配额限制。
	//
	// 这不是给管理员开后门：配额的作用是「防止单个使用者把共享存储塞满」，
	// 而管理员本来就能改配额、能删任何图片、能换存储，限制他只会让运维
	// 在需要紧急备份/迁移时被自己的规则卡住。
	//
	// 角色取库内值而不是会话快照：会话可能是提权/降权之前建立的，
	// 用旧快照判断会让刚被降级的账号继续免配额。
	if user.Role == models.RoleAdmin {
		return nil
	}
	quota := quotaBytesForUser(user, setting)
	if quota <= 0 {
		return nil
	}
	used, err := usedBytesForUser(db, userID)
	if err != nil {
		return err
	}
	// 只挡「已经用满」。剩余额度的判断交给事务里的精确复核。
	//
	// 不要用 incomingBytes 做「本批一定放不下」的判断：那假设了原图大小
	// 就是最终占用，而实际落盘的往往是压缩后的 webp。实测 721KB 的 PNG
	// 会存成 268KB —— 拿 721KB 去比 500KB 的配额就会把「完全放得下」
	// 误判成超限，用户看着配额还剩一半却传不进去。
	if used >= quota {
		return ErrQuotaExceeded
	}
	return nil
}

// EnforceQuotaInTx 用落盘后的真实大小做精确复核，在写库事务内调用。
//
// 放在事务里而不是事前：SQLite 写事务串行化，「读已用 + 写入」在同一事务里
// 不会与并发的第二个上传交错，因此两个请求同时看到「还有额度」然后都写进去
// 是不可能的。这也是把判断放在事务内而不是入口处的关键理由。
func EnforceQuotaInTx(tx *gorm.DB, userID int, incomingBytes int64) error {
	if userID <= 0 {
		return nil
	}
	// 必须走事务自己的连接读 settings，不能调 settings.GetSettings()：
	// SQLite 的连接池被硬编码为 1 条（见 backend/database/pool.go），此刻这条
	// 连接已被本事务占用，再从池里申请连接会永久自锁（database/sql 在
	// context.Background() 下等连接没有超时）；而 settings.GetSettings() 会
	// 全程持有 settingsCache.mu，使 SecurityHeaders.buildCSP 把每一个请求
	// （包括静态资源）都一起堵死，整站表现为 Cloudflare 524。
	var setting models.Settings
	if err := tx.Where("id = ?", 1).First(&setting).Error; err != nil {
		return err
	}
	var user models.User
	if err := tx.Select("id", "role", "quota_bytes").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	// 角色以库内为准，不用会话里的快照：会话可能是提权/降权之前建立的，
	// 拿旧快照判断会让刚被降级的账号继续享受管理员的免配额待遇。
	if user.Role == models.RoleAdmin {
		return nil
	}
	quota := quotaBytesForUser(user, setting)
	if quota <= 0 {
		return nil
	}
	used, err := usedBytesForUser(tx, userID)
	if err != nil {
		return err
	}
	if incomingBytes < 0 {
		incomingBytes = 0
	}
	if used+incomingBytes > quota {
		return ErrQuotaExceeded
	}
	return nil
}

// quotaMessage 生成给终端用户看的说明，带上具体数字。// 「配额已满」这种笼统提示会让人以为是系统故障，写清 used/limit 才能自助排查。
func quotaMessage(used, quota int64) string {
	return "账号存储配额已用完（已用 " + shared.HumanBytes(used) + " / 上限 " + shared.HumanBytes(quota) + "），请删除部分图片或联系管理员调整配额"
}

// FailQuota 把配额超限统一转换成响应，带上具体数字便于用户自助排查。
func FailQuota(uc *uploads.UploadContext, db *gorm.DB, userID int) {
	if setting, err := settings.GetSettings(); err == nil {
		if user, e := loadQuotaUser(db, userID); e == nil {
			quota := quotaBytesForUser(user, setting)
			if used, e2 := usedBytesForUser(db, userID); e2 == nil {
				uc.Fail(http.StatusForbidden, "%s", quotaMessage(used, quota))
				return
			}
		}
	}
	uc.Fail(http.StatusForbidden, "账号存储配额已用完，请删除部分图片或联系管理员调整配额")
}

func loadQuotaUser(db *gorm.DB, userID int) (models.User, error) {
	var user models.User
	err := db.Select("id", "role", "quota_bytes").First(&user, userID).Error
	return user, err
}

// uploadHeadersBytes 汇总一批 multipart 文件头的声明大小。
//
// 用 Header.Size（服务端已解析出的 multipart 元数据）而不是实际读取后的
// 字节数：配额检查必须在落盘之前完成，此时还没有真实数据可数。
// 声明大小可能被压小（分块传输），所以这只是第一道闸；真正的兜底是
// 单文件上限（MaxFileSize）+ 上传后回写 file_size 时的复核。
func uploadHeadersBytes(files []*multipart.FileHeader) int64 {
	var total int64
	for _, f := range files {
		if f != nil && f.Size > 0 {
			total += f.Size
		}
	}
	return total
}

// EnforceQuotaForUpload 是上传入口的统一调用点：解析完文件头、落盘之前调用。
// 返回 false 表示已经写好响应，调用方直接 return。
func EnforceQuotaForUpload(c *gin.Context, uc *uploads.UploadContext, db *gorm.DB, files []*multipart.FileHeader) bool {
	err := EnforceUserQuota(c, db, c.GetInt("user_id"), uploadHeadersBytes(files))
	if err == nil {
		return true
	}
	if errors.Is(err, ErrQuotaExceeded) {
		FailQuota(uc, db, c.GetInt("user_id"))
		return false
	}
	// 查询失败不是用户的错，但也绝不能放行 —— 配额检查失效等于没有配额。
	log.Printf("配额校验失败（用户 %d）: %v", c.GetInt("user_id"), err)
	uc.Fail(http.StatusInternalServerError, "配额校验失败，请稍后重试")
	return false
}

// QuotaExceededShortMessage 返回适合放在单个文件条目里的短提示。
// 批量上传时每个失败文件都会带一条消息，太长会把结果列表撑爆。
func QuotaExceededShortMessage(c *gin.Context, db *gorm.DB) string {
	setting, err := settings.GetSettings()
	if err == nil {
		if user, e := loadQuotaUser(db, c.GetInt("user_id")); e == nil {
			quota := quotaBytesForUser(user, setting)
			if used, e2 := usedBytesForUser(db, c.GetInt("user_id")); e2 == nil {
				return "存储配额不足（已用 " + shared.HumanBytes(used) + " / 上限 " + shared.HumanBytes(quota) + "）"
			}
		}
	}
	return "存储配额不足"
}

// ---------------------------------------------------------------- 管理接口
// UpdateUserQuota 修改单个账号的上传配额。
//
// 权限要求：超管，或持有任一 setting:* 权限码的委派者。
// 用户明确要求「超管/有权限字段的可以改单个用户配额」，因此这里不新增
// 专用权限码，而是复用设置类的授权面 —— 配额本质是「系统级上传策略」，
// 和 max_file_size 属于同一类管控对象（那个走 setting:upload）。
func UpdateUserQuota(c *gin.Context) {
	db := database.GetDB().DB
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, result.Fail(400, "用户ID参数错误"))
		return
	}

	type quotaReq struct {
		// QuotaBytes 为 nil（字段缺省）= 清除单独配额，跟随系统默认；
		// 显式 0 = 不限；正数 = 该账号的绝对配额。
		QuotaBytes *int64 `json:"quota_bytes"`
	}
	var req quotaReq
	if !shared.BindJSON(c, &req) {
		return
	}

	if req.QuotaBytes != nil {
		if *req.QuotaBytes < 0 {
			c.JSON(http.StatusBadRequest, result.Fail(400, "配额不能为负数"))
			return
		}
		// 上界 1 PiB：避免误填天文数字让人以为「等于不限」，
		// 真正的不限请显式写 0。
		const maxQuota = int64(1) << 50
		if *req.QuotaBytes > maxQuota {
			c.JSON(http.StatusBadRequest, result.Fail(400, "配额超出允许范围（最大 1 PiB，不限请填 0）"))
			return
		}
	}

	var user models.User
	if err := db.Select("id", "role", "quota_bytes").First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, result.Fail(404, "用户不存在"))
			return
		}
		shared.InternalFailure(c, "查询用户失败", err)
		return
	}
	if user.ID == models.SuperAdminID {
		c.JSON(http.StatusBadRequest, result.Fail(400, "超级管理员不受配额限制，无需设置"))
		return
	}

	// 非超管不得修改管理员的配额。
	//
	// 理由同 ResetPassword：能改管理员配额的人可以把管理员的额度改成 0
	// 或极小值，让管理员在自己的站点上无法上传 —— 这是一种低成本的
	// 定向干扰。配额对管理员本身也不生效（见 EnforceUserQuota），
	// 所以这种修改除了骚扰没有任何正当用途。
	operator, ok := middlewares.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, result.Fail(401, "未登录"))
		return
	}
	if operator.ID != models.SuperAdminID && user.Role == models.RoleAdmin {
		c.JSON(http.StatusForbidden, result.Fail(403, "不能修改管理员的配额"))
		return
	}

	// gorm 的 Update 对 nil 值会跳过，因此「清除覆盖」必须显式写 NULL。
	if req.QuotaBytes == nil {
		if err := db.Model(&models.User{}).Where("id = ?", id).Update("quota_bytes", nil).Error; err != nil {
			shared.InternalFailure(c, "清除用户配额失败", err)
			return
		}
	} else if err := db.Model(&models.User{}).Where("id = ?", id).Update("quota_bytes", *req.QuotaBytes).Error; err != nil {
		shared.InternalFailure(c, "更新用户配额失败", err)
		return
	}

	setting, err := settings.GetSettings()
	if err != nil {
		shared.InternalFailure(c, "读取系统配额失败", err)
		return
	}
	effective := setting.DefaultUserQuota
	if req.QuotaBytes != nil {
		effective = *req.QuotaBytes
	}
	used, _ := usedBytesForUser(db, id)
	message := "配额已更新"
	if req.QuotaBytes == nil {
		message = "已恢复为跟随系统默认配额"
	}
	c.JSON(http.StatusOK, result.Success(message, map[string]any{
		"user_id":         id,
		"quota_bytes":     req.QuotaBytes,
		"effective_quota": effective,
		"used_bytes":      used,
	}))
}

// quotaSummaryForUser 生成用户列表里展示的配额摘要。
// 前端要同时显示「已用」和「上限」，以及这个上限是自定义还是继承来的。
//
// 管理员也返回摘要（unlimited=true）：前端需要统一的卡片布局，缺一块会让
// 超管卡片的行高、间距和普通用户对不齐。管理员不参与配额限制这件事，
// 由 unlimited 标记表达，而不是「不下发数据」。
func quotaSummaryForUser(db *gorm.DB, user models.User, s models.Settings) map[string]any {
	// 管理员与超管：显式标记为不限，但仍返回已用空间供展示。
	if user.Role == models.RoleAdmin || user.ID == models.SuperAdminID {
		used, err := usedBytesForUser(db, user.ID)
		if err != nil {
			log.Printf("统计用户[%d]已用空间失败: %v", user.ID, err)
			return map[string]any{"quota_bytes": 0, "used_bytes": nil, "quota_custom": false, "unlimited": true, "percent": 0}
		}
		return map[string]any{
			"quota_bytes":  0,
			"used_bytes":   used,
			"quota_custom": false,
			"unlimited":    true,
			"percent":      0,
			"admin_exempt": true,
		}
	}
	quota := quotaBytesForUser(user, s)
	used, err := usedBytesForUser(db, user.ID)
	if err != nil {
		// 统计失败不该让整个用户列表 500：把 used 标成未知，页面照常渲染。
		log.Printf("统计用户[%d]已用空间失败: %v", user.ID, err)
		return map[string]any{"quota_bytes": quota, "used_bytes": nil, "quota_custom": user.QuotaBytes != nil, "unlimited": quota <= 0, "percent": 0}
	}
	percent := 0.0
	if quota > 0 {
		percent = float64(used) / float64(quota) * 100
		if percent > 100 {
			percent = 100
		}
	}
	return map[string]any{
		"quota_bytes":  quota,
		"used_bytes":   used,
		"quota_custom": user.QuotaBytes != nil,
		"unlimited":    quota <= 0,
		"percent":      percent,
	}
}

// CanManageUserQuota 判断当前请求方是否有权管理配额。
// 供 GetUsers 决定是否下发按钮状态，避免前端各自硬编码。
func CanManageUserQuota(c *gin.Context) bool {
	user, ok := middlewares.GetCurrentUser(c)
	if !ok {
		return false
	}
	if user.ID == models.SuperAdminID {
		return true
	}
	for _, code := range user.Permission.Codes {
		if strings.HasPrefix(code, "setting:") {
			return true
		}
	}
	return false
}
