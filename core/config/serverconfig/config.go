package serverconfig

import (
	"fmt"
	"log"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/bytedance/sonic/decoder"
	"github.com/fsnotify/fsnotify"
	"github.com/go-ini/ini"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
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
	Key  string `mapstructure:"key" json:"key" ini:"key" yaml:"key" toml:"key"`
}

// KafkaConfig 配置
type KafkaConfig struct {
	Topic            []string `json:"topic" yaml:"topic"`
	GroupId          string   `json:"group_id" yaml:"group_id"`
	BootstrapServers string   `json:"bootstrap_servers" yaml:"bootstrap_servers"`
	SecurityProtocol string   `json:"security_protocol" yaml:"security_protocol"`
	SaslMechanism    string   `json:"sasl_mechanism" yaml:"sasl_mechanism"`
	SaslUsername     string   `json:"sasl_username" yaml:"sasl_username"`
	SaslPassword     string   `json:"sasl_password" yaml:"sasl_password"`
}

// Database 连接配置
type Database struct {
	DBType          string `json:"db_type" yaml:"db_type"`
	DSN             string `json:"dsn" yaml:"dsn"`
	LogMode         string `json:"log_mode" yaml:"log_mode"`
	Host            string `json:"host" yaml:"host"`
	Port            int    `json:"port" yaml:"port"`
	Database        string `json:"database" yaml:"database"`
	Username        string `json:"username" yaml:"username"`
	Password        string `json:"password" yaml:"password"`
	Charset         string `json:"charset" yaml:"charset"`
	MaxIdleConn     int    `json:"max_idle_conn" yaml:"max_idle_conn"`
	MaxOpenConn     int    `json:"max_open_conn" yaml:"max_open_conn"`
	ConnMaxLifeTime int64  `json:"conn_max_life_time" yaml:"conn_max_life_time"`
}

// RedisConfig 连接配置
type RedisConfig struct {
	Address           string `json:"address" yaml:"address"`
	Port              int    `json:"port" yaml:"port"`
	Password          string `json:"password" yaml:"password"`
	IndexDb           int    `json:"index_db" yaml:"index_db"`
	MaxIdleConn       int    `json:"max_idle_conn" yaml:"max_idle_conn"`
	MaxOpenConn       int    `json:"max_open_conn" yaml:"max_open_conn"`
	MaxActive         int    `json:"max_active" yaml:"max_active"`
	ConnTimeout       int64  `json:"conn_timeout" yaml:"conn_timeout"`
	MaxRetryTimes     int    `json:"max_retry_times" yaml:"max_retry_times"`
	ReConnectInterval int64  `json:"re_connect_interval" yaml:"re_connect_interval"`
}

type Elasticsearch struct {
	Host     []string `json:"host" yaml:"host"`
	Username string   `json:"username" yaml:"username"`
	Password string   `json:"password" yaml:"password"`
	CloudId  string   `json:"cloud_id" yaml:"cloud_id"`
	APIKey   string   `json:"api_key" yaml:"api_key"`
}

type MongoConf struct {
	Host            string `json:"host" yaml:"host"`
	User            string `json:"user" yaml:"user"`
	Password        string `json:"password" yaml:"password"`
	MaxPoolSize     int    `json:"max_pool_size" yaml:"max_pool_size"`
	ConnectTimeout  int    `json:"connect_timeout" yaml:"connect_timeout"`
	MaxConnIdleTime int    `json:"max_conn_idle_time" yaml:"max_conn_idle_time"`
}

// WebServer 服务地址端口配置
type WebServer struct {
	Address string `json:"address" yaml:"address"`
	Port    int    `json:"port" yaml:"port"`
}

type config struct {
	Debug     bool            `json:"debug" yaml:"debug"`
	Crypto    []*CryptoConfig `json:"crypto" yaml:"crypto"`
	WebConfig *WebServer      `json:"web_config" yaml:"web_config"`
	Kafka     *KafkaConfig    `json:"kafka" yaml:"kafka"`
	Database  *Database       `json:"database" yaml:"database"`
	Redis     *RedisConfig    `json:"redis" yaml:"redis"`
	ES        *Elasticsearch  `json:"es" yaml:"es"`
	Mongo     *MongoConf      `json:"mongo" yaml:"mongo"`
}

var serverConfig = new(config)

func GetConfig() *config {
	return serverConfig
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
	err := decoder.Decode(serverConfig)
	if err != nil {
		panic(err)
	}

}

func LoadConfigFromIni(configPath string) {
	err := ini.MapTo(serverConfig, configPath)
	if err != nil {
		log.Println(err)
		return
	}
}

func LoadConfigFromYaml(configPath string) {
	file, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(file, serverConfig)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func LoadConfigFromToml(configPath string) {
	_, err := toml.DecodeFile(configPath, serverConfig)
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
	if err := v.Unmarshal(serverConfig); err != nil {
		log.Println(err)
	}
	return
}
