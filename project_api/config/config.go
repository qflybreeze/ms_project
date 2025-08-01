package config

import (
	"github.com/spf13/viper"
	"go_project/ms_project/project_common/logs"
	"log"
	"os"
)

type Config struct {
	viper *viper.Viper
	SC    *ServerConfig
	GC    *GrpcConfig
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

func InitConfig() *Config {
	conf := &Config{viper: viper.New()}
	workDir, _ := os.Getwd()
	conf.viper.SetConfigName("config")            // 配置文件名
	conf.viper.SetConfigType("yaml")              // 配置文件类型
	conf.viper.AddConfigPath(workDir + "/config") // 配置文件路径
	if err := conf.viper.ReadInConfig(); err != nil {
		log.Fatalln("读取配置文件失败:", err)
	}
	conf.ReadServerConfig()
	conf.InitZapLog()
	return conf
}

func (c *Config) ReadServerConfig() {
	sc := &ServerConfig{}
	sc.Name = c.viper.GetString("server.name")
	sc.Addr = c.viper.GetString("server.addr")
	c.SC = sc
}

func (c *Config) InitZapLog() {
	lc := &logs.LogConfig{
		DebugFileName: c.viper.GetString("zap.debug_file_name"),
		InfoFileName:  c.viper.GetString("zap.info_file_name"),
		WarnFileName:  c.viper.GetString("zap.warn_file_name"),
		MaxSize:       c.viper.GetInt("max_size"),
		MaxAge:        c.viper.GetInt("max_age"),
		MaxBackups:    c.viper.GetInt("max_backups"),
	}
	err := logs.InitLogger(lc)
	if err != nil {
		log.Fatalln("初始化日志失败:", err)
	}
}
