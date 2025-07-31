package router

import (
	"github.com/gin-gonic/gin"
)

//负责路由注册

type Router interface {
	Route(r *gin.Engine)
}

type RegisterRouter struct {
}

func (*RegisterRouter) Route(router Router, r *gin.Engine) {
	router.Route(r)
}
func NewRouter() *RegisterRouter {
	return &RegisterRouter{}
}

var routers []Router

func InitRouter(r *gin.Engine) {
	for _, ro := range routers {
		ro.Route(r)
	}
}

func Register(ro ...Router) {
	routers = append(routers, ro...)
}
