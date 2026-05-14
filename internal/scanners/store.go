package scanners

import (
	"encoding/json"
	"strings"
	"time"

	"go-radar/internal/model"

	"gorm.io/gorm"
)

func StoreSnapshot(db *gorm.DB, payload SnapshotPayload) error {
	token, err := getOrCreateToken(db, payload.Chain, payload.Address, payload.Symbol, payload.Name)
	if err != nil {
		return err
	}
	token.LastSeenAt = nowString()
	if err := db.Save(&token).Error; err != nil {
		return err
	}
	rawJSON := jsonString(payload.Raw)
	snapshot := model.TokenSnapshot{
		TokenID:    token.ID,
		Source:     payload.Source,
		Price:      payload.Price,
		MC:         payload.MC,
		Liq:        payload.Liq,
		Volume:     payload.Volume,
		Holders:    payload.Holders,
		SmartMoney: payload.SmartMoney,
		FundingPct: payload.FundingPct,
		OIUSD:      payload.OIUSD,
		OID6H:      payload.OID6H,
		Buys1H:     payload.Buys1H,
		Sells1H:    payload.Sells1H,
		AgeH:       payload.AgeH,
		RawJSON:    rawJSON,
		CreatedAt:  nowString(),
	}
	return db.Create(&snapshot).Error
}

func StoreSignal(db *gorm.DB, payload SignalPayload, bucketMinutes int) (bool, error) {
	_, created, err := StoreSignalEvent(db, payload, bucketMinutes)
	return created, err
}

func StoreSignalEvent(db *gorm.DB, payload SignalPayload, bucketMinutes int) (*model.SignalEvent, bool, error) {
	token, err := getOrCreateToken(db, payload.Chain, payload.Address, payload.Symbol, payload.Name)
	if err != nil {
		return nil, false, err
	}
	if payload.Token != nil {
		token.Symbol = strings.ToUpper(payload.Token.Symbol)
		token.Name = firstNonEmpty(payload.Token.Name, token.Name)
		token.NarrativeTheme = firstNonEmpty(payload.Token.NarrativeTheme, token.NarrativeTheme)
		token.NarrativeTagsJSON = jsonString(payload.Token.NarrativeTags)
		if payload.Token.SocialLinksJSON != "" {
			token.SocialLinksJSON = payload.Token.SocialLinksJSON
		}
		if payload.Token.Description != "" {
			token.Description = payload.Token.Description
		}
	}
	token.LastSeenAt = nowString()
	if err := db.Save(&token).Error; err != nil {
		return nil, false, err
	}

	dedupeKey := payload.DedupeKey
	if dedupeKey == "" {
		dedupeKey = BuildDedupeKey(payload.Source, payload.Chain, payload.Address, payload.SignalType, bucketMinutes)
	}
	var existing model.SignalEvent
	err = db.Where("dedupe_key = ?", dedupeKey).First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, false, err
	}

	signal := model.SignalEvent{
		TokenID:    &token.ID,
		Source:     payload.Source,
		Chain:      strings.ToLower(payload.Chain),
		Address:    NormalizeAddress(payload.Address),
		Symbol:     strings.ToUpper(payload.Symbol),
		SignalType: payload.SignalType,
		Priority:   payload.Priority,
		Score:      payload.Score,
		Reason:     payload.Reason,
		TagsJSON:   jsonString(payload.Tags),
		RawJSON:    jsonString(payload.Raw),
		DedupeKey:  dedupeKey,
		CreatedAt:  nowString(),
	}
	if err := db.Create(&signal).Error; err != nil {
		return nil, false, err
	}
	return &signal, true, nil
}

func DedupeExists(db *gorm.DB, dedupeKey string) (bool, error) {
	var existing model.SignalEvent
	err := db.Where("dedupe_key = ?", dedupeKey).First(&existing).Error
	if err == nil {
		return true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	return false, err
}

func RecentFundingSnapshot(db *gorm.DB, chain string, address string, source string) (*model.TokenSnapshot, error) {
	var snapshot model.TokenSnapshot
	join := "JOIN t_radar_token ON t_radar_market_snapshot.token_id = t_radar_token.id"
	err := db.Joins(join).
		Where("t_radar_token.chain = ? AND t_radar_token.address = ? AND t_radar_market_snapshot.source = ?", strings.ToLower(chain), NormalizeAddress(address), source).
		Order("t_radar_market_snapshot.created_at desc").
		Limit(1).
		First(&snapshot).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func getOrCreateToken(db *gorm.DB, chain string, address string, symbol string, name string) (model.TokenProfile, error) {
	chain = strings.ToLower(chain)
	address = NormalizeAddress(address)
	var token model.TokenProfile
	err := db.Where("chain = ? AND address = ?", chain, address).First(&token).Error
	if err == nil {
		return token, nil
	}
	if err != gorm.ErrRecordNotFound {
		return model.TokenProfile{}, err
	}
	token = model.TokenProfile{
		Chain:             chain,
		Address:           address,
		Symbol:            strings.ToUpper(symbol),
		Name:              name,
		NarrativeTagsJSON: "[]",
		SocialLinksJSON:   "{}",
		FirstSeenAt:       nowString(),
		LastSeenAt:        nowString(),
	}
	return token, db.Create(&token).Error
}

func NormalizeAddress(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

func BuildDedupeKey(source string, chain string, address string, signalType string, bucketMinutes int) string {
	return strings.Join([]string{
		source,
		strings.ToLower(chain),
		NormalizeAddress(address),
		signalType,
		buildTimeBucket(time.Now().UTC(), bucketMinutes),
	}, "|")
}

func buildTimeBucket(at time.Time, bucketMinutes int) string {
	if bucketMinutes <= 0 {
		bucketMinutes = 30
	}
	bucketStart := at.Truncate(time.Minute)
	minuteDelta := bucketStart.Minute() % bucketMinutes
	bucketStart = bucketStart.Add(-time.Duration(minuteDelta) * time.Minute)
	return bucketStart.Format("2006-01-02T15:04")
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func jsonString(value any) string {
	if value == nil {
		return "{}"
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
