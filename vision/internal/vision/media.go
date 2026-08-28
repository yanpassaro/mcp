package vision

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func isURL(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://")
}

func mimeFromPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	default:
		return "application/octet-stream"
	}
}

func downloadBytes(ctx context.Context, ref string, maxBytes int64) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ref, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to request media: %w", err)
	}
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	limit := maxBytes
	if limit <= 0 {
		limit = 64 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, "", fmt.Errorf("failed to read media: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, "", fmt.Errorf("media exceeds the %d MB limit", limit/1024/1024)
	}
	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if ct == "" || strings.HasPrefix(ct, "application/octet-stream") {
		ct = mimeFromPath(ref)
	}
	return data, ct, nil
}

func LoadImageBytes(ctx context.Context, ref string, maxBytes int64) (MediaInput, error) {
	if isURL(ref) {
		data, ct, err := downloadBytes(ctx, ref, maxBytes)
		if err != nil {
			return MediaInput{}, err
		}
		if !strings.HasPrefix(ct, "image/") {
			ct = mimeFromPath(ref)
		}
		return MediaInput{Kind: "image", MIME: ct, Data: data}, nil
	}
	data, err := os.ReadFile(ref)
	if err != nil {
		return MediaInput{}, fmt.Errorf("failed to read file %s: %w", ref, err)
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return MediaInput{}, fmt.Errorf("image exceeds the %d MB limit", maxBytes/1024/1024)
	}
	return MediaInput{Kind: "image", MIME: mimeFromPath(ref), Data: data}, nil
}

func LoadVideoMedia(ctx context.Context, ref string, maxBytes int64) (MediaInput, func(), error) {
	if isURL(ref) {
		data, ct, err := downloadBytes(ctx, ref, maxBytes)
		if err != nil {
			return MediaInput{}, func() {}, err
		}
		if !strings.HasPrefix(ct, "video/") {
			ct = mimeFromPath(ref)
		}
		return MediaInput{Kind: "video", MIME: ct, Data: data}, func() {}, nil
	}
	data, err := os.ReadFile(ref)
	if err != nil {
		return MediaInput{}, func() {}, fmt.Errorf("failed to read file %s: %w", ref, err)
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return MediaInput{}, func() {}, fmt.Errorf("video exceeds the %d MB limit", maxBytes/1024/1024)
	}
	return MediaInput{Kind: "video", MIME: mimeFromPath(ref), Data: data}, func() {}, nil
}

func ValidVideoExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mov", ".m4v":
		return true
	}
	return false
}
