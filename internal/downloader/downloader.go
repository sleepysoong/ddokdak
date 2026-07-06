package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Downloader는 파일 다운로드를 담당합니다.
type Downloader struct {
	baseDir string
}

// New는 새로운 Downloader 인스턴스를 생성합니다.
func New(baseDir string) (*Downloader, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("다운로드 디렉토리 생성 실패: %w", err)
	}
	return &Downloader{baseDir: baseDir}, nil
}

// Download는 주어진 URL에서 파일을 다운로드하고 로컬 경로를 반환합니다.
func (d *Downloader) Download(url, filename string) (string, error) {
	// 안전한 파일명 생성 (시간 + 원본파일명)
	safeFilename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), sanitizeFilename(filename))
	destPath := filepath.Join(d.baseDir, safeFilename)

	// HTTP GET 요청
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("파일 다운로드 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("파일 다운로드 실패 (상태 코드: %d)", resp.StatusCode)
	}

	// 로컬 파일 생성
	out, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("로컬 파일 생성 실패: %w", err)
	}
	defer out.Close()

	// 내용 복사
	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("파일 내용 저장 실패: %w", err)
	}

	// 절대 경로 반환
	absPath, err := filepath.Abs(destPath)
	if err != nil {
		return destPath, nil
	}

	return absPath, nil
}

// sanitizeFilename은 파일명에서 경로 순회 공격 등을 방지하기 위해 특수문자를 제거합니다.
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	if name == "" {
		return "unknown_file"
	}
	return name
}
