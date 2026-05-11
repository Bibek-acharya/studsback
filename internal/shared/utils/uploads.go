package utils

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"studsphere/backend/internal/shared/storage"
)

var allowedDocumentTypes = map[string]bool{
	"application/pdf":           true,
	"application/msword":        true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel":  true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
}

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

func SaveUploadedDocument(header *multipart.FileHeader, folder string) (string, error) {
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
	if contentType != "" && !allowedDocumentTypes[contentType] && !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("only PDF, DOC, DOCX, XLS, XLSX files are allowed")
	}

	src, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return "", fmt.Errorf("failed to read uploaded file: %w", err)
	}

	ext := getFileExtension(header)
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)

	if contentType == "" {
		contentType = getContentTypeForFile(header.Filename, data)
	}

	if err := storage.UploadBytes(cleanFolder+"/"+filename, data, contentType); err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	return "/uploads/" + cleanFolder + "/" + filename, nil
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

	data, err := io.ReadAll(src)
	if err != nil {
		return "", fmt.Errorf("failed to read uploaded file: %w", err)
	}

	ext := getFileExtension(header)
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)

	if contentType == "" {
		contentType = getContentTypeForFile(header.Filename, data)
	}

	if err := storage.UploadBytes(cleanFolder+"/"+filename, data, contentType); err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	return "/" + filepath.ToSlash(filepath.Join("uploads", cleanFolder, filename)), nil
}

func ReadLatestUploadedImage(folder string) ([]byte, string, error) {
	cleanFolder, err := sanitizeUploadFolder(folder)
	if err != nil {
		return nil, "", err
	}

	reader, info, err := storage.GetLatest(cleanFolder + "/")
	if err != nil {
		return nil, "", err
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image: %w", err)
	}

	ct := info.ContentType
	if ct == "" {
		ct = getContentTypeForFile(info.Key, data)
	}

	return data, ct, nil
}
