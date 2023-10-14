package httpx

import "github.com/go-resty/resty/v2"

var httpClientLog resty.Logger = (*ClientLogger)(nil)

type ClientLogger struct {
	l resty.Logger
}

func (l *ClientLogger) Debugf(format string, v ...interface{}) {
	l.l.Debugf("DEBUG RESTY "+format, v...)
}

func (l *ClientLogger) Errorf(format string, v ...interface{}) {
	l.l.Errorf("ERROR RESTY "+format, v...)
}

func (l *ClientLogger) Warnf(format string, v ...interface{}) {
	l.l.Warnf("WARN RESTY "+format, v...)
}
