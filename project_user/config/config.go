package config

import (
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go_project/ms_project/project_common/logs"
	"go_project/ms_project/util"
	"log"
	"os"
)

type Config struct {
	viper *viper.Viper
	Sc    *ServerConfig
}

var C = InitConfig()

type ServerConfig struct {
	Name string
	Addr string
}

func InitConfig() *Config {
	conf := &Config{viper: viper.New()}
	workDir, _ := os.Getwd()
	conf.viper.SetConfigName("config")                                    // 配置文件名
	conf.viper.SetConfigType("yaml")                                      // 配置文件类型
	conf.viper.AddConfigPath(workDir + "/ms_project/project_user/config") // 配置文件路径
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
	c.Sc = sc
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

func (c *Config) ReadRedisConfig() *redis.Options {
	return &redis.Options{
		Addr:     util.GetWslIP() + ":" + c.viper.GetString("redis.port"),
		Password: c.viper.GetString("redis.password"),
		DB:       c.viper.GetInt("redis.db"),
		Username: c.viper.GetString("redis.username"),
	}
}
