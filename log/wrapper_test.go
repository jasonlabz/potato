package log

import (
	"context"
	"testing"
)

func TestName(t *testing.T) {
	ctx := context.TODO()
	GetLogger(ctx).Error("ttt%s,%s,%s", "sadas", "sdasd", "time")
}
