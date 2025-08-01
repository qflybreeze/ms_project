package router

import (
	"github.com/gin-gonic/gin"
	"go_project/ms_project/project_common/discovery"
	"go_project/ms_project/project_common/logs"
	"go_project/ms_project/project_user/config"
	loginServiceV1 "go_project/ms_project/project_user/pkg/service/login_service_v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
	"log"
	"net"
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

type gRPCConfig struct {
	Addr         string
	RegisterFunc func(*grpc.Server)
}

func RegisterGrpc() *grpc.Server {
	c := gRPCConfig{
		Addr: config.C.GC.Addr,
		RegisterFunc: func(g *grpc.Server) {
			loginServiceV1.RegisterLoginServiceServer(g, loginServiceV1.New())
		},
	}
	s := grpc.NewServer()
	c.RegisterFunc(s)
	lis, err := net.Listen("tcp", c.Addr)
	if err != nil {
		log.Println("gRPC server listen error:", err)
	}
	go func() {
		err := s.Serve(lis)
		if err != nil {
			log.Println("gRPC server serve start error:", err)
			return
		}
	}()
	return s
}

func RegisterEtcdServer() {
	etcdRegister := discovery.NewResolver(config.C.EtcdConfig.Addrs, logs.LG)
	resolver.Register(etcdRegister)
	info := discovery.Server{
		Name:    config.C.GrpcConfig.Name,
		Addr:    config.C.GrpcConfig.Addr,
		Version: config.C.GrpcConfig.Version,
		Weight:  config.C.GrpcConfig.Weight,
	}
	r := discovery.NewRegister(config.C.EtcdConfig.Addrs, logs.LG)
	_, err := r.Register(info, 2)
	if err != nil {
		log.Fatalln(err)
	}
}
