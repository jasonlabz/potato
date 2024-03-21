package log

import (
	"context"
	"testing"
)

func TestName(t *testing.T) {
	GetLogger(context.Background()).Error("ttt%s,%s,%s", "sadas", "sdasd", "time")
}
