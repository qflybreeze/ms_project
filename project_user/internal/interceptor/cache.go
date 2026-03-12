package interceptor

import (
	"context"
	"encoding/json"
	"go_project/ms_project/project_common/encrypts"
	"go_project/ms_project/project_grpc/user/login"
	"go_project/ms_project/project_user/internal/dao"
	"go_project/ms_project/project_user/internal/repo"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
	"golang.org/x/sync/singleflight"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

const (
	emptyCacheValue  = "__EMPTY__"     // 空值标记，防止缓存穿透
	emptyCacheTTL    = 1 * time.Minute // 空值缓存时间
	defaultCacheTTL  = 5 * time.Minute // 默认缓存时间
	refreshThreshold = 1 * time.Minute // 异步刷新阈值
)

type CacheInterceptor struct {
	cache        repo.Cache
	cacheMap     map[string]reflect.Type
	bloomEnabled map[string]bool
	bloomBlock   map[string]bool
	bloomFilter  *bloom.BloomFilter // 布隆过滤器
	bloomMu      sync.RWMutex       // 布隆过滤器读写锁
	bloomReady   atomic.Bool
	sf           singleflight.Group
	refreshing   sync.Map // 正在异步刷新的 key 集合
}

func New() *CacheInterceptor {
	cacheMap := make(map[string]reflect.Type)
	cacheMap["/login.service.v1.LoginService/MyOrgList"] = reflect.TypeOf(&login.OrgListResponse{})
	cacheMap["/login.service.v1.LoginService/FindMemInfoById"] = reflect.TypeOf(&login.MemberMessage{})
	cacheMap["/login.service.v1.LoginService/TokenVerify"] = reflect.TypeOf(&login.LoginResponse{})

	bf := bloom.NewWithEstimates(1000000, 0.01)

	return &CacheInterceptor{
		cache:    dao.Rc,
		cacheMap: cacheMap,
		bloomEnabled: map[string]bool{
			"/login.service.v1.LoginService/MyOrgList":       true,
			"/login.service.v1.LoginService/FindMemInfoById": true,
			"/login.service.v1.LoginService/TokenVerify":     true,
		},
		bloomBlock: map[string]bool{
			"/login.service.v1.LoginService/MyOrgList":       true,
			"/login.service.v1.LoginService/FindMemInfoById": true,
			"/login.service.v1.LoginService/TokenVerify":     false,
		},
		bloomFilter: bf,
	}
}

var CacheClient = New()

func (c *CacheInterceptor) CacheInterceptor() func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		respType, ok := c.cacheMap[info.FullMethod]
		if !ok || respType == nil {
			return handler(ctx, req)
		}

		con, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		cacheKey, ok := c.BuildCacheKey(info.FullMethod, req)
		if !ok {
			zap.L().Warn("cache key build failed, bypass cache", zap.String("method", info.FullMethod))
			return handler(ctx, req)
		}

		// 布隆过滤器检查
		if c.bloomEnabled[info.FullMethod] && c.BloomReady() {
			inBloom := c.bloomContains(cacheKey)
			if !inBloom {
				if c.bloomBlock[info.FullMethod] {
					zap.L().Warn("布隆过滤器拦截，key不存在", zap.String("method", info.FullMethod), zap.String("key", cacheKey))
					return reflect.New(respType.Elem()).Interface(), nil
				}
				zap.L().Info("布隆过滤器未命中，继续访问后端", zap.String("method", info.FullMethod))
			}
		}

		// 查询缓存
		respJson, getErr := c.cache.Get(con, cacheKey)
		if getErr == nil && respJson != "" {
			// 检查是否为空值缓存
			if respJson == emptyCacheValue {
				zap.L().Info("命中空值缓存", zap.String("method", info.FullMethod))
				return reflect.New(respType.Elem()).Interface(), nil
			}

			newInst := reflect.New(respType.Elem()).Interface()
			deserialized := false
			if pm, ok := newInst.(proto.Message); ok {
				if perr := proto.Unmarshal([]byte(respJson), pm); perr == nil {
					deserialized = true
				}
			} else {
				if jerr := json.Unmarshal([]byte(respJson), newInst); jerr == nil {
					deserialized = true
				}
			}
			if deserialized {
				// 缓存命中，检查是否需要异步刷新
				c.tryAsyncRefresh(cacheKey, info.FullMethod, ctx, req, handler)
				return newInst, nil
			}
		}

		// 缓存未命中
		zap.L().Info("缓存未命中，查询数据库", zap.String("method", info.FullMethod))

		v, err, shared := c.sf.Do(cacheKey, func() (interface{}, error) {
			// 双重检查：防止多个请求同时进入 singleflight
			if respJson, getErr := c.cache.Get(con, cacheKey); getErr == nil && respJson != "" {
				if respJson == emptyCacheValue {
					zap.L().Info("双重检查命中空值缓存", zap.String("method", info.FullMethod))
					return reflect.New(respType.Elem()).Interface(), nil
				}
				newInst := reflect.New(respType.Elem()).Interface()
				if pm, ok := newInst.(proto.Message); ok {
					if perr := proto.Unmarshal([]byte(respJson), pm); perr == nil {
						zap.L().Info("双重检查命中缓存", zap.String("method", info.FullMethod))
						return newInst, nil
					}
				} else {
					if jerr := json.Unmarshal([]byte(respJson), newInst); jerr == nil {
						zap.L().Info("双重检查命中缓存", zap.String("method", info.FullMethod))
						return newInst, nil
					}
				}
			}

			resp, err := handler(ctx, req)
			if err != nil {
				return nil, err
			}

			// 写入缓存
			if c.isEmptyResponse(resp) {
				zap.L().Info("存储空值缓存", zap.String("method", info.FullMethod))
				putCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = c.cache.Put(putCtx, cacheKey, emptyCacheValue, emptyCacheTTL)
			} else {
				c.cacheResponse(con, cacheKey, resp, info.FullMethod)
			}

			return resp, nil
		})

		if shared {
			zap.L().Info("singleflight 合并请求", zap.String("key", cacheKey))
		}

		return v, err
	}
}

