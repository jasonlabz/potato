package application

import (
	"log"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/bytedance/sonic/decoder"
	"github.com/fsnotify/fsnotify"
	"github.com/go-ini/ini"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/jasonlabz/potato/core/utils"
)

type CryptoType string

const (
	CryptoTypeAES  CryptoType = "aes"
	CryptoTypeDES  CryptoType = "des"
	CryptoTypeHMAC CryptoType = "hmac"
)

// CryptoConfig 加密配置
type CryptoConfig struct {
	Type string `mapstructure:"type" json:"type" ini:"type" yaml:"type"`
	Key  string `mapstructure:"key" json:"key" ini:"key" yaml:"key"`
}

// KafkaConfig 配置
type KafkaConfig struct {
	Topic            []string `json:"topic" yaml:"topic" ini:"topic"`
	GroupId          string   `json:"group_id" yaml:"group_id" ini:"group_id"`
	BootstrapServers string   `json:"bootstrap_servers" yaml:"bootstrap_servers" ini:"bootstrap_servers"`
	SecurityProtocol string   `json:"security_protocol" yaml:"security_protocol" ini:"security_protocol"`
	SaslMechanism    string   `json:"sasl_mechanism" yaml:"sasl_mechanism" ini:"sasl_mechanism"`
	SaslUsername     string   `json:"sasl_username" yaml:"sasl_username" ini:"sasl_username"`
	SaslPassword     string   `json:"sasl_password" yaml:"sasl_password" ini:"sasl_password"`
}

// Database 连接配置
type Database struct {
	DBType          string `json:"db_type" yaml:"db_type" ini:"db_type"`
	DSN             string `json:"dsn" yaml:"dsn" ini:"dsn"`
	LogMode         string `json:"log_mode" yaml:"log_mode" ini:"log_mode"`
	Host            string `json:"host" yaml:"host" ini:"host"`
	Port            int    `json:"port" yaml:"port" ini:"port"`
	Database        string `json:"database" yaml:"database" ini:"database"`
	Username        string `json:"username" yaml:"username" ini:"username"`
	Password        string `json:"password" yaml:"password" ini:"password"`
	Charset         string `json:"charset" yaml:"charset" ini:"charset"`
	MaxIdleConn     int    `json:"max_idle_conn" yaml:"max_idle_conn" ini:"max_idle_conn"`
	MaxOpenConn     int    `json:"max_open_conn" yaml:"max_open_conn" ini:"max_open_conn"`
	ConnMaxLifeTime int64  `json:"conn_max_life_time" yaml:"conn_max_life_time" ini:"conn_max_life_time"`
}

// RedisConfig 连接配置
type RedisConfig struct {
	ClientName       string   `json:"client_name" yaml:"client_name" ini:"client_name"` // 自定义客户端名
	MasterName       string   `json:"master_name" yaml:"master_name" ini:"master_name"` // 主节点
	Endpoints        []string `json:"endpoints" yaml:"endpoints" ini:"endpoints"`
	Username         string   `json:"username" yaml:"username" ini:"username"`
	Password         string   `json:"password" yaml:"password" ini:"password"`
	IndexDB          int      `json:"index_db" yaml:"index_db" ini:"index_db"`
	MinIdleConns     int      `json:"min_idle_conns" yaml:"min_idle_conns" ini:"min_idle_conns"`
	MaxIdleConns     int      `json:"max_idle_conns" yaml:"max_idle_conns" ini:"max_idle_conns"`
	MaxActiveConns   int      `json:"max_active_conns" yaml:"max_active_conns" ini:"max_active_conns"`
	MaxRetryTimes    int      `json:"max_retry_times" yaml:"max_retry_times" ini:"max_retry_times"`
	SentinelUsername string   `json:"sentinel_username" yaml:"sentinel_username" ini:"sentinel_username"`
	SentinelPassword string   `json:"sentinel_password" yaml:"sentinel_password" ini:"sentinel_password"`
}

type Elasticsearch struct {
	Server   []string `json:"server" yaml:"server" ini:"server"`
	Username string   `json:"username" yaml:"username" ini:"username"`
	Password string   `json:"password" yaml:"password" ini:"password"`
	CloudId  string   `json:"cloud_id" yaml:"cloud_id" ini:"cloud_id"`
	APIKey   string   `json:"api_key" yaml:"api_key" ini:"api_key"`
}

type MongoConf struct {
	Host            string `json:"host" yaml:"host" ini:"host"`
	Port            int    `json:"port" yaml:"port" ini:"port"`
	User            string `json:"user" yaml:"user" ini:"user"`
	Password        string `json:"password" yaml:"password" ini:"password"`
	MaxPoolSize     int    `json:"max_pool_size" yaml:"max_pool_size" ini:"max_pool_size"`
	ConnectTimeout  int    `json:"connect_timeout" yaml:"connect_timeout" ini:"connect_timeout"`
	MaxConnIdleTime int    `json:"max_conn_idle_time" yaml:"max_conn_idle_time" ini:"max_conn_idle_time"`
}

