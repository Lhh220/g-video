package service

// 集成测试：testcontainers 拉起真实 MySQL/Redis 容器，OSS 用本地假服务器模拟
// (假 OSS 实现了分片上传协议的 XML 应答，CompleteMultipartUpload 全链路可测)。
// 懒初始化：只有真正跑到集成用例才启动容器；本机无 Docker 自动 skip，CI 环境强制失败防假绿灯。

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	goss "github.com/aliyun/aliyun-oss-go-sdk/oss"
	socialpb "github.com/Lhh220/g-video/api/proto/social"
	userpb "github.com/Lhh220/g-video/api/proto/user"
	"github.com/Lhh220/g-video/api/proto/video"
	"github.com/Lhh220/g-video/logic-server/internal/config"
	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/oss"
	pkgredis "github.com/Lhh220/g-video/logic-server/pkg/redis"
	"github.com/Lhh220/g-video/logic-server/pkg/utils"
	goredis "github.com/go-redis/redis/v8"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	itOnce    sync.Once
	itErr     error
	itReady   bool
	itMySQL   *tcmysql.MySQLContainer
	itRedis   *tcredis.RedisContainer
	itFakeOSS *httptest.Server
)

func setupIntegration() error {
	itOnce.Do(func() {
		ctx := context.Background()

		// 1. MySQL 容器 + 建表
		itMySQL, itErr = tcmysql.Run(ctx, "mysql:8.0",
			tcmysql.WithDatabase("gvideo"),
			tcmysql.WithUsername("gvideo"),
			tcmysql.WithPassword("gvideo"),
		)
		if itErr != nil {
			itErr = fmt.Errorf("启动 MySQL 容器失败: %w", itErr)
			return
		}
		// 注意：模块的 ConnectionString 会把附加参数格式化出空格导致 DSN 非法，自己拼
		dsn := itMySQL.MustConnectionString(ctx) + "?parseTime=true&charset=utf8mb4&loc=Local"
		var db *gorm.DB
		db, itErr = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if itErr != nil {
			return
		}
		if itErr = db.AutoMigrate(
			&model.User{}, &model.Video{}, &model.Like{}, &model.Comment{}, &model.AuditLog{}, &model.Follow{},
		); itErr != nil {
			return
		}
		database.DB = db

		// 2. Redis 容器
		itRedis, itErr = tcredis.Run(ctx, "redis:7-alpine")
		if itErr != nil {
			itErr = fmt.Errorf("启动 Redis 容器失败: %w", itErr)
			return
		}
		addr, _ := itRedis.ConnectionString(ctx)
		pkgredis.RDB = goredis.NewClient(&goredis.Options{Addr: strings.TrimPrefix(addr, "redis://")})
		if _, itErr = pkgredis.RDB.Ping(ctx).Result(); itErr != nil {
			return
		}

		// 3. 假 OSS：UseCname 走路径风格，URL 直指本服务
		itFakeOSS = newFakeOSS()
		endpoint := strings.TrimPrefix(itFakeOSS.URL, "http://")
		var client *goss.Client
		client, itErr = goss.New(endpoint, "it-ak", "it-sk", goss.UseCname(true))
		if itErr != nil {
			return
		}
		oss.Bucket, itErr = client.Bucket("it-bucket")
		if itErr != nil {
			return
		}
		config.GlobalConfig.OSS.BucketName = "it-bucket"
		config.GlobalConfig.OSS.Endpoint = endpoint

		itReady = true
	})
	return itErr
}

// newFakeOSS 本地假对象存储：实现分片上传协议 XML + 删除/查询一律成功
func newFakeOSS() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.RawQuery, "bucketInfo"):
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><BucketInfo><Bucket><Name>it-bucket</Name><Location>it</Location></Bucket></BucketInfo>`)
		case r.Method == http.MethodPost && q.Has("uploads"): // 初始化分片会话 (query 为 ?uploads 无值，须用 Has 判断)
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><InitiateMultipartUploadResult><Bucket>it-bucket</Bucket><Key>it</Key><UploadId>it-upload-id</UploadId></InitiateMultipartUploadResult>`)
		case r.Method == http.MethodPut && q.Get("partNumber") != "": // 上传分片
			w.Header().Set("ETag", `"ITETAG"`)
		case r.Method == http.MethodGet && q.Get("uploadId") != "": // 列已传分片
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><ListPartsResult><Bucket>it-bucket</Bucket><Key>it</Key><UploadId>it-upload-id</UploadId><Part><PartNumber>1</PartNumber><ETag>ITETAG</ETag><Size>1</Size></Part></ListPartsResult>`)
		case r.Method == http.MethodPost && q.Get("uploadId") != "": // 合并分片
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><CompleteMultipartUploadResult><Location>it</Location><Bucket>it-bucket</Bucket><Key>it</Key><ETag>ITETAG</ETag></CompleteMultipartUploadResult>`)
		default: // 删除对象等
			w.WriteHeader(http.StatusNoContent)
		}
	}))
}

