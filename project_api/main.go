package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	_ "go_project/ms_project/project_api/api"
	"go_project/ms_project/project_api/config"
	"go_project/ms_project/project_api/router"
	srv "go_project/ms_project/project_common"
	"log"
)

func main() {
	log.Println("debug here")
	r := gin.Default()
	fmt.Println("debug here")
	//设置路由
	router.InitRouter(r)
	srv.Run(r, config.C.SC.Name, config.C.SC.Addr, nil)
}
