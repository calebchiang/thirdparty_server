package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type MediaService struct{}

func NewMediaService() *MediaService {
	return &MediaService{}
}

func (m *MediaService) saveUploadedFile(fileHeader *multipart.FileHeader) (string, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	// Ensure uploads directory exists
	uploadDir := "/tmp/uploads"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		return "", err
	}

	ext := filepath.Ext(fileHeader.Filename)
	filename := fmt.Sprintf("original_%s%s", uuid.New().String(), ext)
	dstPath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		_ = os.Remove(dstPath)
		return "", err
	}

	return dstPath, nil
}

func (m *MediaService) normalizeToM4A(inputPath string) (string, error) {

	outputPath := filepath.Join(
		filepath.Dir(inputPath),
		fmt.Sprintf("normalized_%s.m4a", uuid.New().String()),
	)

	// 60-second timeout to prevent hanging FFmpeg processes
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-y",
		"-i", inputPath,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "aac",
		"-b:a", "96k",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(outputPath)
		return "", fmt.Errorf("ffmpeg error: %v, output: %s", err, string(output))
	}

	return outputPath, nil
}

func (m *MediaService) Normalize(fileHeader *multipart.FileHeader) (string, error) {

	// 1. Save original file
	originalPath, err := m.saveUploadedFile(fileHeader)
	if err != nil {
		return "", err
	}

	// 2. Convert to standardized m4a
	normalizedPath, err := m.normalizeToM4A(originalPath)
	if err != nil {
		_ = os.Remove(originalPath)
		return "", err
	}

	// 3. Delete original after successful normalization
	_ = os.Remove(originalPath)

	return normalizedPath, nil
}
