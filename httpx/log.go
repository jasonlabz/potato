// Package httpx
//
//   _ __ ___   __ _ _ __  _   _| |_
//  | '_ ` _ \ / _` | '_ \| | | | __|
//  | | | | | | (_| | | | | |_| | |_
//  |_| |_| |_|\__,_|_| |_|\__,_|\__|
//
//  Buddha bless, no bugs forever!
//
//  Author:    lucas
//  Email:     1783022886@qq.com
//  Created:   2025/11/29 14:22
//  Version:   v1.0.0

package httpx

import (
	"context"

	"github.com/jasonlabz/potato/log"
)

func AdaptLogger(l *log.LoggerWrapper) *Logger {
	if l == nil {
		return &Logger{l: log.GetLogger()}
	}
	return &Logger{l: l}
}

type Logger struct {
	l *log.LoggerWrapper
}

func (l *Logger) Errorf(format string, v ...any) {
	l.l.Errorf(context.Background(), format, v...)
}

func (l *Logger) Warnf(format string, v ...any) {
	l.l.Warnf(context.Background(), format, v...)
}

func (l *Logger) Debugf(format string, v ...any) {
	l.l.Debugf(context.Background(), format, v...)
}
