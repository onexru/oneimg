package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"oneimg/backend/models"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const MaxGalleryFolders = 200
const MaxFolderMoveImages = 100

// GalleryFolderError contains only safe client-facing details.
type GalleryFolderError struct {
	Status  int
	Code    string
	Message string
}

func (e *GalleryFolderError) Error() string { return e.Message }
func folderError(status int, code, message string) error {
	return &GalleryFolderError{Status: status, Code: code, Message: message}
}
func folderDeletingError() error {
	return folderError(409, "folder_deleting", "文件夹正在删除，不能添加、移动或重命名")
}
func folderNotFound() error { return folderError(404, "folder_not_found", "文件夹不存在") }

func normalizeFolderName(raw string) (string, string, error) {
	invalid := func() (string, string, error) {
		return "", "", folderError(400, "invalid_folder_name", "文件夹名称须为 1–64 个字符，不能包含路径分隔符、控制字符或仅由点组成")
	}
	if !utf8.ValidString(raw) {
		return invalid()
	}
	for _, r := range raw {
		if r == '/' || r == '\\' || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return invalid()
		}
	}
	name := norm.NFC.String(strings.TrimSpace(raw))
	if n := utf8.RuneCountInString(name); n == 0 || n > 64 || strings.Trim(name, ".") == "" {
		return invalid()
	}
	// Hashing the folded form makes MySQL, SQLite and PostgreSQL agree on
	// uniqueness, including Unicode folds such as Straße / STRASSE.
	sum := sha256.Sum256([]byte(norm.NFC.String(cases.Fold().String(name))))
	return name, hex.EncodeToString(sum[:]), nil
}

func normalizeFolderDescription(raw string) (string, error) {
	invalid := func() (string, error) {
		return "", folderError(400, "invalid_folder_description", "文件夹描述最多 500 个字符，不能包含不可见控制字符")
	}
	if !utf8.ValidString(raw) {
		return invalid()
	}
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	for _, r := range raw {
		if (unicode.IsControl(r) && r != '\n' && r != '\t') || unicode.Is(unicode.Cf, r) {
			return invalid()
		}
	}
	text := strings.TrimSpace(raw)
	if utf8.RuneCountInString(text) > 500 {
		return invalid()
	}
	return text, nil
}

// Folder mutation lock order is owner -> folder -> image/task. All writers of
// nonzero folder_id (including publication) must take this same owner lock.
// This serializes the per-account cap and fences deletion across processes on
// PostgreSQL; no network/storage operation is performed while holding it.
func lockFolderOwner(tx *gorm.DB, ownerID int) error {
	return checkFolderOwner(tx, ownerID, "UPDATE")
}
func checkFolderOwner(db *gorm.DB, ownerID int, strength string) error {
	if ownerID <= 0 {
		return folderError(403, "folders_unavailable", "请登录后使用文件夹")
	}
	var user models.User
	q := db
	if strength != "" {
		q = q.Clauses(clause.Locking{Strength: strength})
	}
	if err := q.First(&user, ownerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return folderError(403, "folders_unavailable", "用户不存在或已被删除")
		}
		return err
	}
	if user.Role == models.RoleGuest {
		return folderError(403, "folders_unavailable", "游客不能使用文件夹")
	}
	return nil
}
func ownedFolder(db *gorm.DB, ownerID, folderID int, lock bool) (models.Folder, error) {
	var folder models.Folder
	q := db
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := q.Where("id = ? AND user_id = ?", folderID, ownerID).First(&folder).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = folderNotFound()
	}
	return folder, err
}

// ValidateFolderSelection runs before accepting upload bytes. Zero deliberately
// remains valid for legacy and guest uploads. Publication rechecks under locks.
func ValidateFolderSelection(db *gorm.DB, ownerID, role, folderID int) error {
	if folderID < 0 {
		return folderError(400, "invalid_folder_id", "文件夹ID无效")
	}
	if folderID == 0 {
		return nil
	}
	if role == models.RoleGuest {
		return folderError(403, "folders_unavailable", "游客不能使用文件夹")
	}
	if err := checkFolderOwner(db, ownerID, ""); err != nil {
		return err
	}
	folder, err := ownedFolder(db, ownerID, folderID, false)
	if err == nil && folder.Deleting {
		return folderDeletingError()
	}
	return err
}

