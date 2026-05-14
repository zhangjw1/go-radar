package s3

import "math"

// DetectVolumeSurge 判断当前 24 小时成交额是否高于历史均值指定倍数。
func DetectVolumeSurge(vol24h float64, previousVolumes []float64, multiplier float64) (bool, float64) {
	if len(previousVolumes) == 0 {
		return false, 0
	}
	total := 0.0
	for _, value := range previousVolumes {
		total += value
	}
	avgPrevious := total / float64(len(previousVolumes))
	if avgPrevious <= 0 {
		return false, 0
	}
	ratio := vol24h / avgPrevious
	return ratio >= multiplier, ratio
}

// BuildSignalTypes 根据热度、OI 变化和资金费率生成 S3 信号类型。
//
// 同一个币同一轮只返回一个最高组合态，避免“热度”“热度+OI”“热度+负费率”
// 在页面和推送里拆成多条重复信号。
func BuildSignalTypes(heat float64, d6h float64, frPct float64, hasFunding bool, minOIDeltaPct float64) []string {
	if heat > 0 && d6h >= minOIDeltaPct && hasFunding && frPct < -0.03 {
		return []string{"heat_plus_oi_negative_funding"}
	}
	if heat > 0 && d6h >= minOIDeltaPct {
		return []string{"heat_plus_oi"}
	}
	if heat > 0 && hasFunding && frPct < -0.03 {
		return []string{"heat_plus_negative_funding"}
	}
	if heat > 0 {
		return []string{"heat"}
	}
	if math.Abs(d6h) >= 8 {
		return []string{"oi_anomaly"}
	}
	return nil
}