type RabbitMQConf struct {
	Server      string    `json:"server" yaml:"server" ini:"server"`
	Port        int       `json:"port" yaml:"port" ini:"port"`
	User        string    `json:"user" yaml:"user" ini:"user"`
	Password    string    `json:"password" yaml:"password" ini:"password"`
	LimitSwitch int       `json:"limit_switch" yaml:"limit_switch" ini:"limit_switch"`
	RmqConf     LimitConf `json:"rmq_conf" yaml:"rmq_conf" ini:"rmq_conf"`
}

type LimitConf struct {
	AttemptTimes    int `json:"attempt_times" yaml:"attempt_times" ini:"attempt_times"`
	RetryTimeSecond int `json:"retry_time_second" yaml:"retry_time_second" ini:"retry_time_second"`
	PrefetchCount   int `json:"prefetch_count" yaml:"prefetch_count" ini:"prefetch_count"`
	Timeout         int `json:"timeout" yaml:"timeout" ini:"timeout"`
	QueueLimit      int `json:"queue_limit" yaml:"queue_limit" ini:"queue_limit"`
}

// Application 服务地址端口配置
type Application struct {
	Address string `json:"address" yaml:"address" ini:"address"`
	Port    int    `json:"port" yaml:"port" ini:"port"`
}

type Config struct {
	Debug       bool            `json:"debug" yaml:"debug" ini:"debug"`
	Crypto      []*CryptoConfig `json:"crypto" yaml:"crypto" ini:"crypto"`
	Application *Application    `json:"application" yaml:"application" ini:"application"`
	Kafka       *KafkaConfig    `json:"kafka" yaml:"kafka" ini:"kafka"`
	Database    *Database       `json:"database" yaml:"database" ini:"database"`
	Rabbitmq    *RabbitMQConf   `json:"rabbitmq" yaml:"rabbitmq" ini:"rabbitmq"`
	Redis       *RedisConfig    `json:"redis" yaml:"redis" ini:"redis"`
	ES          *Elasticsearch  `json:"es" yaml:"es" ini:"es"`
	Mongo       *MongoConf      `json:"mongo" yaml:"mongo" ini:"mongo"`
}

var applicationConfig = new(Config)

func GetConfig() *Config {
	return applicationConfig
}

func LoadConfigFromJson(configPath string) {
	// 打开文件
	file, _ := os.Open(configPath)
	// 关闭文件
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			panic(err)
		}
	}(file)
	//NewDecoder创建一个从file读取并解码json对象的*Decoder，解码器有自己的缓冲，并可能超前读取部分json数据。
	decoder := decoder.NewStreamDecoder(file)
	//Decode从输入流读取下一个json编码值并保存在v指向的值里
	err := decoder.Decode(applicationConfig)
	if err != nil {
		panic(err)
	}

}

func LoadConfigFromIni(configPath string) {
	err := ini.MapTo(applicationConfig, configPath)
	if err != nil {
		panic(err)
	}
}

func LoadConfigFromYaml(configPath string) {
	file, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(file, applicationConfig)
	if err != nil {
		panic(err)
	}
}

func LoadConfigFromToml(configPath string) {
	_, err := toml.DecodeFile(configPath, applicationConfig)
	if err != nil {
		panic(err)
	}
}

func ParseConfigByViper(configPath, configName, configType string) {
	v := viper.New()
	v.AddConfigPath(configPath)
	v.SetConfigName(configName)
	v.SetConfigType(configType)

	if err := v.ReadInConfig(); err != nil {
		panic(err)
	}
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		if err := v.ReadInConfig(); err != nil {
			panic(err)
		}
	})
	//直接反序列化为Struct
	if err := v.Unmarshal(applicationConfig); err != nil {
		panic(err)
	}
	return
}

func init() {
	var DefaultApplicationYamlConfigPath = "./conf/application.yaml"
	var BakApplicationYamlConfigPath = "./conf/app.yaml"
	var DefaultApplicationIniConfigPath = "./conf/application.ini"
	var BakApplicationIniConfigPath = "./conf/app.ini"
	var DefaultApplicationTomlConfigPath = "./conf/application.toml"
	var BakApplicationTomlConfigPath = "./conf/app.toml"
	// 读取服务配置文件
	var configLoad bool
	if !configLoad && utils.IsExist(DefaultApplicationYamlConfigPath) {
		LoadConfigFromYaml(DefaultApplicationYamlConfigPath)
		configLoad = true
	}

	if !configLoad && utils.IsExist(BakApplicationYamlConfigPath) {
		LoadConfigFromYaml(BakApplicationYamlConfigPath)
		configLoad = true
	}

	if !configLoad && utils.IsExist(BakApplicationIniConfigPath) {
		LoadConfigFromIni(BakApplicationIniConfigPath)
		configLoad = true
	}

	if !configLoad && utils.IsExist(DefaultApplicationIniConfigPath) {
		LoadConfigFromIni(DefaultApplicationIniConfigPath)
		configLoad = true
	}

	if !configLoad && utils.IsExist(DefaultApplicationTomlConfigPath) {
		LoadConfigFromToml(DefaultApplicationTomlConfigPath)
		configLoad = true
	}

	if !configLoad && utils.IsExist(BakApplicationTomlConfigPath) {
		LoadConfigFromToml(BakApplicationTomlConfigPath)
		configLoad = true
	}

	if !configLoad {
		log.Printf("-- There is no application config.")
	}
}
