package log

import (
	"log"
	"os"
	"potato/times"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"potato/core/config/thirdconfig"
	"potato/core/config/thirdconfig/yaml"
)

var DefaultZapConfigName = "default_zap_config"
var DefaultZapConfigPath = "./conf/log/logger.yaml"
var DefaultZapConfigPathBak = "./conf/logger.yaml"
var zapLogger *zap.Logger
var zapSugaredLogger *zap.SugaredLogger

func init() {
	InitLogger()
}

func logger() *zap.Logger {
	if zapLogger != nil {
		return zapLogger
	}
	InitLogger()
	return zapLogger
}

func sugaredLogger() *zap.SugaredLogger {
	if zapSugaredLogger != nil {
		return zapSugaredLogger
	}
	InitLogger()
	return zapSugaredLogger
}

func InitLogger() {
	// 读取zap配置文件
	var configLoad bool
	_, err := os.Stat(DefaultZapConfigPath)
	if err != nil {
		if os.IsExist(err) {
			provider := yaml.NewConfigProvider(DefaultZapConfigPath)
			thirdconfig.AddProviders(DefaultZapConfigName, provider)
			configLoad = true
		}
	} else {
		provider := yaml.NewConfigProvider(DefaultZapConfigPath)
		thirdconfig.AddProviders(DefaultZapConfigName, provider)
		configLoad = true
	}
	if !configLoad {
		_, err = os.Stat(DefaultZapConfigPathBak)
		if err != nil {
			if os.IsExist(err) {
				provider := yaml.NewConfigProvider(DefaultZapConfigPathBak)
				thirdconfig.AddProviders(DefaultZapConfigName, provider)
				configLoad = true
			}
		} else {
			provider := yaml.NewConfigProvider(DefaultZapConfigPathBak)
			thirdconfig.AddProviders(DefaultZapConfigName, provider)
			configLoad = true
		}
	}
	if !configLoad {
		log.Printf("log init by default config")
	}
	log.Printf("log init by default config")
	//获取编码器
	encoder := getEncoder()

	//日志级别
	highPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool { //error级别
		return lev >= zap.ErrorLevel
	})
	lowPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool { //info和debug级别,debug级别是最低的
		return lev <= zap.ErrorLevel && lev >= zap.DebugLevel
	})

	//info文件WriteSyncer
	infoFileWriteSyncer := getInfoWriterSyncer()
	//error文件WriteSyncer
	errorFileWriteSyncer := getErrorWriterSyncer()

	writeFile := thirdconfig.GetBool(DefaultZapConfigName, "write_file")
	//生成core
	//multiWriteSyncer := zapcore.NewMultiWriteSyncer(writerSyncer, zapcore.AddSync(os.Stdout)) //AddSync将io.Writer转换成WriteSyncer的类型
	//同时输出到控制台 和 指定的日志文件中
	infoFileCore := zapcore.NewCore(encoder, func() zapcore.WriteSyncer {
		if writeFile {
			return zapcore.NewMultiWriteSyncer(infoFileWriteSyncer, zapcore.AddSync(os.Stdout))
		}
		return zapcore.AddSync(os.Stdout)
	}(), lowPriority)
	errorFileCore := zapcore.NewCore(encoder, func() zapcore.WriteSyncer {
		if writeFile {
			return zapcore.NewMultiWriteSyncer(errorFileWriteSyncer, zapcore.AddSync(os.Stdout))
		}
		return zapcore.AddSync(os.Stdout)
	}(), highPriority)

	//将infocore 和 errcore 加入core切片
	var coreArr []zapcore.Core
	coreArr = append(coreArr, infoFileCore)
	coreArr = append(coreArr, errorFileCore)

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
	//encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder               //在日志文件中使用大写字母记录日志级别
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder //按级别显示不同颜色，不需要的话取值zapcore.CapitalLevelEncoder就可以了
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder      //显示完整文件路径
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	//return zapcore.NewConsoleEncoder(encoderConfig)  // console 格式打印日志
	//return zapcore.NewJSONEncoder(encoderConfig)     // json 格式打印日志
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// core 三个参数之  日志输出路径
func getInfoWriterSyncer() zapcore.WriteSyncer {
	filename := func() string {
		getString := thirdconfig.GetString(DefaultZapConfigName, "info.log_file_path")
		if getString == "" {
			return "./server_log/info.log"
		}
		return getString
	}()
	maxSize := func() int {
		geInt := thirdconfig.GetInt(DefaultZapConfigName, "info.max_size")
		if geInt == 0 {
			return 10
		}
		return geInt
	}()
	maxAge := func() int {
		geInt := thirdconfig.GetInt(DefaultZapConfigName, "info.max_age")
		if geInt == 0 {
			return 28
		}
		return geInt
	}()
	maxBackups := func() int {
		geInt := thirdconfig.GetInt(DefaultZapConfigName, "info.max_backups")
		if geInt == 0 {
			return 100
		}
		return geInt
	}()
	compress := thirdconfig.GetBool(DefaultZapConfigName, "info.compress")

	//引入第三方库 Lumberjack 加入日志切割功能
	infoLumberIO := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize, // megabytes
		MaxBackups: maxBackups,
		MaxAge:     maxAge,   // days
		Compress:   compress, //Compress确定是否应该使用gzip压缩已旋转的日志文件。默认值是不执行压缩。
	}
	return zapcore.AddSync(infoLumberIO)
}

func getErrorWriterSyncer() zapcore.WriteSyncer {
	filename := func() string {
		getString := thirdconfig.GetString(DefaultZapConfigName, "error.log_file_path")
		if getString == "" {
			return "./server_log/error.log"
		}
		return getString
	}()
	maxSize := func() int {
		geInt := thirdconfig.GetInt(DefaultZapConfigName, "error.max_size")
		if geInt == 0 {
			return 10
		}
		return geInt
	}()
	maxAge := func() int {
		geInt := thirdconfig.GetInt(DefaultZapConfigName, "error.max_age")
		if geInt == 0 {
			return 28
		}
		return geInt
	}()
	maxBackups := func() int {
		geInt := thirdconfig.GetInt(DefaultZapConfigName, "error.max_backups")
		if geInt == 0 {
			return 100
		}
		return geInt
	}()
	compress := thirdconfig.GetBool(DefaultZapConfigName, "error.compress")

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
