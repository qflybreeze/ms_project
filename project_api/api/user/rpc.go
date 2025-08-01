package user

import (
	loginServiceV1 "go_project/ms_project/project_user/pkg/service/login_service_v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
)

var LoginServiceClient loginServiceV1.LoginServiceClient

func InitRpcUserClient() {
	conn, err := grpc.Dial("etcd:///user", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	LoginServiceClient = loginServiceV1.NewLoginServiceClient(conn)

}
