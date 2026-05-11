package repository

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	StorageClient *minio.Client
	StorageBucket string
)

func InitStorage(endpoint, accessKey, secretKey, bucket string) {
	var err error
	StorageClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
		Region: "garage",
	})
	if err != nil {
		panic(err)
	}
	StorageBucket = bucket
}

func GetRawObject(key string) ([]byte, string, error) {
	obj, err := StorageClient.GetObject(context.Background(), StorageBucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", err
	}
	defer obj.Close()

	info, err := obj.Stat()
	if err != nil {
		return nil, "", err
	}

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, "", err
	}

	contentType := info.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return data, contentType, nil
}
