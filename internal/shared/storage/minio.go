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

	bucket := config.AppConfig.MinioBucket
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("failed to check MinIO bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("failed to create MinIO bucket: %w", err)
		}
	}

	return nil
}

func requireClient() error {
	if Client == nil {
		return fmt.Errorf("MinIO client not initialized (check MINIO_* env vars)")
	}
	return nil
}

func Upload(objectPath string, reader io.Reader, size int64, contentType string) error {
	if err := requireClient(); err != nil {
		return err
	}
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
	if err := requireClient(); err != nil {
		return nil, nil, err
	}
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
	if err := requireClient(); err != nil {
		return nil, nil, err
	}
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

func DeleteObject(objectPath string) error {
	if err := requireClient(); err != nil {
		return err
	}
	ctx := context.Background()
	bucket := config.AppConfig.MinioBucket

	return Client.RemoveObject(ctx, bucket, objectPath, minio.RemoveObjectOptions{})
}
