package controllers

import "oneimg/backend/utils/images"

type downloadedImageInfo struct { MIME string }
func inspectDownloadedImage(data []byte)(downloadedImageInfo,error){
 _,actual,err:=images.ValidateDirectImage(data,"")
 return downloadedImageInfo{MIME:actual},err
}