// 向布隆过滤器添加key
func (c *CacheInterceptor) BloomAddKeys(keys []string) {
	c.bloomMu.Lock()
	defer c.bloomMu.Unlock()
	for _, key := range keys {
		c.bloomFilter.AddString(key)
	}
	zap.L().Info("布隆过滤器预热完成", zap.Int("count", len(keys)))
}

func (c *CacheInterceptor) BloomAddKey(key string) {
	c.bloomAdd(key)
}

func (c *CacheInterceptor) BloomContains(key string) bool {
	return c.bloomContains(key)
}

func (c *CacheInterceptor) SetBloomReady(ready bool) {
	c.bloomReady.Store(ready)
}

func (c *CacheInterceptor) BloomReady() bool {
	return c.bloomReady.Load()
}

func (c *CacheInterceptor) bloomAdd(key string) {
	c.bloomMu.Lock()
	defer c.bloomMu.Unlock()
	c.bloomFilter.AddString(key)
}

// 检查key是否可能存在
func (c *CacheInterceptor) bloomContains(key string) bool {
	c.bloomMu.RLock()
	defer c.bloomMu.RUnlock()
	return c.bloomFilter.TestString(key)
}

// 缓存响应数据
func (c *CacheInterceptor) cacheResponse(ctx context.Context, key string, resp interface{}, method string) {
	var valBytes []byte
	if pm, ok := resp.(proto.Message); ok {
		if b, merr := proto.Marshal(pm); merr == nil {
			valBytes = b
		}
	} else {
		if b, merr := json.Marshal(resp); merr == nil {
			valBytes = b
		}
	}
	putCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if len(valBytes) > 0 {
		if perr := c.cache.Put(putCtx, key, string(valBytes), defaultCacheTTL); perr != nil {
			zap.L().Warn("缓存写入失败", zap.String("method", method), zap.Error(perr))
		} else {
			zap.L().Info("写入缓存成功", zap.String("method", method))
			if c.bloomEnabled[method] {
				c.bloomAdd(key)
			}
		}
	}
}

// 判断响应是否为空
func (c *CacheInterceptor) isEmptyResponse(resp interface{}) bool {
	if resp == nil {
		return true
	}

	v := reflect.ValueOf(resp)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// 检查 OrgListResponse
	if orgList, ok := resp.(*login.OrgListResponse); ok {
		return orgList == nil || len(orgList.OrganizationList) == 0
	}

	// 检查 MemberMessage
	if member, ok := resp.(*login.MemberMessage); ok {
		return member == nil || member.Id == 0
	}

	if loginResp, ok := resp.(*login.LoginResponse); ok {
		return loginResp == nil || loginResp.Member == nil || loginResp.Member.Id == 0
	}

	return false
}

// 检查缓存 TTL，若剩余时间低于阈值则异步回源刷新
func (c *CacheInterceptor) tryAsyncRefresh(cacheKey, method string, ctx context.Context, req interface{}, handler grpc.UnaryHandler) {
	// 检查 TTL
	ttlCtx, ttlCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer ttlCancel()
	ttl, err := c.cache.TTL(ttlCtx, cacheKey)
	if err != nil || ttl <= 0 {
		return
	}
	if ttl > refreshThreshold {
		return
	}

	if _, loaded := c.refreshing.LoadOrStore(cacheKey, true); loaded { // 已经有协程在刷新这个 key
		return
	}

	go func() {
		defer c.refreshing.Delete(cacheKey)

		refreshCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		resp, err := handler(refreshCtx, req)
		if err != nil {
			zap.L().Warn("异步刷新缓存回源失败", zap.String("method", method), zap.Error(err))
			return
		}
		if c.isEmptyResponse(resp) {
			return // 空结果不覆盖现有缓存
		}
		c.cacheResponse(refreshCtx, cacheKey, resp, method)
		zap.L().Info("异步刷新缓存完成", zap.String("method", method), zap.String("key", cacheKey))
	}()
}

// BuildCacheKey 构建缓存 key，失败返回 false
func (c *CacheInterceptor) BuildCacheKey(method string, req interface{}) (string, bool) {
	var keyData []byte
	if pm, ok := req.(proto.Message); ok {
		if b, e := proto.Marshal(pm); e == nil {
			keyData = b
		} else {
			zap.L().Warn("proto marshal request failed", zap.Error(e))
			if jb, je := json.Marshal(req); je == nil {
				keyData = jb
			}
		}
	} else {
		if jb, je := json.Marshal(req); je == nil {
			keyData = jb
		}
	}
	if len(keyData) == 0 {
		return "", false
	}
	return method + "::" + encrypts.Md5(string(keyData)), true
}
