package main

import (
	"github.com/gin-gonic/gin"
	srv "go_project/ms_project/project_common"
	_ "go_project/ms_project/project_user/api"
	"go_project/ms_project/project_user/config"
	"go_project/ms_project/project_user/router"
	"os/exec"
	"strings"
)

func GetWslIP() string {
	cmd := exec.Command("wsl", "hostname", "-I")
	out, err := cmd.Output()
	if err != nil {
		return "127.0.0.1" // 回退方案
	}
	ips := strings.TrimSpace(string(out))
	return strings.Split(ips, " ")[0] // 取第一个IP
}

func main() {
	r := gin.Default()
	//路由
	router.InitRouter(r)
	srv.Run(r, config.C.Sc.Name, config.C.Sc.Addr)
}
