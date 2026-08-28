package utils

import (
	"encoding/base64"
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

func SaveUploadedMedia(header *multipart.FileHeader, folder string) (string, error) {
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
	if contentType != "" && !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "video/") {
		return "", fmt.Errorf("only image and video files are allowed")
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

	// Validate and transcode video files for iOS compatibility
	isVideo := strings.HasPrefix(contentType, "video/")
	if isVideo {
		if err := ValidateVideo(data); err != nil {
			return "", err
		}
		transcoded, err := TranscodeToH264MP4(data)
		if err != nil {
			return "", fmt.Errorf("video processing failed: %w", err)
		}
		data = transcoded
		contentType = "video/mp4"
	}

	ext := getFileExtension(header)
	if isVideo {
		ext = ".mp4"
	}
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

func IsDataURI(s string) bool {
	return strings.HasPrefix(s, "data:")
}

func SaveDataURI(dataURI string, folder string) (string, error) {
	if !IsDataURI(dataURI) {
		return "", fmt.Errorf("not a data URI")
	}

	cleanFolder, err := sanitizeUploadFolder(folder)
	if err != nil {
		return "", err
	}

	parts := strings.SplitN(dataURI, ",", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid data URI format")
	}

	header := parts[0]
	data := parts[1]

	var contentType string
	if strings.Contains(header, ";") {
		headerParts := strings.SplitN(header, ";", 2)
		contentType = strings.TrimPrefix(headerParts[0], "data:")
	} else {
		contentType = strings.TrimPrefix(header, "data:")
	}
	if contentType == "" {
		contentType = "image/png"
	}

	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 data: %w", err)
	}

	ext := ""
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/jpeg":
		ext = ".jpg"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	case "image/svg+xml":
		ext = ".svg"
	default:
		ext = ".png"
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)

	if err := storage.UploadBytes(cleanFolder+"/"+filename, raw, contentType); err != nil {
		return "", fmt.Errorf("failed to upload data URI to MinIO: %w", err)
	}

	return "/uploads/" + cleanFolder + "/" + filename, nil
}
