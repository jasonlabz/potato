package log

import (
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/jasonlabz/potato/core/config"
	"github.com/jasonlabz/potato/core/config/yaml"
	"github.com/jasonlabz/potato/core/times"
	"github.com/jasonlabz/potato/core/utils"
)

var (
	DefaultZapConfigName    = "default_zap_config"
	DefaultZapConfigPath    = "./conf/log/logger.yaml"
	DefaultZapConfigPathBak = "./conf/logger.yaml"
	zapLogger               *zap.Logger
	zapSugaredLogger        *zap.SugaredLogger
)

func init() {
	InitLogger(DefaultZapConfigPath)
}

func logger() *zap.Logger {
	if zapLogger != nil {
		return zapLogger
	}
	InitLogger(DefaultZapConfigPath)
	return zapLogger
}

func sugaredLogger() *zap.SugaredLogger {
	if zapSugaredLogger != nil {
		return zapSugaredLogger
	}
	InitLogger(DefaultZapConfigPath)
	return zapSugaredLogger
}

func InitLogger(filePath string) {
	// 读取zap配置文件
	var configLoad bool
	if !configLoad && utils.IsExist(filePath) {
		provider := yaml.NewConfigProvider(filePath)
		config.AddProviders(DefaultZapConfigName, provider)
		configLoad = true
	}

	if !configLoad && utils.IsExist(DefaultZapConfigPath) {
		provider := yaml.NewConfigProvider(DefaultZapConfigPath)
		config.AddProviders(DefaultZapConfigName, provider)
		configLoad = true
	}

	if !configLoad && utils.IsExist(DefaultZapConfigPathBak) {
		provider := yaml.NewConfigProvider(DefaultZapConfigPathBak)
		config.AddProviders(DefaultZapConfigName, provider)
		configLoad = true
	}

	if !configLoad {
		log.Printf("log init by default config")
	}
	//获取编码器
	encoder := getEncoder()
	levelConfig := config.GetString(DefaultZapConfigName, "log_level")
	var loglevel zapcore.Level
	switch levelConfig {
	case "info":
		loglevel = zapcore.InfoLevel
	case "warn":
		loglevel = zapcore.WarnLevel
	case "debug":
		loglevel = zapcore.DebugLevel
	case "error":
		loglevel = zapcore.ErrorLevel
	default:
		loglevel = zapcore.InfoLevel
	}
	//日志级别
	highPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool { //error级别
		return lev >= zap.ErrorLevel
	})
	lowPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool { //info和debug级别,debug级别是最低的
		return lev >= loglevel
	})

	//lowLevel文件WriteSyncer
	lowLevelFileWriteSyncer := getLowLevelWriterSyncer()
	//highLevel文件WriteSyncer
	highLevelFileWriteSyncer := getHighLevelWriterSyncer()

	writeFile := config.GetBool(DefaultZapConfigName, "write_file")
	//生成core
	//multiWriteSyncer := zapcore.NewMultiWriteSyncer(writerSyncer, zapcore.AddSync(os.Stdout)) //AddSync将io.Writer转换成WriteSyncer的类型
	//同时输出到控制台 和 指定的日志文件中
	lowLevelFileCore := zapcore.NewCore(encoder, func() zapcore.WriteSyncer {
		if writeFile {
			return zapcore.NewMultiWriteSyncer(lowLevelFileWriteSyncer, zapcore.AddSync(os.Stdout))
		}
		return zapcore.AddSync(os.Stdout)
	}(), lowPriority)

	highLevelFileCore := zapcore.NewCore(encoder, func() zapcore.WriteSyncer {
		if writeFile {
			return zapcore.NewMultiWriteSyncer(highLevelFileWriteSyncer, zapcore.AddSync(os.Stdout))
		}
		return zapcore.AddSync(os.Stdout)
	}(), highPriority)

	//lowLevelFileCore 和 highLevelFileCore 加入core切片
	var coreArr []zapcore.Core
	coreArr = append(coreArr, lowLevelFileCore)
	coreArr = append(coreArr, highLevelFileCore)

	//生成logger
	zapLogger = zap.New(zapcore.NewTee(coreArr...), zap.AddCaller(), zap.AddCallerSkip(1)) //zap.AddCaller() 显示文件名 和 行号
	zapSugaredLogger = zapLogger.Sugar()
	return
}

// core 三个参数之  Encoder 获取编码器
func getEncoder() zapcore.Encoder {
	//自定义编码配置
	encoderConfig := zap.NewProductionEncoderConfig()
	//encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(times.MicroTimeFormat) //指定时间格式
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder                       //在日志文件中使用大写字母记录日志级别
	//encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder //按级别显示不同颜色，不需要的话取值zapcore.CapitalLevelEncoder就可以了
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder //显示完整文件路径
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	return zapcore.NewConsoleEncoder(encoderConfig) // console 格式打印日志
	//return zapcore.NewJSONEncoder(encoderConfig)     // json 格式打印日志
}

// core 三个参数之  日志输出路径
func getLowLevelWriterSyncer() zapcore.WriteSyncer {
	filename := func() string {
		getString := config.GetString(DefaultZapConfigName, "low_level_conf.log_file_path")
		if getString == "" {
			return "./log/server.log"
		}
		return getString
	}()
	maxSize := func() int {
		geInt := config.GetInt(DefaultZapConfigName, "low_level_conf.max_size")
		if geInt == 0 {
			return 300
		}
		return geInt
	}()
	maxAge := func() int {
		geInt := config.GetInt(DefaultZapConfigName, "low_level_conf.max_age")
		if geInt == 0 {
			return 28
		}
		return geInt
	}()
	maxBackups := func() int {
		geInt := config.GetInt(DefaultZapConfigName, "low_level_conf.max_backups")
		if geInt == 0 {
			return 20
		}
		return geInt
	}()
	compress := config.GetBool(DefaultZapConfigName, "low_level_conf.compress")

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

func getHighLevelWriterSyncer() zapcore.WriteSyncer {
	filename := func() string {
		getString := config.GetString(DefaultZapConfigName, "high_level_conf.log_file_path")
		if getString == "" {
			return "./log/server.wf.log"
		}
		return getString
	}()
	maxSize := func() int {
		geInt := config.GetInt(DefaultZapConfigName, "high_level_conf.max_size")
		if geInt == 0 {
			return 300
		}
		return geInt
	}()
	maxAge := func() int {
		geInt := config.GetInt(DefaultZapConfigName, "high_level_conf.max_age")
		if geInt == 0 {
			return 28
		}
		return geInt
	}()
	maxBackups := func() int {
		geInt := config.GetInt(DefaultZapConfigName, "high_level_conf.max_backups")
		if geInt == 0 {
			return 20
		}
		return geInt
	}()
	compress := config.GetBool(DefaultZapConfigName, "high_level_conf.compress")

	//引入第三方库 Lumberjack 加入日志切割功能
	lumberWriteSyncer := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize, // megabytes
		MaxBackups: maxBackups,
		MaxAge:     maxAge,   // days
		Compress:   compress, //Compress确定是否应该使用gzip压缩已旋转的日志文件。默认值是不执行压缩。
	}
	return zapcore.AddSync(lumberWriteSyncer)
}