// requireIntegration 集成用例统一入口
func requireIntegration(t *testing.T) {
	t.Helper()
	if err := setupIntegration(); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("CI 环境集成测试基础设施不可用: %v", err)
		}
		t.Skipf("集成环境不可用，跳过: %v", err)
	}
}

func TestMain(m *testing.M) {
	code := m.Run()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if itMySQL != nil {
		_ = itMySQL.Terminate(ctx)
	}
	if itRedis != nil {
		_ = itRedis.Terminate(ctx)
	}
	if itFakeOSS != nil {
		itFakeOSS.Close()
	}
	os.Exit(code)
}

// ---- 测试数据辅助 ----

var itSeq int64

func itUnique(prefix string) string {
	itSeq++
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano()%1e9, itSeq)
}

func itMD5() string {
	return fmt.Sprintf("%032x", time.Now().UnixNano()+itSeq)[:32]
}

func itCreateUser(t *testing.T, role int32) (int64, string) {
	t.Helper()
	uname := itUnique("it_user")
	hash, err := bcrypt.GenerateFromPassword([]byte("pass123456"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("生成密码哈希失败: %v", err)
	}
	u := model.User{Username: uname, Password: string(hash), Role: role}
	if err := database.DB.Create(&u).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	return u.ID, uname
}

func itToken(t *testing.T, uid int64) string {
	t.Helper()
	token, err := utils.GenerateToken(uid, 0)
	if err != nil {
		t.Fatalf("生成 token 失败: %v", err)
	}
	return token
}

func itCreateVideo(t *testing.T, author int64, status int32, md5 string) int64 {
	t.Helper()
	playURL := fmt.Sprintf("https://it-bucket.%s/videos/%s.mp4", config.GlobalConfig.OSS.Endpoint, md5)
	v := model.Video{
		AuthorID: author,
		Title:    "it-视频",
		PlayURL:  playURL,
		CoverURL: playURL + "?x-oss-process=video/snapshot",
		FileMD5:  md5,
		Status:   status,
	}
	if err := database.DB.Create(&v).Error; err != nil {
		t.Fatalf("创建视频失败: %v", err)
	}
	// 与真实入库路径保持一致：布隆加指纹 + 首屏缓存失效
	afterVideoCreated(&v)
	return int64(v.ID)
}

// itFeedMap 拉一次时间流，返回 videoID -> Video (含展示计数)
func itFeedMap(t *testing.T, token string) map[int64]*video.Video {
	t.Helper()
	s := &VideoService{}
	resp, err := s.Feed(context.Background(), &video.FeedRequest{Token: token})
	if err != nil || resp.StatusCode != 0 {
		t.Fatalf("Feed 调用失败: err=%v resp=%+v", err, resp)
	}
	m := make(map[int64]*video.Video, len(resp.VideoList))
	for _, v := range resp.VideoList {
		m[v.Id] = v
	}
	return m
}

// ---- 用例 1: 审核可见性矩阵 (历史 P0 的回归保护) ----

func TestIntegration_AuditVisibilityMatrix(t *testing.T) {
	requireIntegration(t)
	ctx := context.Background()

	author, _ := itCreateUser(t, 0)
	admin, _ := itCreateUser(t, 1)
	outsider, _ := itCreateUser(t, 0)

	pendingID := itCreateVideo(t, author, 0, itMD5()) // 待审
	passedID := itCreateVideo(t, author, 1, itMD5())  // 已通过
	s := &VideoService{}

	// 待审不上 Feed、通过的在 Feed
	feed := itFeedMap(t, "")
	if _, ok := feed[pendingID]; ok {
		t.Error("待审视频不应出现在 Feed")
	}
	if _, ok := feed[passedID]; !ok {
		t.Error("已通过视频应出现在 Feed")
	}

	// 非管理员拉不到待审列表
	pls, _ := s.ListPendingVideos(ctx, &video.PendingListRequest{AdminId: outsider, Page: 1, PageSize: 50})
	if pls.StatusCode == 0 {
		t.Error("非管理员不应能拉取待审列表")
	}

	// 待审列表包含待审视频
	pls, _ = s.ListPendingVideos(ctx, &video.PendingListRequest{AdminId: admin, Page: 1, PageSize: 50})
	found := false
	for _, v := range pls.VideoList {
		if v.Id == pendingID {
			found = true
		}
	}
	if !found {
		t.Error("待审列表应包含待审视频")
	}

	// 审核通过 → Feed 可见
	resp, _ := s.AuditVideo(ctx, &video.AuditRequest{AdminId: admin, VideoId: pendingID, Action: 1})
	if resp.StatusCode != 0 {
		t.Fatalf("审核通过失败: %s", resp.StatusMsg)
	}
	if _, ok := itFeedMap(t, "")[pendingID]; !ok {
		t.Error("审核通过后视频应出现在 Feed")
	}

	// 驳回 → 数据库物理删除(含点赞/评论)，假OSS同步删除
	database.DB.Create(&model.Like{UserID: author, VideoID: passedID})
	database.DB.Create(&model.Comment{UserID: author, VideoID: passedID, Content: "it"})
	resp, _ = s.AuditVideo(ctx, &video.AuditRequest{AdminId: admin, VideoId: passedID, Action: 2, Reason: "it-驳回"})
	if resp.StatusCode != 0 {
		t.Fatalf("驳回失败: %s", resp.StatusMsg)
	}
	var cnt int64
	database.DB.Model(&model.Video{}).Where("id = ?", passedID).Count(&cnt)
	if cnt != 0 {
		t.Error("驳回后视频记录应被物理删除")
	}
	database.DB.Model(&model.Like{}).Where("video_id = ?", passedID).Count(&cnt)
	if cnt != 0 {
		t.Error("驳回后点赞应被物理删除")
	}
	database.DB.Model(&model.Comment{}).Where("video_id = ?", passedID).Count(&cnt)
	if cnt != 0 {
		t.Error("驳回后评论应被物理删除")
	}
	if _, ok := itFeedMap(t, "")[passedID]; ok {
		t.Error("驳回后视频不应出现在 Feed")
	}
	database.DB.Model(&model.AuditLog{}).Where("video_id = ?", passedID).Count(&cnt)
	if cnt == 0 {
		t.Error("审核日志应留档")
	}
}

// ---- 用例 2: 注册角色安全 ----

func TestIntegration_RegisterAlwaysRegularUser(t *testing.T) {
	requireIntegration(t)
	uname := itUnique("it_reg")
	s := &UserService{}
	resp, err := s.Register(context.Background(), registerReq(uname))
	if err != nil || resp.StatusCode != 0 {
		t.Fatalf("注册失败: err=%v resp=%+v", err, resp)
	}
	var u model.User
	database.DB.Where("username = ?", uname).First(&u)
	if u.Role != 0 {
		t.Errorf("注册用户 role 必须是普通用户(0)，实际 %d", u.Role)
	}
	// 登录返回的 role 也应为 0
	lr, _ := s.Login(context.Background(), loginReq(uname))
	if lr.StatusCode != 0 || lr.Role != 0 {
		t.Errorf("登录应成功且 role=0，实际 resp=%+v", lr)
	}
}

// ---- 用例 3: 秒传 / 分片会话状态机 ----

func TestIntegration_UploadSessionAndInstantUpload(t *testing.T) {
	requireIntegration(t)
	ctx := context.Background()
	uid, _ := itCreateUser(t, 0)
	token := itToken(t, uid)
	s := &VideoService{}

	// 秒传：库中已有相同指纹
	srcMD5 := itMD5()
	srcID := itCreateVideo(t, uid, 1, srcMD5)
	var src model.Video
	database.DB.First(&src, srcID)

	init, _ := s.InitUpload(ctx, &video.InitUploadRequest{Token: token, Filename: "a.mp4", FileSize: 100, FileMd5: srcMD5})
	if !init.Uploaded {
		t.Fatalf("相同指纹应秒传命中: %+v", init)
	}
	comp, _ := s.CompleteUpload(ctx, &video.CompleteUploadRequest{Token: token, FileMd5: srcMD5, Title: "秒传"})
	if comp.StatusCode != 0 {
		t.Fatalf("秒传完成失败: %s", comp.StatusMsg)
	}
	var dup model.Video
	database.DB.First(&dup, comp.VideoId)
	if dup.PlayURL != src.PlayURL {
		t.Error("秒传应复用源视频的云端地址")
	}

	// 全新会话
	newMD5 := itMD5()
	init2, _ := s.InitUpload(ctx, &video.InitUploadRequest{Token: token, Filename: "b.mp4", FileSize: 100, FileMd5: newMD5})
	if init2.Uploaded || init2.UploadId == "" {
		t.Fatalf("全新文件应返回新会话: %+v", init2)
	}

	// 会话属主校验：他人 token 传分片应被拒
	otherID, _ := itCreateUser(t, 0)
	otherToken := itToken(t, otherID)
	part, _ := s.UploadPart(ctx, &video.UploadPartRequest{Token: otherToken, UploadId: init2.UploadId, PartNumber: 1, Data: []byte("x")})
	if part.StatusCode == 0 {
		t.Error("他人会话上传分片应被拒绝")
	}

	// 本人分片成功 (假OSS应答ETag)
	part, _ = s.UploadPart(ctx, &video.UploadPartRequest{Token: token, UploadId: init2.UploadId, PartNumber: 1, Data: []byte("x")})
	if part.StatusCode != 0 {
		t.Errorf("本人分片上传应成功: %s", part.StatusMsg)
	}

	// 断点续传：带 prev_upload_id 再 init，假OSS ListParts 返回分片1已传
	init3, _ := s.InitUpload(ctx, &video.InitUploadRequest{Token: token, Filename: "b.mp4", FileSize: 100, FileMd5: newMD5, PrevUploadId: init2.UploadId})
	if len(init3.ExistedParts) == 0 {
		t.Error("断点续传应返回已传分片号")
	}

	// 完成合并 (假OSS CompleteMultipartUpload 协议)
	comp2, _ := s.CompleteUpload(ctx, &video.CompleteUploadRequest{Token: token, UploadId: init2.UploadId, FileMd5: newMD5, Title: "分片"})
	if comp2.StatusCode != 0 {
		t.Fatalf("分片合并完成失败: %s", comp2.StatusMsg)
	}
	var merged model.Video
	database.DB.First(&merged, comp2.VideoId)
	if merged.FileMD5 != newMD5 || merged.Status != 0 {
		t.Errorf("合并后记录应有指纹且待审: %+v", merged)
	}
}

// ---- 用例 4: 点赞计数 Redis 写合并 + 落库正确性 ----

func TestIntegration_FavoriteCounterFlush(t *testing.T) {
	requireIntegration(t)
	ctx := context.Background()

	author, _ := itCreateUser(t, 0)
	u1, _ := itCreateUser(t, 0)
	u2, _ := itCreateUser(t, 0)
	u3, _ := itCreateUser(t, 0)
	vID := itCreateVideo(t, author, 1, itMD5())

	s := &SocialService{}
	for _, uid := range []int64{u1, u2, u3} {
		resp, _ := s.FavoriteAction(ctx, favoriteReq(uid, vID, 1))
		if resp.StatusCode != 0 {
			t.Fatalf("点赞失败: %s", resp.StatusMsg)
		}
	}
	// u2 取消
	resp, _ := s.FavoriteAction(ctx, favoriteReq(u2, vID, 2))
	if resp.StatusCode != 0 {
		t.Fatalf("取消点赞失败: %s", resp.StatusMsg)
	}

	// 等待异步 Redis 同步协程完成
	time.Sleep(300 * time.Millisecond)

	// flush 前数据库基数仍为 0，但 Feed 展示值 = 0 + 增量(3-1=2)
	var v model.Video
	database.DB.First(&v, vID)
	if v.FavoriteCount != 0 {
		t.Errorf("flush 前数据库计数应为0，实际 %d", v.FavoriteCount)
	}
	feed := itFeedMap(t, itToken(t, u1))
	fv, ok := feed[vID]
	if !ok {
		t.Fatal("Feed 应包含刚创建的视频")
	}
	if fv.FavoriteCount != 2 {
		t.Errorf("Feed 展示计数应为2(DB0+增量2)，实际 %d", fv.FavoriteCount)
	}
	if !fv.IsFavorite {
		t.Error("u1 的点赞状态应显示已赞")
	}

	// 手动触发落库 → 数据库计数精确为 2
	flushFavoriteDeltas(context.Background())
	database.DB.First(&v, vID)
	if v.FavoriteCount != 2 {
		t.Errorf("flush 后数据库计数应为2，实际 %d", v.FavoriteCount)
	}

	// dirty 集合已清空
	dirty, _ := redisSMembersDirty()
	if len(dirty) != 0 {
		t.Errorf("flush 后 dirty 集合应为空，实际 %v", dirty)
	}
}

// ---- 用例 5: 热门池预计算与排序 ----

func TestIntegration_HotPoolRefreshAndOrder(t *testing.T) {
	requireIntegration(t)
	ctx := context.Background()

	author, _ := itCreateUser(t, 0)
	oldID := itCreateVideo(t, author, 1, itMD5())
	newID := itCreateVideo(t, author, 1, itMD5())
	// 老视频: 3天前发布但 10万赞 (碾压级，免疫前序用例残留数据的分数干扰)；新视频: 刚发布 0 互动
	database.DB.Model(&model.Video{}).Where("id = ?", oldID).Updates(map[string]interface{}{
		"favorite_count": 100000,
		"created_at":     time.Now().Add(-72 * time.Hour),
	})

	refreshHotPool(ctx)

	n, _ := pkgredis.RDB.ZCard(ctx, hotZSetKey).Result()
	if n == 0 {
		t.Fatal("热门池不应为空")
	}
	// 老视频高互动应排在新视频前
	scoreOld, _ := pkgredis.RDB.ZScore(ctx, hotZSetKey, itoa(oldID)).Result()
	scoreNew, _ := pkgredis.RDB.ZScore(ctx, hotZSetKey, itoa(newID)).Result()
	if scoreOld <= scoreNew {
		t.Errorf("高互动老视频(%.2f)应排在零互动新视频(%.2f)前", scoreOld, scoreNew)
	}

	// hot 模式首个返回即老视频
	s := &VideoService{}
	resp, _ := s.Feed(ctx, &video.FeedRequest{Mode: "hot"})
	if resp.StatusCode != 0 || len(resp.VideoList) == 0 {
		t.Fatalf("hot 模式应返回列表: %+v", resp)
	}
	if resp.VideoList[0].Id != oldID {
		t.Errorf("hot 模式首个应为高互动老视频 %d，实际 %d", oldID, resp.VideoList[0].Id)
	}
}

// ---- 用例 6: 分布式锁 (多实例后台任务的互斥基础) ----

func TestIntegration_DistributedLock(t *testing.T) {
	requireIntegration(t)
	ctx := context.Background()
	key := "lock:it:" + itUnique("t")

	ok1, tok1, err := pkgredis.TryLock(ctx, key, 5*time.Second)
	if err != nil || !ok1 {
		t.Fatalf("首次抢锁应成功: ok=%v err=%v", ok1, err)
	}
	// 持锁期间第二个竞争者(模拟另一实例)不应获得
	ok2, _, err := pkgredis.TryLock(ctx, key, 5*time.Second)
	if err != nil {
		t.Fatalf("二次抢锁调用出错: %v", err)
	}
	if ok2 {
		t.Error("持锁期间其他实例不应获得锁")
	}
	// 释放后可重新获得
	pkgredis.ReleaseLock(ctx, key, tok1)
	ok3, tok3, err := pkgredis.TryLock(ctx, key, 5*time.Second)
	if err != nil || !ok3 {
		t.Fatalf("释放后应可重新抢锁: ok=%v err=%v", ok3, err)
	}
	pkgredis.ReleaseLock(ctx, key, tok3)
}

// ---- 测试内的小工具 ----

func registerReq(username string) *userpb.RegisterRequest {
	return &userpb.RegisterRequest{Username: username, Password: "pass123456"}
}

func loginReq(username string) *userpb.LoginRequest {
	return &userpb.LoginRequest{Username: username, Password: "pass123456"}
}

func favoriteReq(uid, vid int64, action int32) *socialpb.FavoriteRequest {
	return &socialpb.FavoriteRequest{UserId: uid, VideoId: vid, ActionType: action}
}

func itoa(n int64) string { return fmt.Sprintf("%d", n) }

func redisSMembersDirty() ([]string, error) {
	return pkgredis.RDB.SMembers(context.Background(), favDirtyKey).Result()
}
