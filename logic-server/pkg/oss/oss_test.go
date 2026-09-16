package oss

import (
	"testing"

	"github.com/Lhh220/g-video/logic-server/internal/config"
)

func setupTestConfig() {
	config.GlobalConfig.OSS.BucketName = "test-bucket"
	config.GlobalConfig.OSS.Endpoint = "oss-cn-test.aliyuncs.com"
}

func TestURLToObjectKey(t *testing.T) {
	setupTestConfig()

	cases := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "普通对象",
			url:  "https://test-bucket.oss-cn-test.aliyuncs.com/videos/abc.mp4",
			want: "videos/abc.mp4",
		},
		{
			name: "带查询参数 (封面截帧)",
			url:  "https://test-bucket.oss-cn-test.aliyuncs.com/videos/abc.mp4?x-oss-process=video/snapshot,t_1000",
			want: "videos/abc.mp4",
		},
		{
			name: "嵌套路径 (HLS切片)",
			url:  "https://test-bucket.oss-cn-test.aliyuncs.com/hls/abc/index.m3u8",
			want: "hls/abc/index.m3u8",
		},
		{
			name: "非本 Bucket 的 URL",
			url:  "https://other-bucket.oss-cn-test.aliyuncs.com/videos/abc.mp4",
			want: "",
		},
		{
			name: "完全无关的 URL",
			url:  "https://example.com/some/file.mp4",
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := URLToObjectKey(c.url); got != c.want {
				t.Errorf("URLToObjectKey(%q) = %q, 期望 %q", c.url, got, c.want)
			}
		})
	}
}

func TestDeletePrefixByHLSURLKeyDerivation(t *testing.T) {
	setupTestConfig()

	// 无法直接断言云端行为，这里至少验证错误分支：非本 bucket 的 URL 必须报错
	if err := DeletePrefixByHLSURL("https://evil.com/hls/x/index.m3u8"); err == nil {
		t.Error("非本 Bucket 的 URL 应当报错")
	}
}
