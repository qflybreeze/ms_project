package router

import (
	"context"
	"encoding/json"
	"go_project/ms_project/project_common/discovery"
	"go_project/ms_project/project_common/logs"
	"go_project/ms_project/project_grpc/user/login"
	"go_project/ms_project/project_user/config"
	"go_project/ms_project/project_user/internal/dao"
	"go_project/ms_project/project_user/internal/data/member"
	"go_project/ms_project/project_user/internal/data/organization"
	"go_project/ms_project/project_user/internal/interceptor"
	"go_project/ms_project/project_user/pkg/model"
	loginServiceV1 "go_project/ms_project/project_user/pkg/service/login_service_v1"
	"log"
	"net"
	"strconv"
	"time"

	"go_project/ms_project/project_common/encrypts"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
	"google.golang.org/protobuf/proto"
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
			login.RegisterLoginServiceServer(g, loginServiceV1.New())
		},
	}
	// 异步预热
	go func() {
		keys, memberIds, ready := loadExistingKeysFromDB()
		if !ready {
			logs.LG.Warn("预热数据加载失败，跳过预热")
			return
		}
		interceptor.CacheClient.BloomAddKeys(keys)
		logs.LG.Info("布隆过滤器数据加载完成", zap.Int("key_count", len(keys)))

		if ok := prewarmLoginTokenVerifyCache(memberIds); !ok {
			logs.LG.Warn("Login/TokenVerify 缓存预热部分失败")
		}
		if ok := prewarmGrpcInterceptorCache(memberIds); !ok {
			logs.LG.Warn("gRPC 拦截器缓存预热部分失败")
		}

		// 全部预热完成后开启布隆过滤器
		interceptor.CacheClient.SetBloomReady(true)
		logs.LG.Info("异步预热全部完成，布隆过滤器已启用")
	}()

	s := grpc.NewServer(
		grpc.UnaryInterceptor(interceptor.CacheClient.CacheInterceptor()),
	)
	c.RegisterFunc(s)
	lis, err := net.Listen("tcp", c.Addr)
	if err != nil {
		log.Println("gRPC server listen error:", err)
	}
	go func() {
		log.Printf("gRPC server start at %s\n", c.Addr)
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
		Name:    config.C.GC.Name,
		Addr:    config.C.GC.Addr,
		Version: config.C.GC.Version,
		Weight:  config.C.GC.Weight,
	}
	r := discovery.NewRegister(config.C.EtcdConfig.Addrs, logs.LG)
	_, err := r.Register(info, 2)
	if err != nil {
		log.Fatalln(err)
	}
}

func loadExistingKeysFromDB() ([]string, []int64, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var keys []string

	// 查询所有有效的 member_id
	memberIds, err := dao.NewMemberDao().FindAllMemberIds(ctx)
	if err != nil {
		logs.LG.Warn("加载 member ids 失败", zap.Error(err))
		return keys, nil, false
	}

	// 构造缓存key
	for _, memberId := range memberIds {
		// MyOrgList 接口的请求参数
		req := &login.UserMessage{MemId: memberId}
		if key, ok := interceptor.CacheClient.BuildCacheKey("/login.service.v1.LoginService/MyOrgList", req); ok {
			keys = append(keys, key)
		}

		// FindMemInfoById 接口
		req2 := &login.UserMessage{MemId: memberId}
		if key2, ok := interceptor.CacheClient.BuildCacheKey("/login.service.v1.LoginService/FindMemInfoById", req2); ok {
			keys = append(keys, key2)
		}

		// 登录后写入缓存的 key
		memIdStr := strconv.FormatInt(memberId, 10)
		keys = append(keys, model.Member+"::"+memIdStr)
		keys = append(keys, model.MemberOrganization+"::"+memIdStr)
	}

	return keys, memberIds, true
}

