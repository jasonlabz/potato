package log

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/jasonlabz/potato/configx"
	"github.com/jasonlabz/potato/consts"
	"github.com/jasonlabz/potato/times"
	"github.com/jasonlabz/potato/utils"
)

var (
	defaultZapConfigPaths = []string{
		"./conf/log/service.yaml",
		"./conf/logger.yaml",
		"./conf/zap.yaml",
		"./conf/log.yaml",
		"./logger.yaml",
		"./zap.yaml",
		"./log.yaml",
	}
)

// Config 日志配置结构体
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
	logType    string   // 日志类型 zap|slog
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

func WithLogType(logType string) Option {
	return func(o *Options) {
		o.logType = logType
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

	for _, confPath := range defaultZapConfigPaths {
		if configLoad {
			break
		}
		if utils.IsExist(confPath) {
			options.configPath = confPath
			configLoad = true
		}
	}

	if !configLoad {
		log.Print("zapx log init by default config")
	}

	// 加载配置
	loadConf(options)

	// 默认使用 zap
	return newZapLogger(options)
}

// 初始化 zap logger
func newZapLogger(options *Options) *LoggerWrapper {
	// 优先程序配置
	var logLevel zapcore.Level
	switch options.logLevel {
	case "debug":
		logLevel = zapcore.DebugLevel
	case "info":
		logLevel = zapcore.InfoLevel
	case "warn":
		logLevel = zapcore.WarnLevel
	case "error":
		logLevel = zapcore.ErrorLevel
	case "fatal":
		logLevel = zapcore.FatalLevel
	default:
		logLevel = zapcore.InfoLevel
	}

	// 日志级别
	highPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev >= zap.WarnLevel
	})
	lowPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev >= logLevel
	})

	// lowLevel文件WriteSyncer
	lowLevelFileWriteSyncer := getLowLevelWriterSyncer(options)
	// highLevel文件WriteSyncer
	highLevelFileWriteSyncer := getHighLevelWriterSyncer(options)

	// 获取编码器
	encoder := getEncoder(options)

	// 生成core
	// 同时输出到控制台 和 指定的日志文件中
	// AddSync将io.Writer转换成WriteSyncer的类型
	lowLevelFileCore := zapcore.NewCore(encoder, func() zapcore.WriteSyncer {
		if options.writeFile {
			return zapcore.NewMultiWriteSyncer(lowLevelFileWriteSyncer, zapcore.AddSync(os.Stdout))
		}
		return zapcore.AddSync(os.Stdout)
	}(), lowPriority)

	// lowLevelFileCore 和 highLevelFileCore 加入core切片
	var coreArr []zapcore.Core
	coreArr = append(coreArr, lowLevelFileCore)

	if options.writeFile {
		highLevelFileCore := zapcore.NewCore(encoder, func() zapcore.WriteSyncer {
			return zapcore.AddSync(highLevelFileWriteSyncer)
		}(), highPriority)
		coreArr = append(coreArr, highLevelFileCore)
	}

	// 生成logger
	zapLogger := zap.New(zapcore.NewTee(coreArr...), zap.AddCaller(), zap.AddCallerSkip(1))

	return &LoggerWrapper{
		logger:   zapLogger,
		logField: options.keyList,
	}
}

// 初始化 slog logger (预留实现)
func newSlogLogger(options *Options) *LoggerWrapper {
	// TODO: 实现 slog 日志初始化
	log.Print("slog logger is not implemented yet, using zap instead")
	return newZapLogger(options)
}

// core 三个参数之  Encoder 获取编码器
func getEncoder(options *Options) zapcore.Encoder {
	// 自定义编码配置
	encoderConfig := zap.NewProductionEncoderConfig()
	// encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(times.MilliTimeFormat) // 指定时间格式
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder                       // 在日志文件中使用大写字母记录日志级别
	// encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder //按级别显示不同颜色，不需要的话取值zapcore.CapitalLevelEncoder就可以了
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder // 显示短文件路径
	// encoderConfig.EncodeCaller = zapcore.FullCallerEncoder //显示完整文件路径
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	if options.logFormat == "json" {
		return zapcore.NewJSONEncoder(encoderConfig) // json 格式打印日志
	}
	return zapcore.NewConsoleEncoder(encoderConfig) // 默认 console 格式打印日志
}

// core 三个参数之  日志输出路径
func getLowLevelWriterSyncer(options *Options) zapcore.WriteSyncer {
	// 引入第三方库 Lumberjack 加入日志切割功能
	infoLumberIO := &lumberjack.Logger{
		Filename:   filepath.Join(options.basePath, options.fileName, options.fileName), // 日志文件存放目录，如果文件夹不存在会自动创建
		MaxSize:    options.maxSize,                                                     // 文件大小限制,单位MB
		MaxBackups: options.maxBackups,                                                  // 最大保留日志文件数量
		MaxAge:     options.maxAge,                                                      // 日志文件保留天数
		Compress:   options.compress,                                                    // Compress确定是否应该使用gzip压缩已旋转的日志文件。默认值是不执行压缩。
	}
	return zapcore.AddSync(infoLumberIO)
}

func getHighLevelWriterSyncer(options *Options) zapcore.WriteSyncer {
	// 引入第三方库 Lumberjack 加入日志切割功能
	lumberWriteSyncer := &lumberjack.Logger{
		Filename:   filepath.Join(options.basePath, options.fileName, options.fileName+".wf"), // 日志文件存放目录，如果文件夹不存在会自动创建
		MaxSize:    options.maxSize,                                                           // 文件大小限制,单位MB
		MaxBackups: options.maxBackups,                                                        // 最大保留日志文件数量
		MaxAge:     options.maxAge,                                                            // 日志文件保留天数
		Compress:   options.compress,                                                          // Compress确定是否应该使用gzip压缩已旋转的日志文件。默认值是不执行压缩。
	}
	return zapcore.AddSync(lumberWriteSyncer)
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
		maxSize:    15,
		maxAge:     7,
		maxBackups: 30,
		compress:   false,
		logType:    "zap", // 默认使用 zap
	}

	// 从配置文件读取日志类型
	logConf := &Config{}
	_ = configx.ParseConfigByViper(options.configPath, logConf)
	options.logType = utils.IsTrueOrNot(options.logType == "",
		utils.IsTrueOrNot(logConf.Type == "", defaultOptions.logType, logConf.Type), options.logType)

	// 从配置文件读取日志名
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
