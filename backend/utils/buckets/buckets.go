package buckets

// 存储配置转换

import (
	"oneimg/backend/models"
	"oneimg/backend/utils/secureconfig"
)

// ConvertToS3Bucket 将map转换为S3Bucket
func ConvertToS3Bucket(config map[string]any) models.S3Bucket {
	return models.S3Bucket{
		S3Endpoint:  secureconfig.GetString(config, "s3_endpoint"),
		S3AccessKey: secureconfig.GetString(config, "s3_access_key"),
		S3SecretKey: secureconfig.GetString(config, "s3_secret_key"),
		S3Bucket:    secureconfig.GetString(config, "s3_bucket"),
	}
}

// ConvertToR2Bucket 将map转换为R2Bucket
func ConvertToR2Bucket(config map[string]any) models.R2Bucket {
	return models.R2Bucket{
		R2Endpoint:  secureconfig.GetString(config, "r2_endpoint"),
		R2AccessKey: secureconfig.GetString(config, "r2_access_key"),
		R2SecretKey: secureconfig.GetString(config, "r2_secret_key"),
		R2Bucket:    secureconfig.GetString(config, "r2_bucket"),
	}
}

// ConvertToFTPBucket 将map转换为FTPBucket
func ConvertToFTPBucket(config map[string]any) models.FTPBucket {
	return models.FTPBucket{
		FTPHost: secureconfig.GetString(config, "ftp_host"),
		FTPUser: secureconfig.GetString(config, "ftp_user"),
		FTPPass: secureconfig.GetString(config, "ftp_pass"),
		FTPPort: secureconfig.GetInt(config, "ftp_port"),
	}
}

// ConvertToWebDavBucket 将map转换为WebDavBucket
func ConvertToWebDavBucket(config map[string]any) models.WebDavBucket {
	return models.WebDavBucket{
		WebdavURL:  secureconfig.GetString(config, "webdav_url"),
		WebdavUser: secureconfig.GetString(config, "webdav_user"),
		WebdavPass: secureconfig.GetString(config, "webdav_pass"),
	}
}

// ConvertToTelegramBucket 将map转换为TelegramBucket
func ConvertToTelegramBucket(config map[string]any) models.TelegramBucket {
	return models.TelegramBucket{
		TGBotToken:  secureconfig.GetString(config, "tg_bot_token"),
		TGReceivers: secureconfig.GetString(config, "tg_receivers"),
	}
}

// 反转

// S3BucketToMap 将S3Bucket转换为map
func S3BucketToMap(s3 models.S3Bucket) map[string]any {
	return map[string]any{
		"s3_endpoint":   s3.S3Endpoint,
		"s3_access_key": s3.S3AccessKey,
		"s3_secret_key": s3.S3SecretKey,
		"s3_bucket":     s3.S3Bucket,
	}
}

// R2BucketToMap 将R2Bucket转换为map
func R2BucketToMap(r2 models.R2Bucket) map[string]any {
	return map[string]any{
		"r2_endpoint":   r2.R2Endpoint,
		"r2_access_key": r2.R2AccessKey,
		"r2_secret_key": r2.R2SecretKey,
		"r2_bucket":     r2.R2Bucket,
	}
}

// FTPBucketToMap 将FTPBucket转换为map
func FTPBucketToMap(ftp models.FTPBucket) map[string]any {
	return map[string]any{
		"ftp_host": ftp.FTPHost,
		"ftp_user": ftp.FTPUser,
		"ftp_pass": ftp.FTPPass,
		"ftp_port": ftp.FTPPort,
	}
}

// WebDavBucketToMap 将WebDavBucket转换为map
func WebDavBucketToMap(wd models.WebDavBucket) map[string]any {
	return map[string]any{
		"webdav_url":  wd.WebdavURL,
		"webdav_user": wd.WebdavUser,
		"webdav_pass": wd.WebdavPass,
	}
}

// TelegramBucketToMap 将TelegramBucket转换为map
func TelegramBucketToMap(tg models.TelegramBucket) map[string]any {
	return map[string]any{
		"tg_bot_token": tg.TGBotToken,
		"tg_receivers": tg.TGReceivers,
	}
}
