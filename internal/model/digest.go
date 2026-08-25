package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// ParamDigest 计算参数摘要：以维度与张量形状指纹作为输入，得到稳定摘要串。
// 该函数不依赖具体参数值（服务只校验形状/维度一致性，不存储权重本身），
// 而是用“维度 + 形状描述”生成可比较的指纹。
func ParamDigest(dimension int, shape []int) string {
	h := sha256.New()
	h.Write([]byte(strconv.Itoa(dimension)))
	h.Write([]byte("|"))
	for i, s := range shape {
		if i > 0 {
			h.Write([]byte(","))
		}
		h.Write([]byte(strconv.Itoa(s)))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ParseShape 从逗号分隔字符串解析形状数组。空串返回空切片。
func ParseShape(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return []int{}, nil
	}
	parts := strings.Split(s, ",")
	shape := make([]int, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("invalid shape element %q: %w", p, err)
		}
		if v <= 0 {
			return nil, fmt.Errorf("invalid shape element %q: must be positive", p)
		}
		shape = append(shape, v)
	}
	return shape, nil
}

// ShapeString 将形状数组格式化为逗号分隔串。
func ShapeString(shape []int) string {
	parts := make([]string, 0, len(shape))
	for _, s := range shape {
		parts = append(parts, strconv.Itoa(s))
	}
	return strings.Join(parts, ",")
}
