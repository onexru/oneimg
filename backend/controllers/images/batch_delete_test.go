package images

import (
	"testing"
	"time"
)

func TestNormalizeDeleteIDs(t *testing.T) {
	got := normalizeDeleteIDs([]int{3, 1, 0, 3, -2, 1, 2})
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("去重排序后长度应为 %d，实际 %v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 位应为 %d，实际 %v", i, want[i], got)
		}
	}
}

func TestNormalizeDeleteIDsDropsInvalid(t *testing.T) {
	if got := normalizeDeleteIDs([]int{0, -1, -99}); len(got) != 0 {
		t.Fatalf("非法 ID 应被全部丢弃，实际 %v", got)
	}
}

func TestBatchDeleteTimeout(t *testing.T) {
	cases := []struct {
		count int
		want  time.Duration
	}{
		{1, time.Minute},        // 单张也要有最低预算
		{3, time.Minute},        // 3*20s=60s，正好持平下限
		{30, 10 * time.Minute},  // 30*20s=10min
		{200, 10 * time.Minute}, // 超出上限收敛到 10 分钟
	}
	for _, c := range cases {
		if got := batchDeleteTimeout(c.count); got != c.want {
			t.Fatalf("%d 张的预算应为 %s，实际 %s", c.count, c.want, got)
		}
	}
}

func TestMaxBatchDeleteCountCoversOnePage(t *testing.T) {
	// 图库一页 30 张，上限必须能一次装下整页选中，否则前端又得拆请求。
	if maxBatchDeleteCount < 30 {
		t.Fatalf("单次批量删除上限 %d 小于一页 30 张", maxBatchDeleteCount)
	}
}
