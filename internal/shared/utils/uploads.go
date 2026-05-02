package utils

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func sanitizeUploadFolder(folder string) (string, error) {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return "", fmt.Errorf("upload folder is required")
	}

	clean := filepath.Clean(folder)
	if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("invalid upload folder")
	}

	return clean, nil
}

func getFileExtension(header *multipart.FileHeader) string {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != "" {
		return ext
	}

	contentType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	switch {
	case strings.Contains(contentType, "png"):
		return ".png"
	case strings.Contains(contentType, "jpeg"), strings.Contains(contentType, "jpg"):
		return ".jpg"
	case strings.Contains(contentType, "gif"):
		return ".gif"
	case strings.Contains(contentType, "webp"):
		return ".webp"
	case strings.Contains(contentType, "svg"):
		return ".svg"
	default:
		return ".png"
	}
}

func getContentTypeForFile(path string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		if contentType := mime.TypeByExtension(ext); contentType != "" {
			return contentType
		}
	}

	if len(data) > 0 {
		return http.DetectContentType(data)
	}

	return "application/octet-stream"
}

func SaveUploadedImage(header *multipart.FileHeader, folder string) (string, error) {
	if header == nil {
		return "", fmt.Errorf("no file provided")
	}

	cleanFolder, err := sanitizeUploadFolder(folder)
	if err != nil {
		return "", err
	}

	contentType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	if contentType == "" {
		contentType = strings.ToLower(mime.TypeByExtension(filepath.Ext(header.Filename)))
	}
	if contentType != "" && !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("only image files are allowed")
	}

	src, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	uploadDir := filepath.Join("uploads", cleanFolder)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	ext := getFileExtension(header)
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	savePath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		return "", fmt.Errorf("failed to create uploaded file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to save uploaded file: %w", err)
	}

	return "/" + filepath.ToSlash(filepath.Join("uploads", cleanFolder, filename)), nil
}

func ReadLatestUploadedImage(folder string) ([]byte, string, error) {
	cleanFolder, err := sanitizeUploadFolder(folder)
	if err != nil {
		return nil, "", err
	}

	uploadDir := filepath.Join("uploads", cleanFolder)
	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		return nil, "", fmt.Errorf("upload directory not found")
	}

	var latestPath string
	var latestModTime time.Time
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if latestPath == "" || info.ModTime().After(latestModTime) {
			latestPath = filepath.Join(uploadDir, entry.Name())
			latestModTime = info.ModTime()
		}
	}

	if latestPath == "" {
		return nil, "", fmt.Errorf("no uploaded image found")
	}

	data, err := os.ReadFile(latestPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read uploaded image: %w", err)
	}

	return data, getContentTypeForFile(latestPath, data), nil
}
