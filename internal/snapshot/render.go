package snapshot

import (
	"fmt"
	"strings"

	"task240-fedlineage/internal/aggregate"
)

// Render 将可聚合集合渲染为人类可读的谱系摘要文本。
func Render(set *aggregate.AggregableSet) string {
	var b strings.Builder
	fmt.Fprintf(&b, "轮次 %s 可聚合集合（期望维度 %d）\n", set.RoundID, set.ExpectedDim)
	fmt.Fprintf(&b, "纳入更新 %d 个：%s\n", set.UpdateCount, strings.Join(set.UpdateIDs, ", "))
	if len(set.Excluded) > 0 {
		b.WriteString("排除更新：\n")
		for _, e := range set.Excluded {
			fmt.Fprintf(&b, "  - %s [%s] %s\n", e.UpdateID, e.State, e.Reason)
		}
	} else {
		b.WriteString("无排除更新。\n")
	}
	return b.String()
}
