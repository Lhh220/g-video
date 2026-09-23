package service

import (
	"math"
	"testing"
	"time"
)

func TestHotScore(t *testing.T) {
	now := time.Now()
	old := now.Add(-24 * time.Hour)
	fresh := now.Add(-30 * time.Minute)

	// 同互动量：越新分越高
	if s1, s2 := hotScore(10, 5, fresh), hotScore(10, 5, old); s1 <= s2 {
		t.Errorf("同样互动量，新视频(%f)应高于旧视频(%f)", s1, s2)
	}

	// 同年龄：互动越多分越高，且评论权重(3)高于点赞(2)
	if s := hotScore(0, 10, old); s <= hotScore(10, 0, old) {
		t.Errorf("10条评论(%.2f)应高于10个赞(%.2f)", s, hotScore(10, 0, old))
	}

	// 零互动新视频有保底分，且严格为正
	if s := hotScore(0, 0, now); s <= 0 {
		t.Errorf("零互动新视频应有正的保底分, got %f", s)
	}

	// 零互动刚发布: 分值 = 1/2^1.5
	if s, want := hotScore(0, 0, now), 1/math.Pow(2, 1.5); s < want*0.99 {
		t.Errorf("刚发布视频分值应≈%.4f, got %.4f", want, s)
	}
}

type spreadItem struct {
	id     int
	author int64
}

func TestSpreadSameAuthors(t *testing.T) {
	// 全是同一作者：无可交换，保持原序不 panic
	all := []spreadItem{{1, 7}, {2, 7}, {3, 7}}
	spreadSameAuthors(all, func(x spreadItem) int64 { return x.author })
	if all[0].id != 1 || all[2].id != 3 {
		t.Errorf("全同作者时应保持原序")
	}

	// 两作者交替输入：打散后不应有相邻同作者
	// (作者7×3 + 作者9×2 + 作者10×1 = 3个7需2个隔断，恰好可解；若某作者占比过半则鸽巢原理下必然相邻，算法尽力减少)
	mixed := []spreadItem{
		{1, 7}, {2, 7}, {3, 7}, // 作者7三连
		{4, 9}, {5, 9},           // 作者9二连
		{6, 10},
	}
	spreadSameAuthors(mixed, func(x spreadItem) int64 { return x.author })
	for i := 1; i < len(mixed); i++ {
		if mixed[i].author == mixed[i-1].author {
			t.Errorf("位置 %d,%d 仍是同作者 %d", i-1, i, mixed[i].author)
		}
	}

	// 打散不应丢失元素
	seen := map[int]bool{}
	for _, x := range mixed {
		seen[x.id] = true
	}
	for _, id := range []int{1, 2, 3, 4, 5, 6} {
		if !seen[id] {
			t.Errorf("打散后丢失元素 %d", id)
		}
	}
}
