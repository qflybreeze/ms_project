package main

import (
	"github.com/gin-gonic/gin"
	srv "go_project/ms_project/project_common"
	"go_project/ms_project/project_user/config"
	"go_project/ms_project/project_user/router"
)

func main() {
	r := gin.Default()
	//设置路由
	router.InitRouter(r)
	gc := router.RegisterGrpc()
	stop := func() {
		gc.Stop()
	}
	srv.Run(r, config.C.SC.Name, config.C.SC.Addr, stop)
}
