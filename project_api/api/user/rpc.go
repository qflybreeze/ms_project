package user

import (
	"go_project/ms_project/project_api/config"
	"go_project/ms_project/project_common/discovery"
	"go_project/ms_project/project_common/logs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"log"
	"project_grpc/user/login"
)

var LoginServiceClient login.LoginServiceClient

func InitRpcUserClient() {
	etcdRegister := discovery.NewResolver(config.C.EtcdConfig.Addrs, logs.LG)
	resolver.Register(etcdRegister)

	conn, err := grpc.Dial("etcd:///user", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	LoginServiceClient = login.NewLoginServiceClient(conn)
}
