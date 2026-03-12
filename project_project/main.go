package main

import (
	"github.com/gin-gonic/gin"
	srv "go_project/ms_project/project_common"
	"go_project/ms_project/project_common/kk"
	"go_project/ms_project/project_project/config"
	"go_project/ms_project/project_project/router"
)

func main() {
	r := gin.Default()
	//设置当前服务名，用于 Kafka 日志标识来源
	kk.SetServiceName("project")
	//设置路由
	router.InitRouter(r)
	//初始化rpc调用
	router.InitUserRpc()
	//grpc服务注册
	gc := router.RegisterGrpc()
	router.RegisterEtcdServer()
	//初始化kafka
	c := config.InitKafkaWriter()
	stop := func() {
		gc.Stop()
		c()
	}

	srv.Run(r, config.C.SC.Name, config.C.SC.Addr, stop)
}
