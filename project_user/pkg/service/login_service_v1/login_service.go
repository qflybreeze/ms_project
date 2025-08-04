package login_service_v1

import (
	"context"
	"go.uber.org/zap"
	common "go_project/ms_project/project_common"
	"go_project/ms_project/project_common/errs"
	"go_project/ms_project/project_user/pkg/dao"
	"go_project/ms_project/project_user/pkg/model"
	"go_project/ms_project/project_user/pkg/repo"
	"log"
	"time"
)

type LoginService struct {
	UnimplementedLoginServiceServer
	cache repo.Cache
}

func New() *LoginService {
	return &LoginService{
		cache: dao.Rc,
	}
}

func (ls *LoginService) GetCaptcha(ctx context.Context, msg *CaptchaMessage) (*CaptchaResponse, error) {
	//1.获取参数
	mobile := msg.Mobile
	//2.校验参数
	if !common.VerifyMobile(mobile) {
		return nil, errs.GrpcError(model.NoLegalMobile)
	}
	//3.生成四位或者六位验证码
	code := "123456"
	//4.调用短信平台
	go func() {
		time.Sleep(2 * time.Second)
		zap.L().Info("短信平台调用成功，发送短信")
		//5.存储验证码到redis，过期时间为15分钟
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := ls.cache.Put(c, "REGISTER_"+mobile, code, 15*time.Minute); err != nil {
			log.Printf("存储验证码到redis失败，手机号：%s，错误：%v", mobile, err)
		} else {
			log.Printf("存储验证码到redis成功，手机号：%s，验证码：%s", mobile, code)
		}
	}()
	return &CaptchaResponse{Code: code}, nil
}
