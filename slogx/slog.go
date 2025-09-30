package slogx

import (
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/jasonlabz/potato/configx"
	"github.com/jasonlabz/potato/configx/file"
	"github.com/jasonlabz/potato/consts"
	"github.com/jasonlabz/potato/syncer"
	"github.com/jasonlabz/potato/utils"
)

var (
	logLevel                     = slog.LevelInfo
	defaultSlogConfigName        = "default_slog_config"
	MilliTimeFormat       string = "2006-01-02 15:04:05.000"
	MicroTimeFormat       string = "2006-01-02 15:04:05.000000"
	NanoTimeFormat        string = "2006-01-02 15:04:05.000000000"
	DateTimeFormat        string = "2006-01-02 15:04:05"
	DateFormat            string = "2006-01-02"
	TimeFormat            string = "15:04:05"
)

var defaultZapConfigPaths = []string{
	"./conf/logger.yaml",
	"./conf/app.yaml",
	"./conf/application.yaml",
	"./conf/slog.yaml",
	"./logger.yaml",
	"./app.yaml",
	"./application.yaml",
	"./slog.yaml",
}

var replaceAttr = func(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey {
		level := a.Value.Any().(slog.Level)
		levelLabel := level.String()
		a.Value = slog.StringValue(levelLabel)
	}

	// NOTE: 可以在这里修改时间输出格式
	if a.Key == slog.TimeKey {
		if t, ok := a.Value.Any().(time.Time); ok {
			a.Value = slog.StringValue(t.Format(MicroTimeFormat))
		}
	}

	return a
}

type Options struct {
	writeFile  bool
	logFormat  string
	name       string   // 应用名
	configPath string   // 日志配置文件
	keyList    []string // 自定义context中需要打印的Field字段
	logLevel   string   // 日志级别
	basePath   string   // 日志目录
	fileName   string   // 日志w文件
	maxSize    int      // 文件大小限制,单位MB
	maxAge     int      // 日志文件保留天数
	maxBackups int      // 最大保留日志文件数量
	compress   bool     // Compress确定是否应该使用gzip压缩已旋转的日志文件。默认值是不执行压缩。
}

type Option func(o *Options)

func WithName(name string) Option {
	return func(o *Options) {
		o.name = name
	}
}

func WithWriteFile(write bool) Option {
	return func(o *Options) {
		o.writeFile = write
	}
}

func WithLevel(level string) Option {
	return func(o *Options) {
		o.logLevel = level
	}
}

func WithBasePath(basePath string) Option {
	return func(o *Options) {
		o.basePath = basePath
	}
}

func WithFileName(fileName string) Option {
	return func(o *Options) {
		o.fileName = fileName
	}
}

func WithLogField(key string) Option {
	return func(o *Options) {
		o.keyList = append(o.keyList, key)
	}
}

func WithLogFields(keys ...string) Option {
	return func(o *Options) {
		o.keyList = append(o.keyList, keys...)
	}
}

func WithConfigPath(path string) Option {
	return func(o *Options) {
		o.configPath = path
	}
}

