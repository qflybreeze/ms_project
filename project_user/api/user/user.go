package user

import (
	"context"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go_project/ms_project/pkg/dao"
	"go_project/ms_project/pkg/model"
	"go_project/ms_project/pkg/repo"
	common "go_project/ms_project/project_common"
	"log"
	"net/http"
	"time"
)

type HandlerUser struct {
	cache repo.Cache
}

func New() *HandlerUser {
	return &HandlerUser{
		cache: dao.Rc,
	}
}
func (h *HandlerUser) getCaptcha(ctx *gin.Context) {
	rsp := &common.Result{}
	//1.获取参数
	mobile := ctx.PostForm("mobile")
	//2.校验参数
	if !common.VerifyMobile(mobile) {
		ctx.JSON(http.StatusOK, rsp.Fail(model.NoLegalMobile, "手机号不合法"))
	}
	//3.生成四位或者六位验证码
	code := "123456"
	//4.调用短信平台
	go func() {
		time.Sleep(2 * time.Second)
		zap.L().Info("短信平台调用成功，发送短信")
		//5.存储验证码到redis，过期时间为15分钟
		c, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := h.cache.Put(c, "REGISTER_"+mobile, code, 15*time.Minute); err != nil {
			log.Printf("存储验证码到redis失败，手机号：%s，错误：%v", mobile, err)
		} else {
			log.Printf("存储验证码到redis成功，手机号：%s，验证码：%s", mobile, code)
		}
	}()
	//为了方便处理，直接返回验证码
	ctx.JSON(http.StatusOK, rsp.Success(code))
}
