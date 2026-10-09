package controllers

import (
 "github.com/gin-gonic/gin"
 "github.com/google/uuid"
 "gorm.io/gorm"
 "oneimg/backend/models"
)

func validGuestUUID(value string) bool {
 parsed,err:=uuid.Parse(value)
 return err==nil && parsed!=uuid.Nil && parsed.Version()==4 && parsed.String()==value
}
func scopeGuestImages(query *gorm.DB, userID int, fingerprint string) *gorm.DB {
 if userID >= 0 || !validGuestUUID(fingerprint) { return query.Where("1 = 0") }
 return query.Where("images.user_id = ? AND images.uuid = ?",userID,fingerprint)
}
func guestOwnsImage(c *gin.Context, image models.Image) bool {
 fingerprint:=CurrentOwnerKey(c)
 return c.GetInt("user_role")==models.RoleGuest && c.GetInt("user_id")<0 && image.UserId==c.GetInt("user_id") && validGuestUUID(fingerprint) && image.UUID==fingerprint
}
// EXISTS avoids join multiplicity and PostgreSQL DISTINCT/ORDER BY coupling.
func filterImageTags(query *gorm.DB, ids []int, untagged bool) *gorm.DB {
 exists:="EXISTS (SELECT 1 FROM image_to_tags WHERE image_to_tags.image_id = images.id AND image_to_tags.tag_id IN ?)"
 noTags:="NOT EXISTS (SELECT 1 FROM image_to_tags WHERE image_to_tags.image_id = images.id)"
 if untagged && len(ids)>0 { return query.Where("("+noTags+" OR "+exists+")",ids) }
 if untagged { return query.Where(noTags) }
 if len(ids)>0 { return query.Where(exists,ids) }
 return query
}