// ValidateFolderRead allows the owner to inspect partially deleted folders;
// write/upload validation deliberately rejects those same folders.
func ValidateFolderRead(db *gorm.DB, ownerID, role, folderID int) error {
	if folderID == 0 {
		return nil
	}
	if folderID < 0 {
		return folderError(400, "invalid_folder_id", "文件夹ID无效")
	}
	if role == models.RoleGuest {
		return folderError(403, "folders_unavailable", "游客不能使用文件夹")
	}
	if err := checkFolderOwner(db, ownerID, ""); err != nil {
		return err
	}
	_, err := ownedFolder(db, ownerID, folderID, false)
	return err
}

// ResolvePublicationFolder must be called inside the Image insert transaction,
// after prevalidation. A folder removed while bytes were in flight becomes
// unfiled; ownership mismatches do NOT silently succeed. Never use a path here.
func ResolvePublicationFolder(tx *gorm.DB, ownerID, folderID int) (int, error) {
	if folderID == 0 {
		return 0, nil
	}
	if folderID < 0 {
		return 0, folderError(400, "invalid_folder_id", "文件夹ID无效")
	}
	if err := lockFolderOwner(tx, ownerID); err != nil {
		return 0, err
	}
	var folder models.Folder
	err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).First(&folder, folderID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if folder.UserID != ownerID {
		return 0, folderNotFound()
	}
	if folder.Deleting {
		return 0, folderDeletingError()
	}
	if folder.DeletedAt.Valid {
		return 0, nil
	}
	return folder.ID, nil
}

type GalleryFolderResponse struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ImageCount  int64     `json:"image_count"`
	Deleting    bool      `json:"deleting"`
	CreatedAt   time.Time `json:"created_at"`
}
type GalleryFolderList struct {
	Folders      []GalleryFolderResponse `json:"folders"`
	Total        int                     `json:"total"`
	UnfiledCount int64                   `json:"unfiled_count"`
}

func folderResponse(db *gorm.DB, folder models.Folder) (GalleryFolderResponse, error) {
	r := GalleryFolderResponse{ID: folder.ID, Name: folder.Name, Description: folder.Description, CreatedAt: folder.CreatedAt, Deleting: folder.Deleting}
	err := db.Model(&models.Image{}).Where("user_id = ? AND folder_id = ? AND deleting = ?", folder.UserID, folder.ID, false).Count(&r.ImageCount).Error
	return r, err
}
func ListGalleryFolders(db *gorm.DB, ownerID int) (GalleryFolderList, error) {
	r := GalleryFolderList{Folders: []GalleryFolderResponse{}}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := checkFolderOwner(tx, ownerID, "SHARE"); err != nil {
			return err
		}
		if err := tx.Model(&models.Folder{}).
			Select("folders.id, folders.name, folders.description, folders.created_at, folders.deleting, COUNT(images.id) AS image_count").
			Joins("LEFT JOIN images ON images.folder_id = folders.id AND images.user_id = folders.user_id AND images.deleting = ?", false).
			Where("folders.user_id = ?", ownerID).
			Group("folders.id, folders.name, folders.description, folders.created_at, folders.deleting").Order("folders.id ASC").Scan(&r.Folders).Error; err != nil {
			return err
		}
		r.Total = len(r.Folders)
		return tx.Model(&models.Image{}).Where("user_id = ? AND folder_id = 0 AND deleting = ?", ownerID, false).Count(&r.UnfiledCount).Error
	})
	return r, err
}
func CreateGalleryFolder(db *gorm.DB, ownerID int, rawName string, descriptions ...string) (GalleryFolderResponse, error) {
	name, key, err := normalizeFolderName(rawName)
	if err != nil {
		return GalleryFolderResponse{}, err
	}
	var r GalleryFolderResponse
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := lockFolderOwner(tx, ownerID); err != nil {
			return err
		}
		if err := ensureFolderNameAvailable(tx, ownerID, key, 0); err != nil {
			return err
		}
		var total int64
		if err := tx.Model(&models.Folder{}).Where("user_id = ?", ownerID).Count(&total).Error; err != nil {
			return err
		}
		if total >= MaxGalleryFolders {
			return folderError(409, "folder_limit", "每个账户最多创建 200 个文件夹")
		}
		description := ""
		if len(descriptions) > 0 {
			var err error
			description, err = normalizeFolderDescription(descriptions[0])
			if err != nil {
				return err
			}
		}
		folder := models.Folder{UserID: ownerID, Name: name, NameKey: key, Description: description}
		if err := tx.Create(&folder).Error; err != nil {
			return err
		}
		r = GalleryFolderResponse{ID: folder.ID, Name: folder.Name, Description: folder.Description, CreatedAt: folder.CreatedAt, Deleting: folder.Deleting}
		return nil
	})
	return r, err
}
func ensureFolderNameAvailable(tx *gorm.DB, ownerID int, key string, exceptID int) error {
	var count int64
	if err := tx.Model(&models.Folder{}).Where("user_id = ? AND name_key = ? AND id <> ?", ownerID, key, exceptID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return folderError(409, "folder_name_exists", "已存在同名文件夹")
	}
	return nil
}
func RenameGalleryFolder(db *gorm.DB, ownerID, folderID int, rawName string, descriptions ...string) (GalleryFolderResponse, error) {
	name, key, err := normalizeFolderName(rawName)
	if err != nil {
		return GalleryFolderResponse{}, err
	}
	var r GalleryFolderResponse
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := lockFolderOwner(tx, ownerID); err != nil {
			return err
		}
		folder, err := ownedFolder(tx, ownerID, folderID, true)
		if err != nil {
			return err
		}
		if folder.Deleting {
			return folderDeletingError()
		}
		if err := ensureFolderNameAvailable(tx, ownerID, key, folderID); err != nil {
			return err
		}
		fields := map[string]any{"name": name, "name_key": key}
		if len(descriptions) > 0 {
			description, err := normalizeFolderDescription(descriptions[0])
			if err != nil {
				return err
			}
			fields["description"] = description
			folder.Description = description
		}
		if err := tx.Model(&folder).Updates(fields).Error; err != nil {
			return err
		}
		folder.Name = name
		r, err = folderResponse(tx, folder)
		return err
	})
	return r, err
}

