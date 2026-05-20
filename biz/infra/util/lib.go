package util

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"runtime"
	"sort"
)

func Convert[T any](in any) (out T, ok bool) {
	if v, ok := in.(T); ok {
		return v, true
	}
	return
}

// GzipCompress gzip压缩
func GzipCompress(data []byte) ([]byte, error) {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, _ = w.Write(data)
	_ = w.Close()
	return b.Bytes(), nil
}

// GzipDecompress gzip解压
func GzipDecompress(src []byte) ([]byte, error) {
	// 1. 空数据检查
	if len(src) == 0 {
		return nil, nil
	}

	// 2. 创建GZIP读取器
	r, err := gzip.NewReader(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("创建解压器失败: %w", err)
	}
	defer func() { _ = r.Close() }()

	// 3. 读取解压数据
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, fmt.Errorf("解压数据读取失败: %w", err)
	}

	// 4. 返回解压结果
	return buf.Bytes(), nil
}

// I2BigEndBytes 将整数变成字节数组
func I2BigEndBytes(n int) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(n))
	return b
}

// BuildBytes 将传入的byte拼接并返回一个新的bytes数组
func BuildBytes(data ...[]byte) []byte {
	var b bytes.Buffer
	for _, d := range data {
		b.Write(d)
	}
	return b.Bytes()
}

func CallerInfo(skip int) string {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	fn := runtime.FuncForPC(pc)
	return fmt.Sprintf("%s:%d %s", file, line, fn.Name())
}

// CalculateChange 计算统计数据变化率（保留2位小数）
func CalculateChange(current, lastWeek float64) float64 {
	if lastWeek == 0 {
		if current == 0 {
			return 0
		}
		return 100.00 // 上周为0，本周有数据，增长100%
	}
	return Round2(((current - lastWeek) / lastWeek) * 100)
}

// KeywordsMap2Slice 转换得到关键词列表
func KeywordsMap2Slice[V any](m map[string]V) []string {
	s := make([]string, 0, len(m))
	for k, _ := range m {
		s = append(s, k)
	}
	return s
}

// Wow 周环比辅助，cur 为本期值，prev 为上期值
type Wow struct{ Cur, Prev int32 }

func (w Wow) Inc() int32    { return w.Cur - w.Prev }
func (w Wow) Rate() float64 { return Rate(w.Cur, w.Prev) }

// Rate 计算增长率 (cur - prev) / prev
func Rate(cur, prev int32) float64 {
	if prev > 0 {
		return Round2(float64(cur-prev) / float64(prev))
	}
	return 0
}

// Round2 四舍五入到小数点后两位
func Round2(f float64) float64 { return math.Round(f*100) / 100 }

// Int32Ptr 返回 int32 指针
func Int32Ptr(v int32) *int32 { return &v }

// Float64Ptr 返回 float64 指针
func Float64Ptr(v float64) *float64 { return &v }

// RiskDistributionCnt2Ratio 将各年级风险用户数转为百分比（凑整，最后一项用 100-sum 保证总和为 100）
func RiskDistributionCnt2Ratio(cntMap map[int32]int32, total int32) map[int32]int32 {
	if total <= 0 || len(cntMap) == 0 {
		return make(map[int32]int32, len(cntMap))
	}

	// 按 key 排序保证最后一项确定
	keys := make([]int32, 0, len(cntMap))
	for k := range cntMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	ratio := make(map[int32]int32, len(cntMap))
	var sum int32
	last := keys[len(keys)-1]
	for _, k := range keys {
		if k == last {
			ratio[k] = 100 - sum
		} else {
			r := (cntMap[k] * 100) / total
			ratio[k] = r
			sum += r
		}
	}
	return ratio
}
