package local

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type DemoStorage struct {
	baseDir string
}

func NewDemoStorage(baseDir string) *DemoStorage {
	return &DemoStorage{baseDir: baseDir}
}

func (s *DemoStorage) Save(reader io.Reader, fileName string) (string, error) {
	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return "", err
	}

	targetName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(fileName))
	targetPath := filepath.Join(s.baseDir, targetName)

	file, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return "", err
	}

	return targetPath, nil
}
