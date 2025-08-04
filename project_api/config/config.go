package config

import (
	"go_project/ms_project/project_common/logs"
	"log"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	viper      *viper.Viper
	SC         *ServerConfig
	GC         *GrpcConfig
	EtcdConfig *EtcdConfig
}

var C = InitConfig()

type ServerConfig struct {
	Name string
	Addr string
}

type GrpcConfig struct {
	Name string
	Addr string
}

type EtcdConfig struct {
	Addrs []string
}

func InitConfig() *Config {
	conf := &Config{viper: viper.New()}
	workDir, _ := os.Getwd()
	conf.viper.SetConfigName("config") // 配置文件名
	conf.viper.SetConfigType("yaml")   // 配置文件类型
	conf.viper.AddConfigPath("/etc/ms_project/user")
	conf.viper.AddConfigPath(workDir + "/config") // 配置文件路径
	if err := conf.viper.ReadInConfig(); err != nil {
		log.Fatalln("api读取配置文件失败:", err)
	}
	conf.ReadServerConfig()
	conf.ReadEtcdConfig()
	conf.InitZapLog()
	return conf
}

func (c *Config) ReadServerConfig() {
	sc := &ServerConfig{}
	sc.Name = c.viper.GetString("server.name")
	sc.Addr = c.viper.GetString("server.addr")
	c.SC = sc
}

func (c *Config) ReadEtcdConfig() {
	ec := &EtcdConfig{}
	var addrs []string
	if err := c.viper.UnmarshalKey("etcd.addrs", &addrs); err != nil {
		log.Fatalln("读取etcd配置失败:", err)
	}
	ec.Addrs = addrs
	c.EtcdConfig = ec
}

func (c *Config) InitZapLog() {
	lc := &logs.LogConfig{
		DebugFileName: c.viper.GetString("zap.debugFileName"),
		InfoFileName:  c.viper.GetString("zap.infoFileName"),
		WarnFileName:  c.viper.GetString("zap.warnFileName"),
		MaxSize:       c.viper.GetInt("maxSize"),
		MaxAge:        c.viper.GetInt("maxAge"),
		MaxBackups:    c.viper.GetInt("maxBackups"),
	}
	err := logs.InitLogger(lc)
	if err != nil {
		log.Fatalln("初始化日志失败:", err)
	}
}
