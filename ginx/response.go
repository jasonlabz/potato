package ginx

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	errors2 "github.com/jasonlabz/potato/errors"
	"github.com/jasonlabz/potato/log"
)

// Response 响应结构体
type Response struct {
	Code        int    `json:"code"`                // 错误码
	Message     string `json:"message,omitempty"`   // 错误信息
	ErrTrace    string `json:"err_trace,omitempty"` // 错误追踪链路信息
	Version     string `json:"version"`             // 版本信息
	CurrentTime string `json:"current_time"`        // 接口返回时间（当前时间）
	Data        any    `json:"data,omitempty"`      // 返回数据
}

// Pagination 分页结构体（该分页只适合数据量很少的情况）
type Pagination struct {
	Page      int64 `json:"page"`       // 当前页
	PageSize  int64 `json:"page_size"`  // 每页多少条记录
	PageCount int64 `json:"page_count"` // 一共多少页
	Total     int64 `json:"total"`      // 一共多少条记录
}

func (p *Pagination) GetPageCount() {
	p.PageCount = int64(math.Ceil(float64(p.Total) / float64(p.PageSize)))
	return
}

func (p *Pagination) GetOffset() (offset int64) {
	offset = (p.Page - 1) * p.PageSize
	return
}

type ResponseWithPagination struct {
	Response
	Pagination *Pagination `json:"pagination,omitempty"`
}

// FileDownloadConfig 文件下载配置
type FileDownloadConfig struct {
	Filename    string    // 下载文件名
	Preview     bool      // 是否开启预览模式
	ContentType string    // 内容类型，默认为 application/octet-stream
	Content     []byte    // 文件内容
	Reader      io.Reader // 文件读取器，与 Content 二选一
	Filepath    string    // 文件路径，与 Content 和 Reader 互斥
	Disposition string    // 内容处置类型，默认为 attachment
	BufferSize  int       // 缓冲区大小，默认为 4096
	DeleteAfter bool      // 下载后是否删除文件（仅对 Filepath 有效）
}

// ResponseOK 返回正确结果及数据
func ResponseOK(c *gin.Context, version string, data any) {
	c.JSON(prepareResponse(c, version, data, nil))
}

// ResponseErr 返回错误
func ResponseErr(c *gin.Context, version string, err error) {
	c.JSON(prepareResponse(c, version, nil, err))
}

// JsonResult 返回结果Json
func JsonResult(c *gin.Context, version string, data any, err error) {
	c.JSON(prepareResponse(c, version, data, err))
}

// FileResult 返回文件流下载
func FileResult(c *gin.Context, version string, config *FileDownloadConfig) {
	handleFileDownload(c, version, config)
}

// FileResultWithError 返回文件流下载，支持错误处理
func FileResultWithError(c *gin.Context, version string, config *FileDownloadConfig, err error) {
	if err != nil {
		ResponseErr(c, version, err)
		return
	}
	FileResult(c, version, config)
}

// PaginationResult 返回结果Json带分页
func PaginationResult(c *gin.Context, version string, data any, err error, pagination *Pagination) {
	c.JSON(prepareResponseWithPagination(c, version, data, err, pagination))
}

// PureJsonResult 返回结果PureJson
func PureJsonResult(c *gin.Context, version string, data any, err error) {
	c.PureJSON(prepareResponse(c, version, data, err))
}

// prepareResponse 准备响应信息
func prepareResponse(c *gin.Context, version string, data any, err error) (int, *Response) {
	// 格式化返回数据，非数组及切片时，转为切片
	data = handleData(data)
	code := http.StatusOK
	var errCode int
	var errMessage string
	var errTrace string

	if err != nil {
		var ex *errors2.Error
		if errors.As(err, &ex) {
			errCode = ex.Code()
			errMessage = ex.Message()
			errTrace = ex.Error()
		} else {
			code = http.StatusInternalServerError
			errMessage = err.Error()
			errTrace = err.Error()
		}
		log.GetLogger().Error(c, "        "+errTrace,
			log.Int("err_code", errCode), log.String("err_message", errMessage))
	}
	// 组装响应结果
	resp := &Response{
		Code:        errCode,
		Message:     errMessage,
		ErrTrace:    errTrace,
		Version:     version,
		Data:        data,
		CurrentTime: time.Now().Format(time.DateTime),
	}
	return code, resp
}

// prepareResponseWithPagination 准备响应信息
func prepareResponseWithPagination(c *gin.Context, version string,
	data any, err error, pagination *Pagination) (int, *ResponseWithPagination) {
	code, resp := prepareResponse(c, version, data, err)
	respWithPagination := &ResponseWithPagination{
		Response:   *resp,
		Pagination: pagination,
	}

	return code, respWithPagination
}

