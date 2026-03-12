package login_service_v1

import (
	"context"
	"encoding/json"
	"fmt"
	common "go_project/ms_project/project_common"
	"go_project/ms_project/project_common/encrypts"
	"go_project/ms_project/project_common/errs"
	"go_project/ms_project/project_common/jwts"
	"go_project/ms_project/project_common/tms"
	"go_project/ms_project/project_grpc/user/login"
	"go_project/ms_project/project_user/config"
	"go_project/ms_project/project_user/internal/dao"
	"go_project/ms_project/project_user/internal/data/member"
	"go_project/ms_project/project_user/internal/data/organization"
	"go_project/ms_project/project_user/internal/database"
	"go_project/ms_project/project_user/internal/database/tran"
	"go_project/ms_project/project_user/internal/interceptor"
	"go_project/ms_project/project_user/internal/repo"
	"go_project/ms_project/project_user/pkg/model"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/copier"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type LoginService struct {
	login.UnimplementedLoginServiceServer
	cache            repo.Cache
	memberRepo       repo.MemberRepo
	organizationRepo repo.OrganizationRepo
	transaction      tran.Transaction
}

func New() *LoginService {
	return &LoginService{
		cache:            dao.Rc,
		memberRepo:       dao.NewMemberDao(),
		organizationRepo: dao.NewOrganizationDao(),
		transaction:      dao.NewTransaction(),
	}
}

func (ls *LoginService) GetCaptcha(ctx context.Context, msg *login.CaptchaMessage) (*login.CaptchaResponse, error) {
	//获取参数
	mobile := msg.Mobile
	//校验参数
	if !common.VerifyMobile(mobile) {
		return nil, errs.GrpcError(model.NoLegalMobile)
	}
	//生成六位随机验证码
	code := fmt.Sprintf("%06d", rand.Intn(900000)+100000)
	//同步写入 Redis，确保注册接口能立刻读到验证码
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := ls.cache.Put(c, model.RegisterRedisKey+mobile, code, 15*time.Minute)
	if err != nil {
		zap.L().Error("存储验证码到redis失败", zap.String("mobile", mobile), zap.Error(err))
		return nil, errs.GrpcError(model.RedisError)
	}
	go func() {
		zap.L().Info("短信平台调用成功，发送短信", zap.String("mobile", mobile))
	}()
	return &login.CaptchaResponse{Code: code}, nil
}

func (ls *LoginService) Register(ctx context.Context, msg *login.RegisterMessage) (*login.RegisterResponse, error) {
	c := context.Background()
	//校验验证码
	redisCode, err := ls.cache.Get(c, model.RegisterRedisKey+msg.Mobile)

	if err == redis.Nil {
		return nil, errs.GrpcError(model.CaptchaNotExist)
	}
	if err != nil {
		zap.L().Error("Register从redis获取验证码失败", zap.Error(err))
		return nil, errs.GrpcError(model.RedisError)
	}
	if redisCode != msg.Captcha {
		return nil, errs.GrpcError(model.CaptchaError)
	}
	//校验业务逻辑数据是否冲突
	exist, err := ls.memberRepo.GetMemberByEmail(c, msg.Email)
	if err != nil {
		zap.L().Error("Register从db获取用户信息失败", zap.Error(err))
		return nil, errs.GrpcError(model.DBError)
	}
	if exist {
		return nil, errs.GrpcError(model.EmailExist)
	}

	exist, err = ls.memberRepo.GetMemberByAccount(c, msg.Name)
	if err != nil {
		zap.L().Error("Register从db获取用户信息失败", zap.Error(err))
		return nil, errs.GrpcError(model.DBError)
	}
	if exist {
		return nil, errs.GrpcError(model.AccountExist)
	}

	exist, err = ls.memberRepo.GetMemberByMobile(c, msg.Mobile)
	if err != nil {
		zap.L().Error("Register从db获取用户信息失败", zap.Error(err))
		return nil, errs.GrpcError(model.DBError)
	}
	if exist {
		return nil, errs.GrpcError(model.MobileExist)
	}
	//将数据存入member表，使用 bcrypt 哈希密码
	pwd, err := encrypts.HashPassword(msg.Password)
	if err != nil {
		zap.L().Error("Register bcrypt hash password error", zap.Error(err))
		return nil, errs.GrpcError(model.DBError)
	}
	mem := &member.Member{
		Account:       msg.Name,
		Password:      pwd,
		Name:          msg.Name,
		Mobile:        msg.Mobile,
		Email:         msg.Email,
		CreateTime:    time.Now().UnixMilli(),
		LastLoginTime: time.Now().UnixMilli(),
		Status:        model.Normal,
	}
	err = ls.transaction.Action(func(conn database.DbConn) error {
		err = ls.memberRepo.SaveMember(conn, c, mem)
		if err != nil {
			zap.L().Error("Register向db存储用户信息失败", zap.Error(err))
			return errs.GrpcError(model.DBError)
		}
		//将数据存入organization表
		org := &organization.Organization{
			Name:       mem.Name + "个人组织",
			MemberId:   mem.Id,
			CreateTime: time.Now().UnixMilli(),
			Personal:   model.Personal,
			Avatar:     "https://gimg2.baidu.com/image_search/src=http%3A%2F%2Fc-ssl.dtstatic.com%2Fuploads%2Fblog%2F202103%2F31%2F20210331160001_9a852.thumb.1000_0.jpg&refer=http%3A%2F%2Fc-ssl.dtstatic.com&app=2002&size=f9999,10000&q=a80&n=0&g=0n&fmt=auto?sec=1673017724&t=ced22fc74624e6940fd6a89a21d30cc5",
		}
		err = ls.organizationRepo.SaveOrganization(conn, c, org)
		if err != nil {
			zap.L().Error("register SaveOrganization db err", zap.Error(err))
			return model.DBError
		}
		return nil
	})
	return &login.RegisterResponse{}, err
}

