package xcron

import (
	"go.uber.org/zap"

	"github.com/jasonlabz/potato/log"
	"github.com/jasonlabz/potato/utils"
)

var defaultLogger cronLogger = cronLogger{}

type cronLogger struct{}

func (l cronLogger) Info(msg string, keysAndValues ...interface{}) {
	log.GetLogger().WithAny(checkFields(keysAndValues)).Info(msg)
}

func (l cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	log.GetLogger().WithError(err).WithAny(checkFields(keysAndValues)).Error(msg)
}

func checkFields(fields []any) (checked []zap.Field) {
	checked = make([]zap.Field, 0)

	if len(fields) == 0 {
		return
	}

	_, isZapField := fields[0].(zap.Field)
	if isZapField {
		for _, field := range fields {
			if f, ok := field.(zap.Field); ok {
				checked = append(checked, f)
			}
		}
		return
	}

	if len(fields) == 1 {
		checked = append(checked, zap.Any("log_field", utils.StringValue(fields[0])))
		return
	}

	for i := 0; i < len(fields)-1; {
		checked = append(checked, zap.Any(utils.StringValue(fields[i]), utils.StringValue(fields[i+1])))
		if i == len(fields)-3 {
			checked = append(checked, zap.Any("log_field", utils.StringValue(fields[i+2])))
		}
		i += 2
	}

	return
}