// DeleteGalleryFolder changes metadata only: no byte, replica, quota, tag,
// direct-task state or source-key deletion is permitted here.
func DeleteGalleryFolder(db *gorm.DB, ownerID, folderID int) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := lockFolderOwner(tx, ownerID); err != nil {
			return err
		}
		folder, err := ownedFolder(tx, ownerID, folderID, true)
		if err != nil {
			return err
		}
		if folder.Deleting {
			return folderDeletingError()
		}
		// Task before image also agrees with thumbnail publication's row order.
		if err := tx.Model(&models.DirectUploadTask{}).Where("owner_id = ? AND folder_id = ?", ownerID, folderID).Update("folder_id", 0).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Image{}).Where("user_id = ? AND folder_id = ?", ownerID, folderID).Update("folder_id", 0).Error; err != nil {
			return err
		}
		return retireGalleryFolder(tx, &folder)
	})
}

// Keep deletion tombstones for stale publications, but release active name uniqueness.
func retireGalleryFolder(tx *gorm.DB, folder *models.Folder) error {
	if err := tx.Model(folder).Update("name_key", fmt.Sprintf("deleted:%d", folder.ID)).Error; err != nil {
		return err
	}
	return tx.Delete(folder).Error
}

func MoveImagesToGalleryFolder(db *gorm.DB, ownerID int, imageIDs []int, folderID int) (int64, error) {
	if len(imageIDs) == 0 || len(imageIDs) > MaxFolderMoveImages || folderID < 0 {
		return 0, folderError(400, "invalid_folder_move", "请选择 1–100 张图片及有效的目标文件夹")
	}
	seen := make(map[int]bool, len(imageIDs))
	ids := make([]int, 0, len(imageIDs))
	for _, id := range imageIDs {
		if id <= 0 {
			return 0, folderError(400, "invalid_image_id", "图片ID无效")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	var moved int64
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := lockFolderOwner(tx, ownerID); err != nil {
			return err
		}
		if folderID != 0 {
			folder, err := ownedFolder(tx, ownerID, folderID, true)
			if err != nil {
				return err
			}
			if folder.Deleting {
				return folderDeletingError()
			}
		}
		var images []models.Image
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ? AND user_id = ? AND deleting = ?", ids, ownerID, false).Order("id ASC").Find(&images).Error; err != nil {
			return err
		}
		// Validate all before changing any. Admins have no cross-account bypass.
		if len(images) != len(ids) {
			return folderError(404, "image_not_found", "图片不存在或不属于当前账户")
		}
		var deleting int64
		if err := tx.Model(&models.Folder{}).Where("user_id = ? AND deleting = ? AND id IN (SELECT folder_id FROM images WHERE id IN ?)", ownerID, true, ids).Count(&deleting).Error; err != nil {
			return err
		}
		if deleting > 0 {
			return folderDeletingError()
		}
		q := tx.Model(&models.Image{}).Where("id IN ? AND user_id = ? AND folder_id <> ?", ids, ownerID, folderID).Update("folder_id", folderID)
		moved = q.RowsAffected
		return q.Error
	})
	if err != nil {
		return 0, err
	}
	return moved, nil
}