func (ls *LoginService) Login(ctx context.Context, msg *login.LoginMessage) (*login.LoginResponse, error) {
	c := context.Background()
	// 先通过账号查询用户（不带密码条件，因为 bcrypt 哈希每次不同，无法在 SQL 中比较）
	mem, err := ls.memberRepo.FindMemberByAccount(c, msg.Account)
	if err != nil {
		zap.L().Error("Login向db查询用户信息失败", zap.Error(err))
		return nil, errs.GrpcError(model.DBError)
	}
	if mem == nil {
		return nil, errs.GrpcError(model.AccountOrPwdError)
	}
	//更新密码为bcrypt哈希
	if isBcryptHash(mem.Password) {
		if !encrypts.VerifyPassword(msg.Password, mem.Password) {
			return nil, errs.GrpcError(model.AccountOrPwdError)
		}
	} else {
		if encrypts.Md5(msg.Password) != mem.Password {
			return nil, errs.GrpcError(model.AccountOrPwdError)
		}
		newHash, err := encrypts.HashPassword(msg.Password)
		if err == nil {
			if upErr := ls.memberRepo.UpdateMemberPassword(c, mem.Id, newHash); upErr != nil {
				zap.L().Warn("auto-upgrade password to bcrypt failed", zap.Int64("memberId", mem.Id), zap.Error(upErr))
			} else {
				zap.L().Info("password auto-upgraded to bcrypt", zap.Int64("memberId", mem.Id))
			}
		}
	}
	memMsg := &login.MemberMessage{}
	err = copier.Copy(memMsg, mem)
	memMsg.Code, _ = encrypts.EncryptInt64(memMsg.Id, model.AESKey)
	memMsg.LastLoginTime = tms.FormatByMill(mem.LastLoginTime)
	memMsg.CreateTime = tms.FormatByMill(mem.CreateTime)
	//根据id查询组织
	orgs, err := ls.organizationRepo.FindOrganizationByMemId(c, mem.Id)
	if err != nil {
		zap.L().Error("Login向db查询用户信息失败", zap.Error(err))
		return nil, errs.GrpcError(model.DBError)
	}
	var orgsMessage []*login.OrganizationMessage
	err = copier.Copy(&orgsMessage, orgs)
	for _, v := range orgsMessage {
		v.Code, _ = encrypts.EncryptInt64(v.Id, model.AESKey)
		v.OwnerCode = memMsg.Code
		o := organization.ToMap(orgs)[v.Id]
		v.CreateTime = tms.FormatByMill(o.CreateTime)
	}
	if len(orgs) > 0 {
		memMsg.OrganizationCode, _ = encrypts.EncryptInt64(orgs[0].Id, model.AESKey)
	}
	//jwt生成token
	memIdStr := strconv.FormatInt(mem.Id, 10)
	exp := time.Duration(config.C.JwtConfig.AccessExp*3600*24) * time.Second
	rExp := time.Duration(config.C.JwtConfig.RefreshExp*3600*24) * time.Second
	token := jwts.CreateToken(memIdStr, exp, config.C.JwtConfig.AccessSecret, rExp, config.C.JwtConfig.RefreshSecret, msg.Ip)
	//给token加密

	tokenList := &login.TokenMessage{
		AccessToken:    token.AccessToken,
		RefreshToken:   token.RefreshToken,
		AccessTokenExp: token.AccessExp,
		TokenType:      "bearer",
	}
	//member orgs放入缓存
	go func() {
		marshal, _ := json.Marshal(mem)
		memKey := model.Member + "::" + memIdStr
		ls.cache.Put(c, memKey, string(marshal), exp)
		orgsJson, _ := json.Marshal(orgs)
		orgKey := model.MemberOrganization + "::" + memIdStr
		ls.cache.Put(c, orgKey, string(orgsJson), exp)
		//布隆过滤器放行新用户登录后的请求
		interceptor.CacheClient.BloomAddKey(memKey)
		interceptor.CacheClient.BloomAddKey(orgKey)

		req := &login.UserMessage{MemId: mem.Id}
		if key, ok := interceptor.CacheClient.BuildCacheKey(
			"/login.service.v1.LoginService/MyOrgList", req); ok {
			interceptor.CacheClient.BloomAddKey(key)
		}
		if key, ok := interceptor.CacheClient.BuildCacheKey(
			"/login.service.v1.LoginService/FindMemInfoById", req); ok {
			interceptor.CacheClient.BloomAddKey(key)
		}
	}()
	return &login.LoginResponse{
		Member:           memMsg,
		OrganizationList: orgsMessage,
		TokenList:        tokenList,
	}, nil
}

