package slogx

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/jasonlabz/potato/configx"
	"github.com/jasonlabz/potato/consts"
	"github.com/jasonlabz/potato/syncer"
	"github.com/jasonlabz/potato/utils"
)

var (
	logLevel               = slog.LevelInfo
	defaultSlogConfigPaths = []string{
		"./conf/log/service.yaml",
		"./conf/logger.yaml",
		"./conf/slog.yaml",
		"./conf/log.yaml",
		"./logger.yaml",
		"./slog.yaml",
		"./log.yaml",
	}
)

// Config 日志配置结构体（与 zap 保持一致）
type Config struct {
	Type       string `mapstructure:"type" json:"type" yaml:"type" ini:"type"`
	Name       string `mapstructure:"name" json:"name" yaml:"name" ini:"name"`
	Format     string `mapstructure:"format" json:"format" yaml:"format" ini:"format"`
	LogLevel   string `mapstructure:"log_level" json:"log_level" yaml:"log_level" ini:"log_level"`
	WriteFile  bool   `mapstructure:"write_file" json:"write_file" yaml:"write_file" ini:"write_file"`
	BasePath   string `mapstructure:"base_path" json:"base_path" yaml:"base_path" ini:"base_path"`
	MaxSize    int    `mapstructure:"max_size" json:"max_size" yaml:"max_size" ini:"max_size"`
	MaxAge     int    `mapstructure:"max_age" json:"max_age" yaml:"max_age" ini:"max_age"`
	MaxBackups int    `mapstructure:"max_backups" json:"max_backups" yaml:"max_backups" ini:"max_backups"`
	Compress   bool   `mapstructure:"compress" json:"compress" yaml:"compress" ini:"compress"`
}

type Options struct {
	writeFile  bool
	logFormat  string
	name       string   // 应用名
	configPath string   // 日志配置文件
	keyList    []string // 自定义context中需要打印的Field字段
	logLevel   string   // 日志级别
	basePath   string   // 日志目录
	fileName   string   // 日志文件
	maxSize    int      // 文件大小限制,单位MB
	maxAge     int      // 日志文件保留天数
	maxBackups int      // 最大保留日志文件数量
	compress   bool     // Compress确定是否应该使用gzip压缩已旋转的日志文件。默认值是不执行压缩。
}

type Option func(o *Options)

func WithWriteFile(write bool) Option {
	return func(o *Options) {
		o.writeFile = write
	}
}

func WithName(name string) Option {
	return func(o *Options) {
		o.name = name
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

	// 读取日志配置文件
	var configLoad bool
	if options.configPath != "" && utils.IsExist(options.configPath) {
		panic(fmt.Errorf("log config not found: %s", options.configPath))
	}

	for _, confPath := range defaultSlogConfigPaths {
		if configLoad {
			break
		}
		if utils.IsExist(confPath) {
			options.configPath = confPath
			configLoad = true
		}
	}

	if !configLoad {
		log.Print("slogx log init by default config")
	}

	// 加载配置
	loadConf(options)

	return newSlogLogger(options)
}

// 初始化 slog logger
func newSlogLogger(options *Options) *LoggerWrapper {
	// 优先程序配置
	switch options.logLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
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

// 自定义属性替换函数
var replaceAttr = func(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey {
		level := a.Value.Any().(slog.Level)
		levelLabel := level.String()
		a.Value = slog.StringValue(levelLabel)
	}

	// 修改时间输出格式
	if a.Key == slog.TimeKey {
		if t, ok := a.Value.Any().(time.Time); ok {
			a.Value = slog.StringValue(t.Format("2006-01-02 15:04:05.000"))
		}
	}

	return a
}

// 日志输出路径
func getLowLevelWriterSyncer(options *Options) syncer.WriteSyncer {
	// 引入第三方库 Lumberjack 加入日志切割功能
	infoLumberIO := &lumberjack.Logger{
		Filename:   filepath.Join(options.basePath, options.fileName), // 日志文件存放目录，如果文件夹不存在会自动创建
		MaxSize:    options.maxSize,                                   // 文件大小限制,单位MB
		MaxBackups: options.maxBackups,                                // 最大保留日志文件数量
		MaxAge:     options.maxAge,                                    // 日志文件保留天数
		Compress:   options.compress,                                  // Compress确定是否应该使用gzip压缩已旋转的日志文件。默认值是不执行压缩。
	}
	return syncer.AddSync(infoLumberIO)
}

func loadConf(options *Options) {
	defaultOptions := Options{
		writeFile:  false,
		logFormat:  "console",
		configPath: "./conf/log/service.yaml",
		keyList:    []string{consts.ContextLOGID, consts.ContextTraceID, consts.ContextUserID, consts.ContextClientAddr},
		logLevel:   "info",
		basePath:   "./log",
		fileName:   "app.log",
		maxSize:    10,
		maxAge:     28,
		maxBackups: 100,
		compress:   false,
	}

	// 从配置文件读取配置
	logConf := &Config{}
	_ = configx.ParseConfigByViper(options.configPath, logConf)

	// 应用名
	options.name = utils.IsTrueOrNot(options.name == "",
		utils.IsTrueOrNot(logConf.Name == "", defaultOptions.name, logConf.Name), options.name)

	// 日志级别
	options.logLevel = utils.IsTrueOrNot(options.logLevel == "",
		utils.IsTrueOrNot(logConf.LogLevel == "", defaultOptions.logLevel, logConf.LogLevel), options.logLevel)

	// 日志格式
	options.logFormat = utils.IsTrueOrNot(options.logFormat == "",
		utils.IsTrueOrNot(logConf.Format == "", defaultOptions.logFormat, logConf.Format), options.logFormat)

	// 是否写入文件
	options.writeFile = utils.IsTrueOrNot(!options.writeFile,
		utils.IsTrueOrNot(!logConf.WriteFile, defaultOptions.writeFile, logConf.WriteFile), options.writeFile)

	// 基础路径
	options.basePath = utils.IsTrueOrNot(options.basePath == "",
		utils.IsTrueOrNot(logConf.BasePath == "", defaultOptions.basePath, logConf.BasePath), options.basePath)

	// 文件大小
	options.maxSize = utils.IsTrueOrNot(options.maxSize == 0,
		utils.IsTrueOrNot(logConf.MaxSize == 0, defaultOptions.maxSize, logConf.MaxSize), options.maxSize)

	// 保留天数
	options.maxAge = utils.IsTrueOrNot(options.maxAge == 0,
		utils.IsTrueOrNot(logConf.MaxAge == 0, defaultOptions.maxAge, logConf.MaxAge), options.maxAge)

	// 最大备份数
	options.maxBackups = utils.IsTrueOrNot(options.maxBackups == 0,
		utils.IsTrueOrNot(logConf.MaxBackups == 0, defaultOptions.maxBackups, logConf.MaxBackups), options.maxBackups)

	// 是否压缩
	options.compress = utils.IsTrueOrNot(!options.compress,
		utils.IsTrueOrNot(!logConf.Compress, defaultOptions.compress, logConf.Compress), options.compress)

	// 设置默认文件名
	if options.fileName == "" {
		if options.name != "" {
			options.fileName = options.name + ".log"
		} else {
			options.fileName = defaultOptions.fileName
		}
	}

	if len(options.keyList) == 0 {
		options.keyList = defaultOptions.keyList
	}

	// 如果有应用名，添加到基础路径中
	if options.name != "" {
		options.basePath = filepath.Join(options.basePath, options.name)
	}
}
