package log

import (
	"log"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/jasonlabz/potato/core/config"
	"github.com/jasonlabz/potato/core/config/yaml"
	"github.com/jasonlabz/potato/core/consts"
	"github.com/jasonlabz/potato/core/times"
	"github.com/jasonlabz/potato/core/utils"
)

var (
	DefaultZapConfigName    = "default_zap_config"
	DefaultZapConfigPathBak = "./conf/logger.yaml"
)

type Options struct {
	writeFile  bool
	logFormat  string
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

func newLogger(opts ...Option) *loggerWrapper {
	options := &Options{}

	for _, opt := range opts {
		opt(options)
	}

	// 读取zap配置文件
	var configLoad bool
	if !configLoad && options.configPath != "" && utils.IsExist(options.configPath) {
		provider := yaml.NewConfigProvider(options.configPath)
		config.AddProviders(DefaultZapConfigName, provider)
		configLoad = true
	}

	if !configLoad && utils.IsExist(consts.DefaultConfigPath) {
		provider := yaml.NewConfigProvider(consts.DefaultConfigPath)
		config.AddProviders(DefaultZapConfigName, provider)
		configLoad = true
	}

	if !configLoad && utils.IsExist(DefaultZapConfigPathBak) {
		provider := yaml.NewConfigProvider(DefaultZapConfigPathBak)
		config.AddProviders(DefaultZapConfigName, provider)
		configLoad = true
	}

	if !configLoad {
		log.Printf("zapx log init by default config")
	}
	// 加载配置
	loadConf(options)

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
	//日志级别
	highPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool { //error级别
		return lev >= zap.WarnLevel
	})
	lowPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool { //info和debug级别,debug级别是最低的
		return lev >= logLevel
	})

	//lowLevel文件WriteSyncer
	lowLevelFileWriteSyncer := getLowLevelWriterSyncer(options)
	//highLevel文件WriteSyncer
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

	//lowLevelFileCore 和 highLevelFileCore 加入core切片
	var coreArr []zapcore.Core
	coreArr = append(coreArr, lowLevelFileCore)

	if options.writeFile {
		highLevelFileCore := zapcore.NewCore(encoder, func() zapcore.WriteSyncer {
			return zapcore.AddSync(highLevelFileWriteSyncer)
		}(), highPriority)
		coreArr = append(coreArr, highLevelFileCore)
	}

	//生成logger
	zapLogger := zap.New(zapcore.NewTee(coreArr...), zap.AddCaller(), zap.AddCallerSkip(1)) //zap.AddCaller() 显示文件名 和 行号

	return &loggerWrapper{
		logger:   zapLogger,
		logField: options.keyList,
	}
}

// core 三个参数之  Encoder 获取编码器
func getEncoder(options *Options) zapcore.Encoder {
	//自定义编码配置
	encoderConfig := zap.NewProductionEncoderConfig()
	//encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(times.MicroTimeFormat) //指定时间格式
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder                       //在日志文件中使用大写字母记录日志级别
	//encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder //按级别显示不同颜色，不需要的话取值zapcore.CapitalLevelEncoder就可以了
	//encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder //显示完整文件路径
	encoderConfig.EncodeCaller = zapcore.FullCallerEncoder //显示完整文件路径
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	if options.logFormat == "json" {
		return zapcore.NewJSONEncoder(encoderConfig) // json 格式打印日志
	}
	return zapcore.NewConsoleEncoder(encoderConfig) // 默认 console 格式打印日志
}

// core 三个参数之  日志输出路径
func getLowLevelWriterSyncer(options *Options) zapcore.WriteSyncer {
	//引入第三方库 Lumberjack 加入日志切割功能
	infoLumberIO := &lumberjack.Logger{
		Filename:   filepath.Join(options.basePath, options.fileName), //日志文件存放目录，如果文件夹不存在会自动创建
		MaxSize:    options.maxSize,                                   //文件大小限制,单位MB
		MaxBackups: options.maxBackups,                                //最大保留日志文件数量
		MaxAge:     options.maxAge,                                    //日志文件保留天数
		Compress:   options.compress,                                  //Compress确定是否应该使用gzip压缩已旋转的日志文件。默认值是不执行压缩。
	}
	return zapcore.AddSync(infoLumberIO)
}

func getHighLevelWriterSyncer(options *Options) zapcore.WriteSyncer {
	//引入第三方库 Lumberjack 加入日志切割功能
	lumberWriteSyncer := &lumberjack.Logger{
		Filename:   filepath.Join(options.basePath, options.fileName+".wf"), //日志文件存放目录，如果文件夹不存在会自动创建
		MaxSize:    options.maxSize,                                         //文件大小限制,单位MB
		MaxBackups: options.maxBackups,                                      //最大保留日志文件数量
		MaxAge:     options.maxAge,                                          //日志文件保留天数
		Compress:   options.compress,                                        //Compress确定是否应该使用gzip压缩已旋转的日志文件。默认值是不执行压缩。
	}
	return zapcore.AddSync(lumberWriteSyncer)
}

func loadConf(options *Options) {
	defaultOptions := Options{
		writeFile:  false,
		logFormat:  "console",
		configPath: "./conf/app.yaml",
		keyList:    []string{consts.ContextTraceID, consts.ContextUserID},
		logLevel:   "info",
		basePath:   "./log",
		fileName:   "app.log",
		maxSize:    15,
		maxAge:     7,
		maxBackups: 30,
		compress:   false,
	}

	level := config.GetString(DefaultZapConfigName, "log.log_level")
	if level == "" {
		options.logLevel = defaultOptions.logLevel
	}

	logFormat := config.GetString(DefaultZapConfigName, "log.format")
	if logFormat == "" {
		options.logFormat = defaultOptions.logFormat
	}

	writeFile := config.GetBool(DefaultZapConfigName, "log.write_file")
	if writeFile == false {
		options.writeFile = defaultOptions.writeFile
	}

	basePath := config.GetString(DefaultZapConfigName, "log.log_file_conf.base_path")
	if basePath == "" {
		options.basePath = defaultOptions.basePath
	}

	fileName := config.GetString(DefaultZapConfigName, "log.log_file_conf.file_name")
	if fileName == "" {
		options.fileName = defaultOptions.fileName
	}

	maxSize := config.GetInt(DefaultZapConfigName, "log.log_file_conf.max_size")
	if maxSize == 0 {
		options.maxSize = defaultOptions.maxSize
	}

	maxAge := config.GetInt(DefaultZapConfigName, "log.log_file_conf.max_age")
	if maxAge == 0 {
		options.maxAge = defaultOptions.maxAge
	}

	maxBackups := config.GetInt(DefaultZapConfigName, "log.log_file_conf.max_backups")
	if maxBackups == 0 {
		options.maxBackups = defaultOptions.maxBackups
	}

	compress := config.GetBool(DefaultZapConfigName, "log.log_file_conf.compress")
	if compress == false {
		options.compress = defaultOptions.compress
	}

	if len(options.keyList) == 0 {
		options.keyList = defaultOptions.keyList
	}
}
