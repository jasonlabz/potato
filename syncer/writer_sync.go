package syncer

import (
	"fmt"
	"os"
)

// FileWriter 定义结构体，实现 io.Writer 和自定义 Syncer
type FileWriter struct {
	file *os.File
}

// NewFileWriter 创建一个 FileWriter 实例
func NewFileWriter(filename string) (*FileWriter, error) {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return &FileWriter{file: file}, nil
}

// Write 实现 io.Writer 接口
func (ws *FileWriter) Write(p []byte) (n int, err error) {
	return ws.file.Write(p)
}

// Sync 实现 Sync 功能
func (ws *FileWriter) Sync() error {
	return ws.file.Sync()
}

// Close 关闭文件
func (ws *FileWriter) Close() error {
	return ws.file.Close()
}
