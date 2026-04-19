package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"studsphere/backend/internal/shared/config"
	"studsphere/backend/internal/shared/logger"
)

var minioClient *minio.Client

func InitMinIO() error {
	if minioClient != nil {
		return nil // Already initialized
	}

	cfg := config.AppConfig
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	// Use API endpoint if configured, otherwise use regular endpoint
	endpoint := cfg.MinIOEndpoint
	if cfg.MinIOAPIEndpoint != "" {
		endpoint = cfg.MinIOAPIEndpoint
		logger.Info("Using MinIO API endpoint", "endpoint", endpoint)
	} else {
		logger.Warn("No MinIO API endpoint configured, using console endpoint (may not work for S3 operations)")
	}

	var err error
	minioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOUseSSL,
	})
	if err != nil {
		return fmt.Errorf("failed to create MinIO client: %w", err)
	}

	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, cfg.MinIOBucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = minioClient.MakeBucket(ctx, cfg.MinIOBucket, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		logger.Info("MinIO bucket created", "bucket", cfg.MinIOBucket)
	}

	// Set bucket policy to public read
	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {"AWS": "*"},
				"Action": ["s3:GetObject"],
				"Resource": ["arn:aws:s3:::%s/*"]
			}
		]
	}`, cfg.MinIOBucket)

	err = minioClient.SetBucketPolicy(ctx, cfg.MinIOBucket, policy)
	if err != nil {
		logger.Warn("Failed to set bucket policy", "error", err)
	}

	logger.Info("MinIO initialized", "endpoint", endpoint, "bucket", cfg.MinIOBucket)
	return nil
}

func GetMinIOClient() *minio.Client {
	return minioClient
}

func UploadImageToMinIO(file multipart.File, header *multipart.FileHeader, folder string) (string, error) {
	if minioClient == nil {
		return "", fmt.Errorf("MinIO client not initialized")
	}

	cfg := config.AppConfig
	if cfg == nil {
		return "", fmt.Errorf("config not loaded")
	}

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".png" // default extension
	}

	filename := fmt.Sprintf("%s/%d%s", folder, time.Now().UnixNano(), ext)
	objectName := strings.TrimPrefix(filename, "/")

	// Read file content
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Upload to MinIO
	ctx := context.Background()
	reader := bytes.NewReader(fileContent)

	_, err = minioClient.PutObject(ctx, cfg.MinIOBucket, objectName, reader, int64(len(fileContent)), minio.PutObjectOptions{
		ContentType: getContentType(ext),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to MinIO: %w", err)
	}

	// Generate public URL
	var url string
	if cfg.MinIOUseSSL {
		url = fmt.Sprintf("https://%s/%s/%s", cfg.MinIOEndpoint, cfg.MinIOBucket, objectName)
	} else {
		url = fmt.Sprintf("http://%s/%s/%s", cfg.MinIOEndpoint, cfg.MinIOBucket, objectName)
	}

	logger.Info("Image uploaded to MinIO", "url", url)
	return url, nil
}

func UploadImageBytesToMinIO(imageData []byte, filename, folder string) (string, error) {
	if minioClient == nil {
		return "", fmt.Errorf("MinIO client not initialized")
	}

	cfg := config.AppConfig
	if cfg == nil {
		return "", fmt.Errorf("config not loaded")
	}

	// Generate unique filename
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".png"
	}

	objectName := fmt.Sprintf("%s/%d%s", strings.TrimPrefix(folder, "/"), time.Now().UnixNano(), ext)

	// Upload to MinIO
	ctx := context.Background()
	reader := bytes.NewReader(imageData)

	_, err := minioClient.PutObject(ctx, cfg.MinIOBucket, objectName, reader, int64(len(imageData)), minio.PutObjectOptions{
		ContentType: getContentType(ext),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to MinIO: %w", err)
	}

	// Generate public URL
	var url string
	if cfg.MinIOUseSSL {
		url = fmt.Sprintf("https://%s/%s/%s", cfg.MinIOEndpoint, cfg.MinIOBucket, objectName)
	} else {
		url = fmt.Sprintf("http://%s/%s/%s", cfg.MinIOEndpoint, cfg.MinIOBucket, objectName)
	}

	logger.Info("Image uploaded to MinIO", "url", url)
	return url, nil
}

func GetImageURL(objectName string) string {
	cfg := config.AppConfig
	if cfg == nil {
		return ""
	}

	if cfg.MinIOUseSSL {
		return fmt.Sprintf("https://%s/%s/%s", cfg.MinIOEndpoint, cfg.MinIOBucket, objectName)
	}
	return fmt.Sprintf("http://%s/%s/%s", cfg.MinIOEndpoint, cfg.MinIOBucket, objectName)
}

// GetLogoFromMinIO retrieves the StudSphere logo from MinIO
func GetLogoFromMinIO() ([]byte, string, error) {
	if minioClient == nil {
		if err := InitMinIO(); err != nil {
			return nil, "", fmt.Errorf("failed to initialize MinIO: %w", err)
		}
	}

	cfg := config.AppConfig
	if cfg == nil {
		return nil, "", fmt.Errorf("config not loaded")
	}

	// Try the actual filename first, then fallback to logo name
	possibleNames := []string{"logos/studsphere.png", "studsphere.png", "logo.png"}

	for _, objectName := range possibleNames {
		ctx := context.Background()
		object, err := minioClient.GetObject(ctx, cfg.MinIOBucket, objectName, minio.GetObjectOptions{})
		if err != nil {
			continue
		}

		// Read the object
		data, err := io.ReadAll(object)
		object.Close()
		if err != nil {
			continue
		}

		// Determine content type
		contentType := getContentType(objectName)
		if contentType == "image/png" && len(data) > 0 {
			return data, contentType, nil
		}
	}

	return nil, "", fmt.Errorf("logo not found in MinIO")
}

func DeleteImageFromMinIO(objectName string) error {
	if minioClient == nil {
		return fmt.Errorf("MinIO client not initialized")
	}

	cfg := config.AppConfig
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	ctx := context.Background()
	err := minioClient.RemoveObject(ctx, cfg.MinIOBucket, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete from MinIO: %w", err)
	}

	logger.Info("Image deleted from MinIO", "object", objectName)
	return nil
}

func getContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "image/png"
	}
}
