package repository

import (
	"testing"
	"time"
)

// TestJitteredTTLRange 纯单元测试：验证 CacheUser 的 TTL 随机化函数
//   - TTL 必须落在 [base-jitter, base+jitter] 范围内（24h ± 1h → 23h~25h）
//   - 多次采样必须出现不同取值（非固定值，防缓存雪崩）
func TestJitteredTTLRange(t *testing.T) {
	base := UserCacheBaseTTL  // 24h
	jitter := UserCacheJitter // 1h
	const samples = 2000

	distinct := make(map[time.Duration]bool)
	for i := 0; i < samples; i++ {
		ttl := jitteredTTL(base, jitter)
		if ttl < base-jitter {
			t.Fatalf("TTL 低于下限: got=%v want>=%v", ttl, base-jitter)
		}
		if ttl > base+jitter {
			t.Fatalf("TTL 超过上限: got=%v want<=%v", ttl, base+jitter)
		}
		distinct[ttl] = true
	}

	t.Logf("采样 %d 次，TTL 允许范围 [%v, %v]，实际不同取值 %d 个",
		samples, base-jitter, base+jitter, len(distinct))
	if len(distinct) < 2 {
		t.Fatalf("TTL 无随机偏移，疑似固定值，不同取值数=%d", len(distinct))
	}
}
