package service

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/redis"
	goredis "github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"
)

// 缓存三件套：singleflight 防击穿 / 首屏短TTL缓存 / 布隆过滤器防穿透

// ---- 1. latest Feed 首屏缓存 (3s TTL + singleflight 重建) ----

const latestFirstKey = "feed:latest:first"

var latestFeedSF singleflight.Group

// getLatestFirstPage 首屏(无游标)的最近30条：短TTL缓存扛住刷首页的热点读，
// 击穿时 singleflight 合并并发回源，只有一个请求打到 MySQL
func getLatestFirstPage(ctx context.Context) []model.Video {
	if val, err := redis.RDB.Get(ctx, latestFirstKey).Result(); err == nil {
		var vs []model.Video
		if json.Unmarshal([]byte(val), &vs) == nil && len(vs) > 0 {
			return vs
		}
	}

	v, err, _ := latestFeedSF.Do(latestFirstKey, func() (interface{}, error) {
		var vs []model.Video
		if err := database.DB.Where("status = ?", 1).
			Order("created_at desc").Limit(30).Find(&vs).Error; err != nil {
			return nil, err
		}
		if data, err := json.Marshal(vs); err == nil {
			redis.RDB.Set(context.Background(), latestFirstKey, data, 3*time.Second)
		}
		return vs, nil
	})
	if err != nil {
		return nil
	}
	vs, _ := v.([]model.Video)
	return vs
}

// invalidateLatestFirstPage 内容变更时主动失效 (短TTL兜底，这里只是更快)
func invalidateLatestFirstPage() {
	redis.RDB.Del(context.Background(), latestFirstKey)
}

// afterVideoCreated 新视频入库后的缓存维护：布隆加指纹 + 首屏缓存失效
func afterVideoCreated(v *model.Video) {
	if v.FileMD5 != "" {
		bloomAdd(context.Background(), v.FileMD5)
	}
	invalidateLatestFirstPage()
}

// ---- 2. 用户信息缓存击穿防护 ----

var userSF singleflight.Group

// loadUserSF 同一用户的热 key 失效瞬间，N 个并发请求只放一个回源 MySQL
func loadUserSF(userID int64) (*model.User, error) {
	v, err, _ := userSF.Do(userCacheKey(userID), func() (interface{}, error) {
		var u model.User
		if err := database.DB.First(&u, userID).Error; err != nil {
			return nil, err
		}
		return &u, nil
	})
	if err != nil {
		return nil, err
	}
	u, _ := v.(*model.User)
	return u, nil
}

func userCacheKey(userID int64) string {
	return "sf:user:" + strconv.FormatInt(userID, 10)
}

// ---- 3. 布隆过滤器：秒传 MD5 前置判断 (防无效指纹穿透 DB) ----

const (
	bloomKey     = "bloom:filemd5"
	bloomBits    = 1 << 21 // 2M bits ≈ 256KB；10万条目时误报率约 0.8%
	bloomHashes  = 7
)

// bloomIndexes 双重散列 (Kirsch-Mitzenmacher)：h_i = h1 + i*h2 mod m
func bloomIndexes(md5 string) []int64 {
	h1 := fnv.New64a()
	h1.Write([]byte(md5))
	a := h1.Sum64()
	h2 := fnv.New64()
	h2.Write([]byte(md5))
	b := h2.Sum64()

	idx := make([]int64, bloomHashes)
	for i := int64(0); i < bloomHashes; i++ {
		idx[i] = int64((a + uint64(i)*b) % uint64(bloomBits))
	}
	return idx
}

// bloomAdd 写入指纹 (可批量)
func bloomAdd(ctx context.Context, md5s ...string) {
	if len(md5s) == 0 {
		return
	}
	pipe := redis.RDB.Pipeline()
	for _, m := range md5s {
		for _, i := range bloomIndexes(m) {
			pipe.SetBit(ctx, bloomKey, i, 1)
		}
	}
	_, _ = pipe.Exec(ctx)
}

// bloomMayExist 任一位为 0 → 肯定不存在(无漏报)；全 1 → 可能存在(有误报，DB 兜底确认)
func bloomMayExist(ctx context.Context, md5 string) bool {
	pipe := redis.RDB.Pipeline()
	cmds := make([]*goredis.IntCmd, 0, bloomHashes)
	for _, i := range bloomIndexes(md5) {
		cmds = append(cmds, pipe.GetBit(ctx, bloomKey, i))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return true // Redis 异常时放行去查 DB (正确性优先)
	}
	for _, c := range cmds {
		if v, _ := c.Result(); v == 0 {
			return false
		}
	}
	return true
}

// RebuildBloomFilter 启动时从库重建位图 (进程重启位图不丢失：位图本身存 Redis，重建用于补漏)
func RebuildBloomFilter(ctx context.Context) {
	var rows []model.Video
	if err := database.DB.Where("file_md5 <> ''").Find(&rows).Error; err != nil {
		return
	}
	md5s := make([]string, 0, len(rows))
	for _, r := range rows {
		md5s = append(md5s, r.FileMD5)
	}
	bloomAdd(ctx, md5s...)
}
