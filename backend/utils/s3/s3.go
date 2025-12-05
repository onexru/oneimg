package s3

import (
	"context"
	"fmt"

	"oneimg/backend/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// 创建S3客户端
func NewS3Client(setting models.Settings) (*s3.Client, error) {
	var (
		endpoint  string
		bucket    string
		accessKey string
		secretKey string
		region    = "auto" // R2使用auto区域
	)
	endpoint = setting.S3Endpoint
	bucket = setting.S3Bucket
	accessKey = setting.S3AccessKey
	secretKey = setting.S3SecretKey

	if setting.StorageType == "s3" {
		region = "us-east-1"
	}

	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("S3/R2密钥为空")
	}
	if bucket == "" || endpoint == "" {
		return nil, fmt.Errorf("S3/R2配置缺失 [bucket:%s, endpoint:%s]", bucket, endpoint)
	}

	// 创建AWS配置
	awsCfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(region),
		awsconfig.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               endpoint,
					HostnameImmutable: true,
				}, nil
			},
		)),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey,
			secretKey,
			"", // Token
		)),
	)

	if err != nil {
		return nil, fmt.Errorf("加载 AWS 配置失败: %w", err)
	}

	// 创建S3客户端
	client := s3.NewFromConfig(awsCfg)

	return client, err
}

func GetObject(client s3.Client, ctx context.Context, input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	return client.GetObject(ctx, input)
}

func DeleteObject(client s3.Client, ctx context.Context, input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
	return client.DeleteObject(ctx, input)
}
