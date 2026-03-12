package main

import (
	"fmt"
	_ "go_project/ms_project/project_api/api"
	"go_project/ms_project/project_api/api/midd"
	"go_project/ms_project/project_api/config"
	"go_project/ms_project/project_api/router"
	srv "go_project/ms_project/project_common"
	"go_project/ms_project/project_common/encrypts"
	"net/http"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
)

func main() {
	fmt.Println(encrypts.DecryptNoErr("e08c"))
	r := gin.Default()

	r.Use(midd.RequestLog())
	// 全局限流：整个网关每秒最多处理 200 个请求，允许突发 50 个
	r.Use(midd.GlobalRateLimit(200, 50))
	// 按 IP 限流：每个 IP 每秒最多 20 个请求，允许突发 10 个
	r.Use(midd.PerIPRateLimit(20, 10))

	r.StaticFS("/upload", http.Dir("upload"))
	//设置路由
	router.InitRouter(r)
	pprof.Register(r)
	srv.Run(r, config.C.SC.Name, config.C.SC.Addr, nil)
}
