package s2

import "math"

// ComputeOISegments 将 OI 序列切成四段均值，用于判断整体持仓是否上升。
func ComputeOISegments(oiValues []float64) ([]float64, float64, bool) {
	if len(oiValues) < 12 {
		return nil, 0, false
	}
	segLen := len(oiValues) / 4
	if segLen < 3 {
		return nil, 0, false
	}
	segments := []float64{
		average(oiValues[:segLen]),
		average(oiValues[segLen : segLen*2]),
		average(oiValues[segLen*2 : segLen*3]),
		average(oiValues[segLen*3:]),
	}
	first := segments[0]
	oiChange := 0.0
	if first > 0 {
		oiChange = (segments[3] - first) / first * 100
	}
	return segments, oiChange, oiChange > 0
}

// IsFundingFlip 判断资金费率是否从非负切换为负值。
func IsFundingFlip(previous *float64, current *float64) bool {
	if previous == nil || current == nil {
		return false
	}
	return *previous >= 0 && *current < 0
}

// ScoreFundingSignal 为 S2 资金费率翻负且 OI 上升的信号评分。
func ScoreFundingSignal(currentFRPct float64, oiChangePct float64, hasSpot bool, volumeUSD float64) float64 {
	score := 55 + math.Min(math.Abs(currentFRPct)*800, 20) + math.Min(math.Max(oiChangePct, 0), 40)
	if hasSpot {
		score += 5
	}
	if volumeUSD >= 50_000_000 {
		score += 10
	} else if volumeUSD >= 10_000_000 {
		score += 5
	}
	return math.Round(score*100) / 100
}

// average 计算均值，空数组返回 0。
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}
