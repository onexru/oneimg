package s3

import (
	"fmt"
	"net/url"
	"strings"

	"oneimg/backend/models"
	utilsBuckets "oneimg/backend/utils/buckets"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// 创建S3/R2兼容客户端
func NewS3Client(setting models.Settings, buckets models.Buckets) (*minio.Client, error) {
	var (
		endpoint  string
		bucket    string
		accessKey string
		secretKey string
		region    = "us-east-1"
		useSSL    = true
		host      string
		lookup    minio.BucketLookupType
	)

	switch buckets.Type {
	case "s3":
		storageConfig := utilsBuckets.ConvertToS3Bucket(buckets.Config)
		endpoint = storageConfig.S3Endpoint
		bucket = storageConfig.S3Bucket
		accessKey = storageConfig.S3AccessKey
		secretKey = storageConfig.S3SecretKey
		region = "us-east-1"
		lookup = minio.BucketLookupDNS

	case "r2":
		storageConfig := utilsBuckets.ConvertToR2Bucket(buckets.Config)
		endpoint = storageConfig.R2Endpoint
		bucket = storageConfig.R2Bucket
		accessKey = storageConfig.R2AccessKey
		secretKey = storageConfig.R2SecretKey
		region = "auto"
		lookup = minio.BucketLookupPath
	}

	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("S3/R2密钥为空")
	}
	if bucket == "" || endpoint == "" {
		return nil, fmt.Errorf("S3/R2配置缺失 [bucket:%s, endpoint:%s]", bucket, endpoint)
	}
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("解析 Endpoint 失败: %w", err)
	}

	host = parsedURL.Host
	if host == "" {
		path := parsedURL.Path
		if idx := strings.Index(path, "/"); idx != -1 {
			host = path[:idx]
		} else {
			host = path
		}
	}

	switch parsedURL.Scheme {
	case "http":
		useSSL = false
	case "https":
		useSSL = true
	}

	if buckets.Type == "s3" && bucket != "" {
		prefix := strings.ToLower(bucket) + "."
		if strings.HasPrefix(strings.ToLower(host), prefix) {
			host = host[len(bucket)+1:]
		}
	}

	client, err := minio.New(host, &minio.Options{
		Creds:        credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:       useSSL,
		Region:       region,
		BucketLookup: lookup,
	})

	if err != nil {
		return nil, fmt.Errorf("初始化 S3 客户端失败: %w", err)
	}

	return client, nil
}