// 用 RefreshToken 换取新的 AccessToken + RefreshToken
func (ls *LoginService) TokenRefresh(ctx context.Context, msg *login.LoginMessage) (*login.LoginResponse, error) {
	refreshTokenStr := msg.Token
	if strings.Contains(refreshTokenStr, "bearer ") {
		refreshTokenStr = strings.ReplaceAll(refreshTokenStr, "bearer ", "")
	}
	// 用 RefreshSecret 解析
	memIdStr, err := jwts.ParseRefreshToken(refreshTokenStr, config.C.JwtConfig.RefreshSecret)
	if err != nil {
		zap.L().Error("TokenRefresh解析refreshToken失败", zap.Error(err))
		return nil, errs.GrpcError(model.NoLogin)
	}
	memberId, _ := strconv.ParseInt(memIdStr, 10, 64)

	// 查询用户是否存在
	memberById, err := ls.memberRepo.FindMemberById(ctx, memberId)
	if err != nil || memberById == nil {
		return nil, errs.GrpcError(model.NoLogin)
	}

	// 签发新的双 Token（绑定当前请求的 IP）
	exp := time.Duration(config.C.JwtConfig.AccessExp*3600*24) * time.Second
	rExp := time.Duration(config.C.JwtConfig.RefreshExp*3600*24) * time.Second
	newToken := jwts.CreateToken(memIdStr, exp, config.C.JwtConfig.AccessSecret, rExp, config.C.JwtConfig.RefreshSecret, msg.Ip)

	tokenList := &login.TokenMessage{
		AccessToken:    newToken.AccessToken,
		RefreshToken:   newToken.RefreshToken,
		AccessTokenExp: newToken.AccessExp,
		TokenType:      "bearer",
	}
	return &login.LoginResponse{
		TokenList: tokenList,
	}, nil
}

func (ls *LoginService) TokenVerify(ctx context.Context, msg *login.LoginMessage) (*login.LoginResponse, error) {
	token := msg.Token
	if strings.Contains(token, "bearer ") {
		token = strings.ReplaceAll(token, "bearer ", "")
	}
	parseToken, err := jwts.ParseToken(token, config.C.JwtConfig.AccessSecret, msg.Ip)
	if err != nil {
		zap.L().Error("TokenVerify解析token失败", zap.Error(err))
		return nil, errs.GrpcError(model.NoLogin)
	}

	id, _ := strconv.ParseInt(parseToken, 10, 64)
	exp := time.Duration(config.C.JwtConfig.AccessExp*3600*24) * time.Second

	// 获取用户信息
	memKey := model.Member + "::" + parseToken
	var memberById *member.Member

	memJson, _ := ls.cache.Get(context.Background(), memKey)
	if memJson != "" {
		// 缓存命中
		memberById = &member.Member{}
		json.Unmarshal([]byte(memJson), memberById)
	} else {
		// 缓存未命中，回源 DB
		zap.L().Warn("TokenVerify member 缓存未命中，回源DB", zap.String("memberId", parseToken))
		memberById, err = ls.memberRepo.FindMemberById(context.Background(), id)
		if err != nil {
			zap.L().Error("TokenVerify从DB查询用户信息失败", zap.Error(err))
			return nil, errs.GrpcError(model.NoLogin)
		}
		// 异步回填缓存和布隆过滤器
		go func() {
			if marshal, merr := json.Marshal(memberById); merr == nil {
				putCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				ls.cache.Put(putCtx, memKey, string(marshal), exp)
				interceptor.CacheClient.BloomAddKey(memKey)
			}
		}()
	}

	memMsg := &login.MemberMessage{}
	copier.Copy(memMsg, memberById)
	memMsg.Code, _ = encrypts.EncryptInt64(memMsg.Id, model.AESKey)

	// 获取组织信息
	orgKey := model.MemberOrganization + "::" + parseToken
	var orgs []*organization.Organization

	orgsJson, _ := ls.cache.Get(context.Background(), orgKey)
	if orgsJson != "" {
		// 缓存命中
		json.Unmarshal([]byte(orgsJson), &orgs)
	} else {
		// 缓存未命中，回源 DB
		zap.L().Warn("TokenVerify org 缓存未命中，回源DB", zap.String("memberId", parseToken))
		orgs, err = ls.organizationRepo.FindOrganizationByMemId(context.Background(), id)
		if err != nil {
			zap.L().Error("TokenVerify从DB查询组织信息失败", zap.Error(err))
			return nil, errs.GrpcError(model.NoLogin)
		}
		// 异步回填
		go func() {
			if orgJson, oerr := json.Marshal(orgs); oerr == nil {
				putCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				ls.cache.Put(putCtx, orgKey, string(orgJson), exp)
				interceptor.CacheClient.BloomAddKey(orgKey)
			}
		}()
	}

	if len(orgs) > 0 {
		memMsg.OrganizationCode, _ = encrypts.EncryptInt64(orgs[0].Id, model.AESKey)
	}
	memMsg.CreateTime = tms.FormatByMill(memberById.CreateTime)
	return &login.LoginResponse{Member: memMsg}, nil
}

