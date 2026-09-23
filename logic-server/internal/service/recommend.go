package service

import (
	"context"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/Lhh220/g-video/api/proto/user"
	"github.com/Lhh220/g-video/api/proto/video"
	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/logx"
	"github.com/Lhh220/g-video/logic-server/pkg/redis"
	"github.com/Lhh220/g-video/logic-server/pkg/utils"
	goredis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// 推荐流 V1：三路召回(热门/最新/关注) → 分数归并(关注加权) → 同作者打散 + 已看过过滤
// 热门池由后台协程每分钟预计算进 Redis ZSet，读路径零实时打分

const (
	hotZSetKey         = "video:hot:zset"
	hotScanWindowDays  = 30    // 参与打分的视频年龄窗口
	hotScanMaxVideos   = 2000  // 单次打分最多扫描条数
	hotRefreshInterval = time.Minute
	feedBatchSize      = 30  // 单次返回条数
	recallHotCount     = 40  // mix 模式热门召回条数
	recallLatestCount  = 30  // mix 模式最新召回条数
	recallFollowCount  = 20  // mix 模式关注召回条数
	followBoost        = 1.5 // 关注作者的视频加权倍数
	viewedKeyFmt       = "user:viewed:%d"
	viewedTTL          = 7 * 24 * time.Hour
)

// hotScore 热度分：互动加权和 / 时间衰减因子 (Hacker News 式)。
// +1 保证零互动的新视频也有初始分；+2 小时避免除零与过度惩罚刚发布的内容。
func hotScore(likes, comments int64, createdAt time.Time) float64 {
	ageHours := time.Since(createdAt).Hours() + 2.0
	return float64(2*likes+3*comments+1) / math.Pow(ageHours, 1.5)
}

// RunHotPoolRefresher 周期性把审核通过的视频按热度分写入 Redis ZSet
func RunHotPoolRefresher(ctx context.Context) {
	refreshHotPool(ctx)

	ticker := time.NewTicker(hotRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logx.L().Info("热门池刷新协程已退出")
			return
		case <-ticker.C:
			refreshHotPool(ctx)
		}
	}
}

// refreshHotPool 写临时 ZSet 后 RENAME 原子替换：读侧任何时刻都能看到完整热门池
func refreshHotPool(ctx context.Context) {
	var videos []model.Video
	if err := database.DB.
		Where("status = ? AND created_at > ?", 1, time.Now().AddDate(0, 0, -hotScanWindowDays)).
		Order("created_at desc").
		Limit(hotScanMaxVideos).
		Find(&videos).Error; err != nil {
		logx.L().Warn("热门池拉取视频失败", zap.Error(err))
		return
	}

	tmpKey := hotZSetKey + ":new"
	pipe := redis.RDB.Pipeline()
	pipe.Del(ctx, tmpKey)
	for _, v := range videos {
		pipe.ZAdd(ctx, tmpKey, &goredis.Z{
			Score:  hotScore(v.FavoriteCount, v.CommentCount, v.CreatedAt),
			Member: strconv.FormatInt(int64(v.ID), 10),
		})
	}
	pipe.Expire(ctx, tmpKey, 10*time.Minute) // 刷新协程挂掉后热门池自动过期，避免无限陈旧
	pipe.Rename(ctx, tmpKey, hotZSetKey)

	if _, err := pipe.Exec(ctx); err != nil {
		logx.L().Warn("热门池刷新失败", zap.Error(err))
		return
	}
	logx.L().Info("🔥 热门池已刷新", zap.Int("videos", len(videos)))
}

// ---- 已看过 (曝光过滤) ----

func filterViewed(ctx context.Context, userID int64, ids []int64) []int64 {
	if userID == 0 || len(ids) == 0 {
		return ids
	}
	flags, err := redis.RDB.SMIsMember(ctx, viewedKey(userID), convertIDs(ids)...).Result()
	if err != nil {
		return ids // Redis 异常时不过滤，保可用性
	}
	kept := make([]int64, 0, len(ids))
	for i, id := range ids {
		if !flags[i] {
			kept = append(kept, id)
		}
	}
	return kept
}

func recordViewed(userID int64, ids []int64) {
	if userID == 0 || len(ids) == 0 {
		return
	}
	go func() {
		ctx := context.Background()
		key := viewedKey(userID)
		redis.RDB.SAdd(ctx, key, convertIDs(ids))
		redis.RDB.Expire(ctx, key, viewedTTL)
	}()
}

func viewedKey(userID int64) string {
	return "user:viewed:" + strconv.FormatInt(userID, 10)
}

func convertIDs(ids []int64) []interface{} {
	out := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		out = append(out, strconv.FormatInt(id, 10))
	}
	return out
}

