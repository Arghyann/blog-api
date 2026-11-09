package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const maxImageSize = 10 << 20 // 10 MB

var allowedImages = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

func initR2() *minio.Client {
	endpoint := os.Getenv("R2_ENDPOINT") // <account_id>.r2.cloudflarestorage.com
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("R2_ACCESS_KEY_ID"), os.Getenv("R2_SECRET_ACCESS_KEY"), ""),
		Secure: true,
		Region: "auto",
	})
	if err != nil {
		log.Fatal("R2 init: ", err)
	}
	return client
}

func (a *API) UploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImageSize)
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "image field required (max 10MB)", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}
	contentType := http.DetectContentType(data)
	ext, ok := allowedImages[contentType]
	if !ok && isSafeSVG(data) {
		contentType, ext, ok = "image/svg+xml", ".svg", true
	}
	if !ok {
		http.Error(w, "unsupported image type", http.StatusUnsupportedMediaType)
		return
	}

	name := strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename))
	key := fmt.Sprintf("%s-%s%s", uuid.New().String()[:8], sanitize(name), ext)

	_, err = a.r2.PutObject(r.Context(), os.Getenv("R2_BUCKET"), key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType, CacheControl: "public, max-age=31536000, immutable"})
	if err != nil {
		log.Println("R2 upload failed:", err)
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	url := strings.TrimRight(os.Getenv("R2_PUBLIC_URL"), "/") + "/" + key
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"url": url, "markdown": fmt.Sprintf("![%s](%s)", name, url)})
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, s)
}

var svgActive = regexp.MustCompile(`(?i)<script|<foreignobject|<iframe|<embed|<object|javascript:|\bon[a-z]+\s*=`)

// isSafeSVG accepts SVG files that contain no scripting. The sniffer can't
// identify SVG, so we look for an <svg root and reject active content.
func isSafeSVG(data []byte) bool {
	head := data
	if len(head) > 4096 {
		head = head[:4096]
	}
	if !bytes.Contains(bytes.ToLower(head), []byte("<svg")) {
		return false
	}
	return !svgActive.Match(data)
}