func (l *LoginService) MyOrgList(ctx context.Context, msg *login.UserMessage) (*login.OrgListResponse, error) {
	fmt.Println("MyOrgList")
	memId := msg.MemId
	orgs, err := l.organizationRepo.FindOrganizationByMemId(ctx, memId)
	if err != nil {
		zap.L().Error("MyOrgList FindOrganizationByMemId err", zap.Error(err))
		return nil, errs.GrpcError(model.DBError)
	}
	var orgsMessage []*login.OrganizationMessage
	err = copier.Copy(&orgsMessage, orgs)
	for _, org := range orgsMessage {
		org.Code, _ = encrypts.EncryptInt64(org.Id, model.AESKey)
	}
	return &login.OrgListResponse{OrganizationList: orgsMessage}, nil
}

func (ls *LoginService) FindMemInfoById(ctx context.Context, msg *login.UserMessage) (*login.MemberMessage, error) {
	memberById, err := ls.memberRepo.FindMemberById(context.Background(), msg.MemId)
	if err != nil {
		zap.L().Error("FindMemInfoById向db查询用户信息失败", zap.Error(err))
		return nil, errs.GrpcError(model.DBError)
	}
	memMsg := &login.MemberMessage{}
	copier.Copy(memMsg, memberById)
	memMsg.Code, _ = encrypts.EncryptInt64(memMsg.Id, model.AESKey)
	orgs, err := ls.organizationRepo.FindOrganizationByMemId(context.Background(), memberById.Id)
	if err != nil {
		zap.L().Error("FindMemInfoById向db查询用户信息失败", zap.Error(err))
		return nil, errs.GrpcError(model.DBError)
	}
	if len(orgs) > 0 {
		memMsg.OrganizationCode, _ = encrypts.EncryptInt64(orgs[0].Id, model.AESKey)
	}
	memMsg.CreateTime = tms.FormatByMill(memberById.CreateTime)
	memMsg.Code = encrypts.EncryptNoErr(memMsg.Id)
	return memMsg, nil
}

func (ls *LoginService) FindMemInfoByIds(ctx context.Context, msg *login.UserMessage) (*login.MemberMessageList, error) {
	memberList, err := ls.memberRepo.FindMemberByIds(context.Background(), msg.MIds)
	if err != nil {
		zap.L().Error("FindMemInfoByIds向db查询用户信息失败", zap.Error(err))
		return nil, errs.GrpcError(model.DBError)
	}
	if memberList == nil || len(memberList) <= 0 {
		return &login.MemberMessageList{List: nil}, nil
	}
	mMap := make(map[int64]*member.Member)
	for _, v := range memberList {
		mMap[v.Id] = v
	}
	var memMsgs []*login.MemberMessage
	copier.Copy(&memMsgs, memberList)
	for _, v := range memMsgs {
		m := mMap[v.Id]
		v.CreateTime = tms.FormatByMill(m.CreateTime)
		v.Code = encrypts.EncryptNoErr(v.Id)
	}

	return &login.MemberMessageList{List: memMsgs}, nil
}

// bcrypt 哈希固定以 "$2a$" 或 "$2b$" 开头，长度 60 字符
func isBcryptHash(hash string) bool {
	return len(hash) == 60 && strings.HasPrefix(hash, "$2")
}
