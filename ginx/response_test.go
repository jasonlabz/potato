package ginx

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFileResultContentHasNoTrailingError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/download", nil)

	FileResult(c, "v1", &FileDownloadConfig{Filename: "a.txt", Content: []byte("hello")})
	if got := w.Body.String(); got != "hello" {
		t.Fatalf("FileResult() body = %q, want %q", got, "hello")
	}
}