func prewarmLoginTokenVerifyCache(memberIds []int64) bool {
	if len(memberIds) == 0 {
		return true
	}
	exp := time.Duration(config.C.JwtConfig.AccessExp*3600*24) * time.Second
	cache := dao.Rc
	memberDao := dao.NewMemberDao()
	orgDao := dao.NewOrganizationDao()
	const batchSize = 500
	success := true

	for i := 0; i < len(memberIds); i += batchSize {
		end := i + batchSize
		if end > len(memberIds) {
			end = len(memberIds)
		}
		batch := memberIds[i:end]
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		members, err := memberDao.FindMemberByIds(ctx, batch)
		if err != nil {
			logs.LG.Warn("预热缓存查询成员失败", zap.Error(err))
			success = false
			cancel()
			continue
		}
		orgMap, err := orgDao.FindOrganizationsByMemIds(ctx, batch)
		if err != nil {
			logs.LG.Warn("预热缓存查询组织失败", zap.Error(err))
			success = false
			orgMap = map[int64][]*organization.Organization{}
		}
		cancel()

		memberMap := make(map[int64]*member.Member, len(members))
		for _, m := range members {
			memberMap[m.Id] = m
		}

		for _, memberId := range batch {
			mem, ok := memberMap[memberId]
			if !ok {
				continue
			}
			memIdStr := strconv.FormatInt(memberId, 10)
			memKey := model.Member + "::" + memIdStr
			orgKey := model.MemberOrganization + "::" + memIdStr

			if memJson, merr := json.Marshal(mem); merr == nil {
				putCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				if err := cache.Put(putCtx, memKey, string(memJson), exp); err == nil {
					interceptor.CacheClient.BloomAddKey(memKey)
				} else {
					logs.LG.Warn("预热 MEMBER 缓存失败", zap.String("key", memKey), zap.Error(err))
					success = false
				}
				cancel()
			} else {
				logs.LG.Warn("预热 MEMBER 序列化失败", zap.Error(merr))
				success = false
			}

			orgs := orgMap[memberId]
			if orgJson, oerr := json.Marshal(orgs); oerr == nil {
				putCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				if err := cache.Put(putCtx, orgKey, string(orgJson), exp); err == nil {
					interceptor.CacheClient.BloomAddKey(orgKey)
				} else {
					logs.LG.Warn("预热 MEMBER_ORGANIZATION 缓存失败", zap.String("key", orgKey), zap.Error(err))
					success = false
				}
				cancel()
			} else {
				logs.LG.Warn("预热 MEMBER_ORGANIZATION 序列化失败", zap.Error(oerr))
				success = false
			}
		}
	}

	return success
}

// 将 MyOrgList 和 FindMemInfoById 的响应数据proto序列化写入 Redis
func prewarmGrpcInterceptorCache(memberIds []int64) bool {
	if len(memberIds) == 0 {
		return true
	}
	cache := dao.Rc
	memberDao := dao.NewMemberDao()
	orgDao := dao.NewOrganizationDao()
	const batchSize = 500
	success := true

	for i := 0; i < len(memberIds); i += batchSize {
		end := i + batchSize
		if end > len(memberIds) {
			end = len(memberIds)
		}
		batch := memberIds[i:end]
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		members, err := memberDao.FindMemberByIds(ctx, batch)
		if err != nil {
			logs.LG.Warn("gRPC 缓存预热查询成员失败", zap.Error(err))
			success = false
			cancel()
			continue
		}
		orgMap, err := orgDao.FindOrganizationsByMemIds(ctx, batch)
		if err != nil {
			logs.LG.Warn("gRPC 缓存预热查询组织失败", zap.Error(err))
			success = false
			orgMap = map[int64][]*organization.Organization{}
		}
		cancel()

		memberMap := make(map[int64]*member.Member, len(members))
		for _, m := range members {
			memberMap[m.Id] = m
		}

		for _, memberId := range batch {
			mem, ok := memberMap[memberId]
			if !ok {
				continue
			}

			// 预热FindMemInfoById
			req := &login.UserMessage{MemId: memberId}
			cacheKey, keyOk := interceptor.CacheClient.BuildCacheKey(
				"/login.service.v1.LoginService/FindMemInfoById", req)
			if keyOk {
				memMsg := &login.MemberMessage{}
				_ = copier.Copy(memMsg, mem)
				memMsg.Code, _ = encrypts.EncryptInt64(mem.Id, model.AESKey)
				orgs := orgMap[memberId]
				if len(orgs) > 0 {
					memMsg.OrganizationCode, _ = encrypts.EncryptInt64(orgs[0].Id, model.AESKey)
				}
				if b, merr := proto.Marshal(memMsg); merr == nil {
					putCtx, pCancel := context.WithTimeout(context.Background(), 2*time.Second)
					if perr := cache.Put(putCtx, cacheKey, string(b), 5*time.Minute); perr == nil {
						interceptor.CacheClient.BloomAddKey(cacheKey)
					} else {
						success = false
					}
					pCancel()
				}
			}

			// MyOrgList
			cacheKey2, keyOk2 := interceptor.CacheClient.BuildCacheKey(
				"/login.service.v1.LoginService/MyOrgList", req)
			if keyOk2 {
				orgs := orgMap[memberId]
				var orgsMessage []*login.OrganizationMessage
				_ = copier.Copy(&orgsMessage, orgs)
				for _, org := range orgsMessage {
					org.Code, _ = encrypts.EncryptInt64(org.Id, model.AESKey)
				}
				orgResp := &login.OrgListResponse{OrganizationList: orgsMessage}
				if b, merr := proto.Marshal(orgResp); merr == nil {
					putCtx, pCancel := context.WithTimeout(context.Background(), 2*time.Second)
					if perr := cache.Put(putCtx, cacheKey2, string(b), 5*time.Minute); perr == nil {
						interceptor.CacheClient.BloomAddKey(cacheKey2)
					} else {
						success = false
					}
					pCancel()
				}
			}
		}
	}
	logs.LG.Info("gRPC 拦截器缓存预热完成", zap.Int("memberCount", len(memberIds)))
	return success
}
