package config

import (
	"context"
	"github.com/redis/go-redis/v9"
	"go_project/ms_project/project_project/internal/dao"
	"log"
)

func (c *Config) ReConnRedis() {
	// 检查是否已有连接，如果有，先关闭旧连接
	if dao.Rc != nil && dao.Rc.Rdb != nil {
		err := dao.Rc.Rdb.Close()
		if err != nil {
			log.Printf("关闭旧Redis连接失败: %v", err)
		}
	}

	// 创建新连接
	rdb := redis.NewClient(c.ReadRedisConfig())

	// Ping 确保连接成功
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Printf("Redis 连接失败: %v", err)
		return
	}

	if dao.Rc == nil {
		dao.Rc = &dao.RedisCache{}
	}
	dao.Rc.Rdb = rdb

	log.Println("Redis 客户端初始化/重载完成")
}