// ---- 打散：同作者视频不相邻 (贪心交换，保持分数序基本不变) ----
// 边界：某作者占比超过一半时鸽巢原理下必然仍有相邻，算法只保证相邻对数最少化
func spreadSameAuthors[T any](items []T, authorID func(T) int64) {
	for i := 1; i < len(items); i++ {
		if authorID(items[i]) != authorID(items[i-1]) {
			continue
		}
		// 与后面第一个不同作者的位置交换
		for j := i + 1; j < len(items); j++ {
			if authorID(items[j]) != authorID(items[i]) {
				items[i], items[j] = items[j], items[i]
				break
			}
		}
	}
}

// hotFeed 热门流：直接读预计算 ZSet，多取一倍用于已看过过滤后仍满额
func (s *VideoService) hotFeed(ctx context.Context, req *video.FeedRequest) (*video.FeedResponse, error) {
	currentUserID := currentUserFromToken(req.Token)

	members, err := redis.RDB.ZRevRange(ctx, hotZSetKey, 0, feedBatchSize*2-1).Result()
	if err != nil || len(members) == 0 {
		// 热门池未就绪/Redis 异常 → 降级为时间流，保证接口可用
		return s.Feed(ctx, &video.FeedRequest{LatestTime: req.LatestTime, Token: req.Token})
	}

	ids := make([]int64, 0, len(members))
	for _, m := range members {
		if id, err := strconv.ParseInt(m, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	ids = filterViewed(ctx, currentUserID, ids)
	if len(ids) > feedBatchSize {
		ids = ids[:feedBatchSize]
	}

	ordered := loadVideosOrdered(ctx, ids)

	followingMap := loadFollowingMap(ctx, currentUserID)
	list := assembleRecommendList(ctx, ordered, currentUserID, followingMap)
	recordViewed(currentUserID, ids)

	return &video.FeedResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		VideoList:  list,
		NextTime:   time.Now().UnixMilli(),
	}, nil
}

// feedCandidate mix 归并阶段的候选
type feedCandidate struct {
	v     model.Video
	score float64
}

// mixFeed 推荐流：热门/最新/关注三路召回 → 归并(关注×1.5) → 打散 → 过滤已看过
func (s *VideoService) mixFeed(ctx context.Context, req *video.FeedRequest) (*video.FeedResponse, error) {
	currentUserID := currentUserFromToken(req.Token)
	now := time.Now()

	// scoreMap: videoID -> 当前最高召回分
	scoreMap := make(map[int64]float64)

	// 召回1: 热门池 (分数已预计算)
	if hotMembers, err := redis.RDB.ZRevRangeWithScores(ctx, hotZSetKey, 0, recallHotCount-1).Result(); err == nil {
		for _, z := range hotMembers {
			if id, err := strconv.ParseInt(z.Member.(string), 10, 64); err == nil {
				scoreMap[id] = z.Score
			}
		}
	}

	// 召回2: 最新池
	var latest []model.Video
	database.DB.Where("status = ?", 1).
		Order("created_at desc").Limit(recallLatestCount).Find(&latest)
	for _, v := range latest {
		id := int64(v.ID)
		if s := hotScore(v.FavoriteCount, v.CommentCount, v.CreatedAt); s > scoreMap[id] {
			scoreMap[id] = s
		}
	}

	// 召回3: 关注作者最新 (游客无此路)
	if currentUserID != 0 {
		var followed []model.Video
		database.DB.Table("videos").
			Joins("JOIN follows ON follows.to_user_id = videos.author_id").
			Where("follows.user_id = ? AND videos.status = ?", currentUserID, 1).
			Order("videos.created_at desc").Limit(recallFollowCount).Find(&followed)
		for _, v := range followed {
			id := int64(v.ID)
			s := hotScore(v.FavoriteCount, v.CommentCount, v.CreatedAt) * followBoost
			if s > scoreMap[id] {
				scoreMap[id] = s
			} else {
				scoreMap[id] *= followBoost // 已在其他池命中，补关注加权
			}
		}
	}

	if len(scoreMap) == 0 {
		return s.Feed(ctx, &video.FeedRequest{LatestTime: req.LatestTime, Token: req.Token})
	}

	// 过滤已看过 → 一次 IN 查全量 → 排序
	allIDs := make([]int64, 0, len(scoreMap))
	for id := range scoreMap {
		allIDs = append(allIDs, id)
	}
	allIDs = filterViewed(ctx, currentUserID, allIDs)

	var videos []model.Video
	if err := database.DB.Where("id IN ? AND status = ?", allIDs, 1).Find(&videos).Error; err != nil {
		return &video.FeedResponse{StatusCode: 1, StatusMsg: "查询失败"}, nil
	}

	candidates := make([]feedCandidate, 0, len(videos))
	for _, v := range videos {
		candidates = append(candidates, feedCandidate{v: v, score: scoreMap[int64(v.ID)]})
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })

	// 同作者打散后截断
	spreadSameAuthors(candidates, func(c feedCandidate) int64 { return c.v.AuthorID })
	if len(candidates) > feedBatchSize {
		candidates = candidates[:feedBatchSize]
	}

	ordered := make([]model.Video, 0, len(candidates))
	for _, c := range candidates {
		ordered = append(ordered, c.v)
	}

	followingMap := loadFollowingMap(ctx, currentUserID)
	list := assembleRecommendList(ctx, ordered, currentUserID, followingMap)

	viewedIDs := make([]int64, 0, len(ordered))
	for _, v := range ordered {
		viewedIDs = append(viewedIDs, int64(v.ID))
	}
	recordViewed(currentUserID, viewedIDs)

	return &video.FeedResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		VideoList:  list,
		NextTime:   now.UnixMilli(),
	}, nil
}

