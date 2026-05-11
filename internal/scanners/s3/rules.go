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

// BuildSignalTypes 根据热度、OI 变化和资金费率组合生成 S3 信号类型列表。
func BuildSignalTypes(heat float64, d6h float64, frPct float64, minOIDeltaPct float64) []string {
	signalTypes := []string{}
	if heat > 0 {
		signalTypes = append(signalTypes, "heat")
	}
	if heat > 0 && d6h >= minOIDeltaPct {
		signalTypes = append(signalTypes, "heat_plus_oi")
	}
	if heat > 0 && frPct < -0.03 {
		signalTypes = append(signalTypes, "heat_plus_negative_funding")
	}
	if math.Abs(d6h) >= 8 && heat == 0 {
		signalTypes = append(signalTypes, "oi_anomaly")
	}
	return signalTypes
}
