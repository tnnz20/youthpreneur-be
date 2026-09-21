package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	// MaxThumbnailSize is the 5 MiB ceiling for uploaded thumbnail images.
	MaxThumbnailSize = 5 * 1024 * 1024
)

var (
	// ErrFileTooLarge reports that an upload exceeds MaxThumbnailSize.
	ErrFileTooLarge = errors.New("file exceeds maximum allowed size of 5MB")
	// ErrInvalidFileType reports an unsupported file MIME type.
	ErrInvalidFileType = errors.New("only PNG, JPG, and JPEG images are allowed")
)

// UploadService manages saving and validating uploaded assets.
type UploadService interface {
	SaveThumbnail(reader io.Reader, originalFilename string, size int64) (string, error)
}

type uploadService struct {
	baseDir string
}

// NewUploadService creates a filesystem-backed upload service rooted at baseDir.
func NewUploadService(baseDir string) UploadService {
	return &uploadService{baseDir: baseDir}
}

func (s *uploadService) SaveThumbnail(reader io.Reader, originalFilename string, size int64) (string, error) {
	if size > MaxThumbnailSize {
		return "", ErrFileTooLarge
	}

	// Read first 512 bytes for MIME type sniffing
	buffer := make([]byte, 512)
	n, err := io.ReadFull(reader, buffer)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", fmt.Errorf("read file header: %w", err)
	}

	contentType := http.DetectContentType(buffer[:n])
	var ext string
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/jpeg":
		ext = ".jpg"
	default:
		return "", ErrInvalidFileType
	}

	// Generate unique filename
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate random filename: %w", err)
	}
	filename := fmt.Sprintf("thumb_%d_%s%s", time.Now().Unix(), hex.EncodeToString(randomBytes), ext)

	thumbDir := filepath.Join(s.baseDir, "thumbnails")
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		return "", fmt.Errorf("create thumbnail directory: %w", err)
	}

	destPath := filepath.Join(thumbDir, filename)
	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create destination file: %w", err)
	}
	defer func() { _ = out.Close() }()

	if _, err := out.Write(buffer[:n]); err != nil {
		return "", fmt.Errorf("write file header: %w", err)
	}

	limitedReader := io.LimitReader(reader, MaxThumbnailSize-int64(n)+1)
	written, err := io.Copy(out, limitedReader)
	if err != nil {
		return "", fmt.Errorf("write file body: %w", err)
	}
	if int64(n)+written > MaxThumbnailSize {
		_ = os.Remove(destPath)
		return "", ErrFileTooLarge
	}

	return "/uploads/thumbnails/" + filename, nil
}