// ---- 组装辅助 (hot/mix 共用，循环内零 SQL) ----

func currentUserFromToken(token string) int64 {
	if token == "" {
		return 0
	}
	if claims, err := utils.ParseToken(token); err == nil {
		return claims.UserID
	}
	return 0
}

func loadFollowingMap(ctx context.Context, userID int64) map[int64]bool {
	m := make(map[int64]bool)
	if userID == 0 {
		return m
	}
	ids, err := redis.RDB.SMembers(ctx, "user:following:"+strconv.FormatInt(userID, 10)).Result()
	if err != nil {
		return m
	}
	for _, idStr := range ids {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			m[id] = true
		}
	}
	return m
}

// loadVideosOrdered 按 id 顺序批量加载 (过滤掉已下架/删除的)
func loadVideosOrdered(ctx context.Context, ids []int64) []model.Video {
	if len(ids) == 0 {
		return nil
	}
	var videos []model.Video
	if err := database.DB.Where("id IN ? AND status = ?", ids, 1).Find(&videos).Error; err != nil {
		return nil
	}
	byID := make(map[int64]model.Video, len(videos))
	for _, v := range videos {
		byID[int64(v.ID)] = v
	}
	ordered := make([]model.Video, 0, len(ids))
	for _, id := range ids {
		if v, ok := byID[id]; ok {
			ordered = append(ordered, v)
		}
	}
	return ordered
}

// assembleRecommendList 批量组装：作者 IN 查询、点赞状态 IN 查询、计数增量合并
func assembleRecommendList(ctx context.Context, videos []model.Video, currentUserID int64, followingMap map[int64]bool) []*video.Video {
	if len(videos) == 0 {
		return nil
	}

	videoIDs := make([]int64, 0, len(videos))
	authorSet := make(map[int64]struct{})
	for _, v := range videos {
		videoIDs = append(videoIDs, int64(v.ID))
		authorSet[v.AuthorID] = struct{}{}
	}

	// 作者批量
	authorIDs := make([]int64, 0, len(authorSet))
	for id := range authorSet {
		authorIDs = append(authorIDs, id)
	}
	var authors []model.User
	database.DB.Where("id IN ?", authorIDs).Find(&authors)
	authorMap := make(map[int64]*model.User, len(authors))
	for i := range authors {
		authorMap[authors[i].ID] = &authors[i]
	}

	// 点赞状态批量
	favoriteMap := make(map[int64]bool, len(videos))
	if currentUserID != 0 {
		var likes []model.Like
		database.DB.Where("user_id = ? AND video_id IN ?", currentUserID, videoIDs).Find(&likes)
		for _, l := range likes {
			favoriteMap[int64(l.VideoID)] = true
		}
	}

	// 未落库的计数增量
	favoriteDeltaMap := getFavoriteDeltas(ctx, videoIDs)

	list := make([]*video.Video, 0, len(videos))
	for _, v := range videos {
		author := authorMap[v.AuthorID]
		if author == nil {
			author = &model.User{ID: v.AuthorID, Username: "未知用户"}
		}
		list = append(list, &video.Video{
			Id:            int64(v.ID),
			PlayUrl:       v.PlayURL,
			CoverUrl:      v.CoverURL,
			HlsUrl:        v.HLSURL,
			Title:         v.Title,
			FavoriteCount: v.FavoriteCount + favoriteDeltaMap[int64(v.ID)],
			CommentCount:  v.CommentCount,
			IsFavorite:    favoriteMap[int64(v.ID)],
			Author: &user.User{
				Id:            int64(author.ID),
				Username:      author.Username,
				Avatar:        author.Avatar,
				IsFollow:      followingMap[v.AuthorID],
			},
		})
	}
	return list
}
