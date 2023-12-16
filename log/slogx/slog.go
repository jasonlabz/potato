package slogx

import (
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"log"
	"log/slog"
	"os"

	"github.com/jasonlabz/potato/core/config"
	"github.com/jasonlabz/potato/core/config/yaml"
	"github.com/jasonlabz/potato/core/utils"
)

var (
	logLevel                 = slog.LevelInfo
	DefaultSlogConfigName    = "default_slog_config"
	DefaultSlogConfigPath    = "./conf/log/logger.yaml"
	DefaultSlogConfigPathBak = "./conf/logger.yaml"
	slogLogger               *slog.Logger
)

func init() {
	InitLogger(DefaultSlogConfigPath)
}

func logger() *slog.Logger {
	if slogLogger != nil {
		return slogLogger
	}
	InitLogger(DefaultSlogConfigPath)
	return slogLogger
}

func InitLogger(filePath string) {
	// 读取Slog配置文件
	var configLoad bool
	if !configLoad && utils.IsExist(filePath) {
		provider := yaml.NewConfigProvider(filePath)
		config.AddProviders(DefaultSlogConfigName, provider)
		configLoad = true
	}

	if !configLoad && utils.IsExist(DefaultSlogConfigPath) {
		provider := yaml.NewConfigProvider(DefaultSlogConfigPath)
		config.AddProviders(DefaultSlogConfigName, provider)
		configLoad = true
	}

	if !configLoad && utils.IsExist(DefaultSlogConfigPathBak) {
		provider := yaml.NewConfigProvider(DefaultSlogConfigPathBak)
		config.AddProviders(DefaultSlogConfigName, provider)
		configLoad = true
	}

	if !configLoad {
		log.Printf("slogx init by default config")
	}

	levelConfig := config.GetString(DefaultSlogConfigName, "log_level")
	switch levelConfig {
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "debug":
		logLevel = slog.LevelDebug
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	//lowLevel文件WriteSyncer
	lowLevelFileWriteSyncer := getLowLevelWriterSyncer()

	writeFile := config.GetBool(DefaultSlogConfigName, "write_file")
	jsonLog := config.GetBool(DefaultSlogConfigName, "json_log")
	var handler slog.Handler
	if jsonLog {
		handler = slog.NewJSONHandler(func() zapcore.WriteSyncer {
			if writeFile {
				return zapcore.NewMultiWriteSyncer(lowLevelFileWriteSyncer, zapcore.AddSync(os.Stdout))
			}
			return zapcore.AddSync(os.Stdout)
		}(), &slog.HandlerOptions{
			AddSource: true,
			Level:     logLevel,
		})
	} else {
		handler = slog.NewTextHandler(func() zapcore.WriteSyncer {
			if writeFile {
				return zapcore.NewMultiWriteSyncer(lowLevelFileWriteSyncer, zapcore.AddSync(os.Stdout))
			}
			return zapcore.AddSync(os.Stdout)
		}(), &slog.HandlerOptions{
			AddSource: true,
			Level:     logLevel,
		})
	}

	slogLogger = slog.New(handler)
	slog.SetDefault(slogLogger)
	return
}

// core 三个参数之  日志输出路径
func getLowLevelWriterSyncer() zapcore.WriteSyncer {
	filename := func() string {
		getString := config.GetString(DefaultSlogConfigName, "log_file_conf.log_file_path")
		if getString == "" {
			getString = "./log/server.log"
		}
		return getString
	}()
	maxSize := func() int {
		geInt := config.GetInt(DefaultSlogConfigName, "log_file_conf.max_size")
		if geInt == 0 {
			return 300
		}
		return geInt
	}()
	maxAge := func() int {
		geInt := config.GetInt(DefaultSlogConfigName, "log_file_conf.max_age")
		if geInt == 0 {
			return 28
		}
		return geInt
	}()
	maxBackups := func() int {
		geInt := config.GetInt(DefaultSlogConfigName, "log_file_conf.max_backups")
		if geInt == 0 {
			return 20
		}
		return geInt
	}()
	compress := config.GetBool(DefaultSlogConfigName, "log_file_conf.compress")

	//引入第三方库 Lumberjack 加入日志切割功能
	infoLumberIO := &lumberjack.Logger{
		Filename:   filename,   //日志文件存放目录，如果文件夹不存在会自动创建
		MaxSize:    maxSize,    //文件大小限制,单位MB
		MaxBackups: maxBackups, //最大保留日志文件数量
		MaxAge:     maxAge,     //日志文件保留天数
		Compress:   compress,   //Compress确定是否应该使用gzip压缩已旋转的日志文件。默认值是不执行压缩。
	}
	return zapcore.AddSync(infoLumberIO)
}
