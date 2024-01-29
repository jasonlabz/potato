package log

import (
	"context"
	"testing"
)

func TestName(t *testing.T) {
	GetLogger(context.Background()).Error("ttt", "sadas", "sdasd", "time")
}
