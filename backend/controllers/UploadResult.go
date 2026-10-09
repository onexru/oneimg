package controllers

import (
 "context"
 "fmt"
 "mime/multipart"
 "oneimg/backend/database"
 "oneimg/backend/interfaces"
 "oneimg/backend/models"
 "oneimg/backend/services"
 "oneimg/backend/utils/uploads"
 "github.com/gin-gonic/gin"
 "gorm.io/gorm"
)
func failedUploadResult(file *multipart.FileHeader,message string)interfaces.ImageUploadResult{
 return interfaces.ImageUploadResult{Success:false,FileName:file.Filename,Message:message}
}
func respondUploadBatch(uc *uploads.UploadContext,results []interfaces.ImageUploadResult,message string,targets int){
 count:=0;for _,r:=range results{if r.Success{count++}}
 status:=200;if count==0{status=400;message="所有文件上传失败"}else if count<len(results){message="部分文件上传成功，请查看逐项结果"}
 uc.Gin().JSON(status,gin.H{"code":status,"message":message,"data":map[string]any{"files":results,"count":count,"failed_count":len(results)-count,"sync_targets":targets}})
}
func chargeUploadQuota(tx *gorm.DB,bucket models.Buckets,result *interfaces.ImageUploadResult)error{
 size:=result.FileSize+result.ThumbnailSize
 if size<0{return fmt.Errorf("invalid upload size")}
 col:=database.UsageColumn(tx)
 query:=tx.Model(&models.Buckets{}).Where("id = ?",bucket.Id)
 if bucket.Type!="default" && bucket.Type!="telegram"{
  // subtraction form cannot overflow and includes equality at the boundary.
  query=query.Where("capacity = 0 OR ("+col+" <= capacity AND capacity - "+col+" >= ?)",size)
 }
 update:=query.UpdateColumn("usage",gorm.Expr(col+" + ?",size))
 if update.Error!=nil{return update.Error};if update.RowsAffected!=1{return fmt.Errorf("bucket quota exhausted")};return nil
}
func cleanupUnpublishedUpload(c *gin.Context,image models.Image,result *interfaces.ImageUploadResult){
 if image.Storage=="default"{cleanupLocalUpload(image);return}
 // No published Image exists, but materialize a retryable deletion manifest so
 // an SDK failure cannot orphan already stored objects invisibly.
 db:=database.GetDB().DB
 image.Id=0;image.Deleting=true;image.FolderId=0
 err:=db.Transaction(func(tx *gorm.DB)error{
  if err:=tx.Create(&image).Error;err!=nil{return err}
  replica:=models.ImageStorage{ImageID:image.Id,BucketID:image.BucketId,Storage:image.Storage,Status:models.ImageStorageStatusDeleting,URL:image.Url,Thumbnail:image.Thumbnail,FileSize:result.FileSize,ThumbnailSize:result.ThumbnailSize,Metadata:map[string]any{"delete_charged":false}}
  return tx.Create(&replica).Error
 })
 if err==nil { if services.DeleteImageReplicas(context.Background(),image)==nil { _=db.Delete(&image).Error } }
}
