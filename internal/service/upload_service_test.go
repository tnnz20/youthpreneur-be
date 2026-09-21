package service

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Minimal PNG header magic bytes
var samplePNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
	0x42, 0x60, 0x82,
}

// Minimal JPEG header magic bytes
var sampleJPEG = []byte{
	0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46,
	0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
	0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
	0x00, 0x03, 0x02, 0x02, 0x03, 0x02, 0x02, 0x03,
	0xFF, 0xD9,
}

func TestUploadServiceSaveThumbnailPNG(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewUploadService(tempDir)

	urlPath, err := svc.SaveThumbnail(bytes.NewReader(samplePNG), "test.png", int64(len(samplePNG)))
	if err != nil {
		t.Fatalf("unexpected error = %v", err)
	}

	if !strings.HasPrefix(urlPath, "/uploads/thumbnails/thumb_") || !strings.HasSuffix(urlPath, ".png") {
		t.Errorf("urlPath = %q, want /uploads/thumbnails/thumb_*.png", urlPath)
	}

	relPath := strings.TrimPrefix(urlPath, "/uploads/")
	fullPath := filepath.Join(tempDir, relPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !bytes.Equal(content, samplePNG) {
		t.Errorf("saved content differs from source")
	}
}

func TestUploadServiceSaveThumbnailJPEG(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewUploadService(tempDir)

	urlPath, err := svc.SaveThumbnail(bytes.NewReader(sampleJPEG), "photo.jpg", int64(len(sampleJPEG)))
	if err != nil {
		t.Fatalf("unexpected error = %v", err)
	}

	if !strings.HasPrefix(urlPath, "/uploads/thumbnails/thumb_") || !strings.HasSuffix(urlPath, ".jpg") {
		t.Errorf("urlPath = %q, want /uploads/thumbnails/thumb_*.jpg", urlPath)
	}
}

func TestUploadServiceRejectsUnsupportedType(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewUploadService(tempDir)

	pdf := []byte("%PDF-1.4 header text")
	_, err := svc.SaveThumbnail(bytes.NewReader(pdf), "doc.pdf", int64(len(pdf)))
	if !errors.Is(err, ErrInvalidFileType) {
		t.Errorf("got error %v, want ErrInvalidFileType", err)
	}
}

func TestUploadServiceRejectsOversizedFile(t *testing.T) {
	tempDir := t.TempDir()
	svc := NewUploadService(tempDir)

	_, err := svc.SaveThumbnail(bytes.NewReader([]byte("")), "big.png", MaxThumbnailSize+1)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("got error %v, want ErrFileTooLarge", err)
	}
}