func NewLogger(opts ...Option) *LoggerWrapper {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	// 读取Slog配置文件
	var configLoad bool
	for _, confPath := range defaultZapConfigPaths {
		if configLoad {
			break
		}
		if utils.IsExist(confPath) {
			provider, err := file.NewConfigProvider(confPath)
			if err != nil {
				log.Printf("init logger {%s} err: %v", confPath, err)
				continue
			}
			configx.AddProviders(defaultSlogConfigName, provider)
			configLoad = true
		}
	}

	if !configLoad {
		log.Print("log init by default config")
	}
	// 加载配置
	loadConf(options)

	// 优先程序配置
	switch options.logLevel {
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
	// lowLevel文件WriteSyncer
	lowLevelFileWriteSyncer := getLowLevelWriterSyncer(options)

	var handler slog.Handler

	writeSyncer := func() syncer.WriteSyncer {
		if options.writeFile {
			return syncer.NewMultiWriteSyncer(lowLevelFileWriteSyncer, syncer.AddSync(os.Stdout))
		}
		return syncer.AddSync(os.Stdout)
	}()
	handlerOptions := &slog.HandlerOptions{
		AddSource:   true,
		Level:       logLevel,
		ReplaceAttr: replaceAttr,
	}

	if options.logFormat == "json" {
		handler = slog.NewJSONHandler(writeSyncer, handlerOptions)
	} else {
		handler = slog.NewTextHandler(writeSyncer, handlerOptions)
	}

	slogLogger := slog.New(handler)
	slog.SetDefault(slogLogger)
	return &LoggerWrapper{
		logger:     slogLogger,
		logField:   options.keyList,
		core:       writeSyncer,
		callerSkip: 3,
	}
}

// core 三个参数之  日志输出路径
func getLowLevelWriterSyncer(options *Options) syncer.WriteSyncer {
	fileName := filepath.Join(options.basePath, options.fileName)
	// 引入第三方库 Lumberjack 加入日志切割功能
	infoLumberIO := &lumberjack.Logger{
		Filename:   fileName,           // 日志文件存放目录，如果文件夹不存在会自动创建
		MaxSize:    options.maxSize,    // 文件大小限制,单位MB
		MaxBackups: options.maxBackups, // 最大保留日志文件数量
		MaxAge:     options.maxAge,     // 日志文件保留天数
		Compress:   options.compress,   // Compress确定是否应该使用gzip压缩已旋转的日志文件。默认值是不执行压缩。
	}
	return syncer.AddSync(infoLumberIO)
}

func loadConf(options *Options) {
	defaultOptions := Options{
		writeFile:  false,
		logFormat:  "console",
		configPath: "./conf/logger.yaml",
		keyList:    []string{consts.ContextLOGID, consts.ContextTraceID, consts.ContextUserID, consts.ContextClientAddr},
		logLevel:   "info",
		basePath:   "./log",
		fileName:   "app.log",
		maxSize:    15,
		maxAge:     7,
		maxBackups: 30,
		compress:   false,
	}

	level := configx.GetString(defaultSlogConfigName, "log.log_level")
	options.logLevel = utils.IsTrueOrNot(options.logLevel == "",
		utils.IsTrueOrNot(level == "", defaultOptions.logLevel, level), options.logLevel)

	logFormat := configx.GetString(defaultSlogConfigName, "log.format")
	options.logFormat = utils.IsTrueOrNot(options.logFormat == "",
		utils.IsTrueOrNot(logFormat == "", defaultOptions.logFormat, logFormat), options.logFormat)

	writeFile := configx.GetBool(defaultSlogConfigName, "log.write_file")
	options.writeFile = utils.IsTrueOrNot(!options.writeFile,
		utils.IsTrueOrNot(!writeFile, defaultOptions.writeFile, writeFile), options.writeFile)

	basePath := configx.GetString(defaultSlogConfigName, "log.log_file_conf.base_path")
	options.basePath = utils.IsTrueOrNot(options.basePath == "",
		utils.IsTrueOrNot(basePath == "", defaultOptions.basePath, basePath), options.basePath)

	fileName := configx.GetString(defaultSlogConfigName, "log.log_file_conf.file_name")
	options.fileName = utils.IsTrueOrNot(options.fileName == "",
		utils.IsTrueOrNot(fileName == "", defaultOptions.fileName, fileName), options.fileName)

	maxSize := configx.GetInt(defaultSlogConfigName, "log.log_file_conf.max_size")
	options.maxSize = utils.IsTrueOrNot(options.maxSize == 0,
		utils.IsTrueOrNot(maxSize == 0, defaultOptions.maxSize, maxSize), options.maxSize)

	maxAge := configx.GetInt(defaultSlogConfigName, "log.log_file_conf.max_age")
	options.maxAge = utils.IsTrueOrNot(options.maxAge == 0,
		utils.IsTrueOrNot(maxAge == 0, defaultOptions.maxAge, maxAge), options.maxAge)

	maxBackups := configx.GetInt(defaultSlogConfigName, "log.log_file_conf.max_backups")
	options.maxBackups = utils.IsTrueOrNot(options.maxBackups == 0,
		utils.IsTrueOrNot(maxBackups == 0, defaultOptions.maxBackups, maxBackups), options.maxBackups)

	compress := configx.GetBool(defaultSlogConfigName, "log.log_file_conf.compress")
	options.compress = utils.IsTrueOrNot(!options.compress,
		utils.IsTrueOrNot(!compress, defaultOptions.compress, compress), options.compress)

	if len(options.keyList) == 0 {
		options.keyList = defaultOptions.keyList
	}

	if options.name != "" {
		options.basePath = filepath.Join(options.basePath, options.name)
	}
}
