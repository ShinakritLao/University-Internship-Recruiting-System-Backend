package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// UploadToSupabase uploads a multipart file to a Supabase Storage bucket and returns the public URL.
func UploadToSupabase(bucket string, file *multipart.FileHeader) (string, error) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	serviceKey := os.Getenv("SUPABASE_SERVICE_KEY")
	if supabaseURL == "" || serviceKey == "" {
		return "", fmt.Errorf("supabase storage env vars not set")
	}

	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, src); err != nil {
		return "", err
	}

	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		return "", err
	}
	ext := filepath.Ext(file.Filename)
	objectPath := fmt.Sprintf("%s%s", hex.EncodeToString(randBytes), ext)

	uploadURL := fmt.Sprintf("%s/storage/v1/object/%s/%s", supabaseURL, bucket, objectPath)
	req, err := http.NewRequest("POST", uploadURL, buf)
	if err != nil {
		return "", err
	}

	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+serviceKey)
	req.Header.Set("x-upsert", "true")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed (%d): %s", resp.StatusCode, string(body))
	}

	publicURL := fmt.Sprintf("%s/storage/v1/object/public/%s/%s", supabaseURL, bucket, objectPath)
	return publicURL, nil
}

func SaveUploadedFile(file *multipart.FileHeader, folder string) (string, error) {
	// Create folder if it doesn't exist
	err := os.MkdirAll(folder, os.ModePerm)
	if err != nil {
		return "", err
	}

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)

	// Full save path
	savePath := filepath.Join(folder, filename)

	// Save file
	err = SaveFile(file, savePath)
	if err != nil {
		return "", err
	}

	// Return public path
	return "/" + savePath, nil
}

func SaveFile(file *multipart.FileHeader, path string) error {
	return os.WriteFile(path, []byte{}, 0644)
}