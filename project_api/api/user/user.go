package user

import (
	"context"
	"github.com/gin-gonic/gin"
	common "go_project/ms_project/project_common"
	"go_project/ms_project/project_common/errs"
	logServiceV1 "go_project/ms_project/project_user/pkg/service/login_service_v1"
	"net/http"
	"time"
)

type HandlerUser struct {
}

func New() *HandlerUser {
	return &HandlerUser{}
}

func (*HandlerUser) getCaptcha(ctx *gin.Context) {
	result := &common.Result{}
	mobile := ctx.PostForm("mobile")
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rsp, err := LoginServiceClient.GetCaptcha(c, &logServiceV1.CaptchaMessage{Mobile: mobile})
	if err != nil {
		code, msg := errs.ParseGrpcError(err)
		ctx.JSON(http.StatusOK, result.Fail(code, msg))
		return
	}
	ctx.JSON(http.StatusOK, result.Success(rsp.Code))
}
