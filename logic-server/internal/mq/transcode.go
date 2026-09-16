package mq

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/oss"
)

// runHLSTranscode 把原视频切片为 HLS (m3u8 + ts) 并上传 OSS，成功后回填 hls_url
// 前置条件：宿主机/容器装有 ffmpeg；没有则跳过 (转码是增强能力，不是硬依赖)
func runHLSTranscode(videoID int64, playURL string) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Printf("ℹ️ [HLS] 未安装 ffmpeg，跳过转码 (视频仍可用 mp4 直链播放)")
		return
	}

	objectKey := oss.URLToObjectKey(playURL)
	if objectKey == "" {
		log.Printf("⚠️ [HLS] 无法解析对象 Key: %s", playURL)
		return
	}

	// 已转码过则跳过 (消息重复投递时保持幂等)
	var v model.Video
	if err := database.DB.First(&v, videoID).Error; err != nil {
		log.Printf("⚠️ [HLS] 视频不存在: %d", videoID)
		return
	}
	if v.HLSURL != "" {
		log.Printf("ℹ️ [HLS] 视频 %d 已有切片，跳过", videoID)
		return
	}

	// 1. 下载原视频到临时目录
	tmpDir, err := os.MkdirTemp("", "gvideo-hls-*")
	if err != nil {
		log.Printf("⚠️ [HLS] 创建临时目录失败: %v", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "src"+filepath.Ext(objectKey))
	if err := oss.DownloadToFile(objectKey, srcPath); err != nil {
		log.Printf("⚠️ [HLS] 下载原视频失败: %v", err)
		return
	}

	// 2. ffmpeg 切片：-codec copy 只做容器封装转换不重编码，速度最快
	outDir := filepath.Join(tmpDir, "hls")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Printf("⚠️ [HLS] 创建输出目录失败: %v", err)
		return
	}
	cmd := exec.Command(ffmpegPath,
		"-i", srcPath,
		"-codec", "copy",
		"-hls_time", "10",               // 每片约 10 秒
		"-hls_playlist_type", "vod",      // 点播模式，列表写死所有片
		"-hls_segment_filename", filepath.Join(outDir, "seg_%03d.ts"),
		filepath.Join(outDir, "index.m3u8"),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("⚠️ [HLS] ffmpeg 切片失败: %v\n%s", err, string(out))
		return
	}

	// 3. 整目录上传：hls/{去后缀的文件名}/index.m3u8 + seg_*.ts
	// (对象 Key 由 MD5 决定，同一视频的切片天然落在同一前缀下)
	base := strings.TrimSuffix(filepath.Base(objectKey), filepath.Ext(objectKey))
	hlsURL, err := oss.UploadDir(outDir, "hls/"+base+"/")
	if err != nil {
		log.Printf("⚠️ [HLS] 上传切片失败: %v", err)
		return
	}

	// 4. 回填播放地址，前端检测到 hls_url 就走切片播放
	if err := database.DB.Model(&model.Video{}).
		Where("id = ?", videoID).
		Update("hls_url", hlsURL).Error; err != nil {
		log.Printf("⚠️ [HLS] 回填 hls_url 失败: %v", err)
		return
	}
	log.Printf("✅ [HLS] 视频 %d 切片完成: %s", videoID, hlsURL)
}
