package service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/redis"
	"gorm.io/gorm"
)

// 点赞计数的高并发优化：
// 写路径只动 Redis (INCR 增量)，由后台协程每 5 秒批量合并进 MySQL (写合并/削峰)；
// 读路径展示值 = DB 基数 + Redis 未落库增量 (保证读己之写)。
// 注意：GETDEL 需要 Redis 6.2+ (docker-compose 的 redis:alpine 已满足)

const (
	favDirtyKey      = "video:fav:dirty"             // 待落库的视频 ID 集合
	favDeltaKeyFmt   = "video:fav:delta:%d"          // 每个视频的未落库增量
	favFlushInterval = 5 * time.Second
)

// incrFavoriteDelta 计数变更走 Redis。
// 顺序必须是先 INCR delta 再 SADD dirty：
// flusher 先 SMEMBERS+DEL dirty、后 GETDEL delta，此顺序下任何交错都不会丢计数。
func incrFavoriteDelta(ctx context.Context, videoID int64, delta int64) {
	redis.RDB.IncrBy(ctx, fmt.Sprintf(favDeltaKeyFmt, videoID), delta)
	redis.RDB.SAdd(ctx, favDirtyKey, videoID)
}

// getFavoriteDeltas 批量读取未落库增量，供 Feed 合并展示
func getFavoriteDeltas(ctx context.Context, videoIDs []int64) map[int64]int64 {
	result := make(map[int64]int64)
	if len(videoIDs) == 0 {
		return result
	}

	keys := make([]string, 0, len(videoIDs))
	for _, id := range videoIDs {
		keys = append(keys, fmt.Sprintf(favDeltaKeyFmt, id))
	}

	vals, err := redis.RDB.MGet(ctx, keys...).Result()
	if err != nil {
		return result
	}
	for i, val := range vals {
		if s, ok := val.(string); ok {
			if d, err := strconv.ParseInt(s, 10, 64); err == nil && d != 0 {
				result[videoIDs[i]] = d
			}
		}
	}
	return result
}

// RunFavoriteCounterFlusher 后台协程：定时把 Redis 增量合并进 MySQL
func RunFavoriteCounterFlusher(ctx context.Context) {
	ticker := time.NewTicker(favFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 点赞计数落库协程已退出")
			return
		case <-ticker.C:
			flushFavoriteDeltas(ctx)
		}
	}
}

func flushFavoriteDeltas(ctx context.Context) {
	// 1. 取走 dirty 集合 (之后新来的计数会进入下一轮)
	ids, err := redis.RDB.SMembers(ctx, favDirtyKey).Result()
	if err != nil || len(ids) == 0 {
		return
	}
	redis.RDB.Del(ctx, favDirtyKey)

	// 2. 逐个取走增量并落库；GREATEST 防止负数
	flushed := 0
	for _, idStr := range ids {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}

		val, err := redis.RDB.GetDel(ctx, fmt.Sprintf(favDeltaKeyFmt, id)).Result()
		if err != nil {
			continue // key 不存在 = 增量已被上一轮落库
		}
		delta, err := strconv.ParseInt(val, 10, 64)
		if err != nil || delta == 0 {
			continue
		}

		if err := database.DB.Model(&model.Video{}).Where("id = ?", id).
			UpdateColumn("favorite_count", gorm.Expr("GREATEST(favorite_count + ?, 0)", delta)).Error; err != nil {
			log.Printf("⚠️ [Counter Flush] 视频 %d 计数落库失败: %v", id, err)
			// 落库失败把增量放回去，等下一轮重试
			incrFavoriteDelta(ctx, id, delta)
			continue
		}
		flushed++
	}

	if flushed > 0 {
		log.Printf("✅ [Counter Flush] 已合并 %d 个视频的点赞增量到 MySQL", flushed)
	}
}