// handleData 格式化返回数据，非数组及切片时，转为切片
func handleData(data any) any {
	v := reflect.ValueOf(data)
	if !v.IsValid() || v.Kind() == reflect.Ptr && v.IsNil() {
		return make([]any, 0)
	}
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		return data
	}
	return []any{data}
}

// handleFileDownload 处理文件下载
func handleFileDownload(c *gin.Context, version string, config *FileDownloadConfig) {
	if config == nil {
		ResponseErr(c, version, errors.New("file download config is nil"))
		return
	}

	// 设置默认值
	if config.ContentType == "" {
		config.ContentType = "application/octet-stream"
	}
	if config.Disposition == "" {
		config.Disposition = "attachment"
	}
	if config.BufferSize <= 0 {
		config.BufferSize = 4096
	}
	if config.Preview {
		config.Disposition = "inline"
	}

	// 设置响应头
	filename := getDownloadFilename(config.Filename)
	c.Header("Content-Type", config.ContentType)
	c.Header("Content-Disposition", config.Disposition+"; filename="+filename)
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Cache-Control", "no-cache")

	// 处理不同的文件来源
	if config.Filepath != "" {
		handleFileDownloadFromPath(c, version, config)
	} else if config.Reader != nil {
		handleFileDownloadFromReader(c, version, config)
	} else if config.Content != nil {
		handleFileDownloadFromContent(c, version, config)
	}

	ResponseErr(c, version, errors.New("no file content provided"))
}

// handleFileDownloadFromPath 从文件路径下载
func handleFileDownloadFromPath(c *gin.Context, version string, config *FileDownloadConfig) {
	// 检查文件是否存在
	if _, err := os.Stat(config.Filepath); errors.Is(err, fs.ErrNotExist) {
		ResponseErr(c, version, errors.New("file not found: "+config.Filepath))
		return
	}

	// 打开文件
	file, err := os.Open(config.Filepath)
	if err != nil {
		ResponseErr(c, version, fmt.Errorf("open file error: %w", err))
		return
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := file.Stat()
	if err != nil {
		ResponseErr(c, version, fmt.Errorf("get file info error: %w", err))
		return
	}

	// 设置文件大小
	c.Header("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))

	// 如果文件名为空，使用原文件名
	if config.Filename == "" {
		config.Filename = filepath.Base(config.Filepath)
	}

	// 流式传输文件
	_, err = io.CopyBuffer(c.Writer, file, make([]byte, config.BufferSize))
	if err != nil {
		ResponseErr(c, version, fmt.Errorf("copy file error: %w", err))
		return
	}

	// 下载后删除文件
	if config.DeleteAfter {
		if err = os.Remove(config.Filepath); err != nil {
			log.GetLogger().Error(c, "failed to delete file after download: "+err.Error())
		}
	}

	return
}

// handleFileDownloadFromReader 从 Reader 下载
func handleFileDownloadFromReader(c *gin.Context, version string, config *FileDownloadConfig) {
	// 流式传输
	if _, err := io.CopyBuffer(c.Writer, config.Reader, make([]byte, config.BufferSize)); err != nil {
		ResponseErr(c, version, fmt.Errorf("download file error: %w", err))
	}
	return
}

// handleFileDownloadFromContent 从字节内容下载
func handleFileDownloadFromContent(c *gin.Context, version string, config *FileDownloadConfig) {
	// 设置内容长度
	c.Header("Content-Length", strconv.Itoa(len(config.Content)))

	// 直接写入内容
	if _, err := c.Writer.Write(config.Content); err != nil {
		ResponseErr(c, version, fmt.Errorf("download file error: %w", err))
	}
	return
}

// getDownloadFilename 处理下载文件名，确保浏览器兼容
func getDownloadFilename(filename string) string {
	if filename == "" {
		return "download"
	}

	// 对文件名进行编码，确保特殊字符正确处理
	return strings.ReplaceAll(strconv.Quote(filename), "\"", "")
}

// SimpleFileDownload 简化版文件下载（快速使用）
func SimpleFileDownload(c *gin.Context, version, filePath string, fileName string) {
	config := &FileDownloadConfig{
		Filepath:    filePath,
		Filename:    fileName,
		ContentType: getContentType(filePath),
	}
	FileResult(c, version, config)
}

// getContentType 根据文件扩展名获取内容类型
func getContentType(fileName string) string {
	if fileName == "" {
		return "application/octet-stream"
	}

	// 首先尝试标准库的检测
	contentType := mime.TypeByExtension(filepath.Ext(fileName))
	if contentType != "" {
		return contentType
	}

	// 标准库没有的类型，使用自定义映射
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileName), "."))
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".csv":
		return "text/csv"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".doc":
		return "application/msword"
	case ".zip":
		return "application/zip"
	case ".rar":
		return "application/x-rar-compressed"
	default:
		return "application/octet-stream"
	}
}
