package service

import (
	"context"
	"fmt"
	"testing"
)

// 布隆纯函数：无漏报 (加过的必报"可能存在") + 索引分布
func TestBloomIndexesNoCollisionExplosion(t *testing.T) {
	seen := map[int64]int{}
	for i := 0; i < 1000; i++ {
		for _, idx := range bloomIndexes(fmt.Sprintf("md5-%06d", i)) {
			if idx < 0 || idx >= bloomBits {
				t.Fatalf("索引越界: %d", idx)
			}
			seen[idx]++
		}
	}
	// 7000 个索引落点应足够分散 (唯一落点 > 5000)
	if len(seen) <= 5000 {
		t.Errorf("索引分布过于集中: 唯一落点 %d", len(seen))
	}
}

// 集成：真实 Redis 位图 — 加过的指纹必命中(无漏报)，随机未加指纹误报率 < 5%
func TestIntegration_BloomFilter(t *testing.T) {
	requireIntegration(t)
	ctx := context.Background()

	added := make([]string, 0, 2000)
	for i := 0; i < 2000; i++ {
		m := fmt.Sprintf("%032x", i)
		added = append(added, m)
	}
	bloomAdd(ctx, added...)

	for _, m := range added {
		if !bloomMayExist(ctx, m) {
			t.Fatalf("布隆出现漏报(不可接受): %s", m)
		}
	}

	// 未加入的指纹：误报率应显著低于 5%
	fp := 0
	total := 10000
	for i := 100000; i < 100000+total; i++ {
		if bloomMayExist(ctx, fmt.Sprintf("%032x", i)) {
			fp++
		}
	}
	if rate := float64(fp) / float64(total); rate > 0.05 {
		t.Errorf("误报率 %.2f%% 超过阈值 5%%", rate*100)
	} else {
		t.Logf("误报率 %.3f%% (2000条目)", rate*100)
	}
}
