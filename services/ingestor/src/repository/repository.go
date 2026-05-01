package repository

import (
	"bytes"
	"context"
	"mime/multipart"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
)

var (
	RedisClient *redis.Client
	MinioClient *minio.Client
	BucketName  string
)

func InitRedis(addr string) {
	RedisClient = redis.NewClient(&redis.Options{
		Addr: addr,
	})
}

func InitMinio(endpoint, accessKey, secretKey string) {
	var err error
	MinioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
		Region: "garage",
	})
	if err != nil {
		panic(err)
	}

	BucketName = os.Getenv("BUCKET_NAME")
	if BucketName == "" {
		BucketName = "truckguard-data"
	}

	// Ensure bucket exists
	ctx := context.Background()
	exists, err := MinioClient.BucketExists(ctx, BucketName)
	if err != nil || !exists {
		MinioClient.MakeBucket(ctx, BucketName, minio.MakeBucketOptions{})
	}
}

// UploadToS3 uploads raw bytes to S3
func UploadToS3(key string, data []byte) error {
	ctx := context.Background()
	_, err := MinioClient.PutObject(
		ctx,
		BucketName,
		key,
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/json"},
	)
	return err
}

// UploadFileToS3 uploads a multipart file to S3
func UploadFileToS3(file *multipart.FileHeader, key string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	ctx := context.Background()
	_, err = MinioClient.PutObject(
		ctx,
		BucketName,
		key,
		src,
		file.Size,
		minio.PutObjectOptions{ContentType: file.Header.Get("Content-Type")},
	)
	return err
}

// PublishToStream publishes an event to Valkey Stream
func PublishToStream(stream string, data string) error {
	ctx := context.Background()
	return RedisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]interface{}{
			"data": data,
		},
	}).Err()
}
