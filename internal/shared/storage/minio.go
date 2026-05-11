package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"studsphere/backend/internal/shared/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var Client *minio.Client

func Init() error {
	cfg := config.AppConfig
	if cfg.MinioEndpoint == "" || cfg.MinioAccessKey == "" || cfg.MinioSecretKey == "" {
		return fmt.Errorf("MinIO configuration incomplete")
	}

	client, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		return fmt.Errorf("failed to create MinIO client: %w", err)
	}

	Client = client
	return nil
}

func Upload(objectPath string, reader io.Reader, size int64, contentType string) error {
	ctx := context.Background()
	bucket := config.AppConfig.MinioBucket

	_, err := Client.PutObject(ctx, bucket, objectPath, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("failed to upload to MinIO: %w", err)
	}
	return nil
}

type ObjectInfo struct {
	Key          string
	LastModified time.Time
	ContentType  string
}

func Get(objectPath string) (io.Reader, *ObjectInfo, error) {
	ctx := context.Background()
	bucket := config.AppConfig.MinioBucket

	obj, err := Client.GetObject(ctx, bucket, objectPath, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get object from MinIO: %w", err)
	}

	stat, err := obj.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("object not found in MinIO: %w", err)
	}

	info := &ObjectInfo{
		Key:          stat.Key,
		LastModified: stat.LastModified,
		ContentType:  stat.ContentType,
	}

	return obj, info, nil
}

func GetLatest(prefix string) (io.Reader, *ObjectInfo, error) {
	ctx := context.Background()
	bucket := config.AppConfig.MinioBucket

	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}

	var objects []minio.ObjectInfo
	for obj := range Client.ListObjects(ctx, bucket, opts) {
		if obj.Err != nil {
			continue
		}
		objects = append(objects, obj)
	}

	if len(objects) == 0 {
		return nil, nil, fmt.Errorf("no objects found in prefix: %s", prefix)
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.After(objects[j].LastModified)
	})

	latest := objects[0]
	obj, err := Client.GetObject(ctx, bucket, latest.Key, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, err
	}

	info := &ObjectInfo{
		Key:          latest.Key,
		LastModified: latest.LastModified,
		ContentType:  latest.ContentType,
	}

	return obj, info, nil
}

func UploadBytes(objectPath string, data []byte, contentType string) error {
	return Upload(objectPath, bytes.NewReader(data), int64(len(data)), contentType)
}
