package recallstats

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// Heat model (plan §10, issue #24):
//
//	heat = heat·e^(−Δt/τ) + w      τ = 30d
//
// Only qualifying *use* adds weight: cat w=1, meta w=0.3. Bare search
// impressions NEVER update heat — exposure must not self-reinforce ranking
// (the rich-get-richer trap behind the aliyun-skill pollution in the audit).
// A search is only counted as "converted" (in the query log) when a cat of a
// uri that appeared in that search's top-k follows within SearchCatWindow
// (10 min) — stored on the query row so doctor metrics and later calibration
// (P1.3) can join it.
const (
	HeatTau         = 30 * 24 * time.Hour // decay time constant
	WeightCat       = 1.0
	WeightMeta      = 0.3
	SearchCatWindow = 10 * time.Minute
	DoctorWindow    = 14 * 24 * time.Hour // telemetry window for doctor metrics
	HeatAlarmTopN   = 10
	HeatAlarmShare  = 0.5 // >50% of cats on top-10 hottest = feedback loop
)

// DecayHeat applies the exponential-decay update rule. lastUseMS == 0 means
// never used: no decay, just +w.
func DecayHeat(heat float64, lastUseMS, nowMS int64, tau time.Duration, w float64) float64 {
	if lastUseMS <= 0 {
		return heat + w
	}
	dtMS := nowMS - lastUseMS
	if dtMS < 0 {
		dtMS = 0
	}
	return heat*math.Exp(-float64(dtMS)/float64(tau.Milliseconds())) + w
}

// SetClock overrides the service clock (unit tests). nil restores time.Now.
func (s *Service) SetClock(now func() time.Time) {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	s.mu.Lock()
	s.now = now
	s.mu.Unlock()
}

func (s *Service) clock() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.now().UTC()
}

// recordHeatAndCounters performs the UPSERT with heat decay for a cat or meta
// use event, then attempts the search→cat join for qualifying searches.
func (s *Service) recordHeatAndCounters(ctx context.Context, target, kind string, w float64) error {
	nowMS := s.clock().UnixMilli()

	var heat float64
	var lastUse sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT heat, last_use_at FROM recall_stats WHERE uri = ?`, target,
	).Scan(&heat, &lastUse)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("load heat for %q: %w", target, err)
	}
	var lastMS int64
	if lastUse.Valid {
		lastMS = lastUse.Int64
	}
	newHeat := DecayHeat(heat, lastMS, nowMS, HeatTau, w)

	lastCol := "last_cated_at"
	countCol := "cat_count"
	if kind == "meta" {
		lastCol = "last_metaed_at"
		countCol = "meta_count"
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO recall_stats (uri, search_count, cat_count, meta_count, `+lastCol+`, heat, last_use_at, updated_at)
		VALUES (?, 0, `+countVal(kind)+`, `+countVal(other(kind))+`, ?, ?, ?, ?)
		ON CONFLICT(uri) DO UPDATE SET
			`+countCol+` = `+countCol+` + 1,
			`+lastCol+` = excluded.`+lastCol+`,
			heat = excluded.heat,
			last_use_at = excluded.last_use_at,
			updated_at = excluded.updated_at`,
		target, nowMS, newHeat, nowMS, nowMS,
	); err != nil {
		return fmt.Errorf("record %s for %q: %w", kind, target, err)
	}
	return nil
}

func countVal(kind string) string {
	if kind == "cat" {
		return "1"
	}
	return "0"
}

func other(kind string) string {
	if kind == "cat" {
		return "meta"
	}
	return "cat"
}

// joinSearchToCat marks the most recent search (within SearchCatWindow) whose
// top-k contained the cated uri as converted. This is the only way a search
// ever influences usage telemetry.
func (s *Service) joinSearchToCat(ctx context.Context, target string, nowMS int64) {
	since := nowMS - SearchCatWindow.Milliseconds()
	if _, err := s.db.ExecContext(ctx, `
		UPDATE search_queries
		SET catted_uri = ?, catted_at = ?
		WHERE id = (
			SELECT id FROM search_queries
			WHERE catted_uri IS NULL
			  AND ts >= ? AND ts <= ?
			  AND top_uris LIKE ?
			ORDER BY ts DESC LIMIT 1
		)`,
		target, nowMS, since, nowMS, `%"`+target+`"%`,
	); err != nil {
		// Telemetry only: never fail a cat because the join failed.
		_ = err
	}
}

// RecordQuery logs a search invocation with its ranked top-k so usage can be
// joined to exposure later (LOCAL ONLY — see README privacy note).
func (s *Service) RecordQuery(ctx context.Context, query string, scopes []string, k int, uris []string) error {
	if query = strings.TrimSpace(query); query == "" {
		return nil
	}
	top, err := json.Marshal(uris)
	if err != nil {
		return err
	}
	scope := strings.Join(scopes, ",")
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO search_queries (query, scope, k, top_uris, ts)
		VALUES (?, ?, ?, ?, ?)`,
		query, scope, k, string(top), s.clock().UnixMilli(),
	); err != nil {
		return fmt.Errorf("record query: %w", err)
	}
	return nil
}

func (s *Service) RecordQueryAsync(query string, scopes []string, k int, uris []string) {
	go func() {
		_ = s.RecordQuery(context.Background(), query, scopes, k, uris)
	}()
}

// DoctorMetrics reports retrieval-health signals (issue #24):
//   - zero-cat search rate: searches with no cat of a top-k uri within
//     SearchCatWindow / all searches in the window
//   - heat concentration: share of all cat events landing on the top-N
//     hottest memories; > HeatAlarmShare means ranking feedback loop
type DoctorMetrics struct {
	WindowDays        int     `json:"window_days"`
	Searches          int64   `json:"searches"`
	ConvertedSearch   int64   `json:"converted_searches"`
	ZeroCatRate       float64 `json:"zero_cat_search_rate"`
	TotalCats         int64   `json:"total_cats"`
	TopCats           int64   `json:"top_heats_cats"`
	HeatConcentration float64 `json:"heat_concentration"`
	HeatAlarm         bool    `json:"heat_concentration_alarm"`
}

func (s *Service) DoctorMetrics(ctx context.Context) (DoctorMetrics, error) {
	var m DoctorMetrics
	m.WindowDays = int(DoctorWindow.Hours() / 24)
	since := s.clock().Add(-DoctorWindow).UnixMilli()

	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN catted_uri IS NOT NULL THEN 1 ELSE 0 END), 0)
		FROM search_queries WHERE ts >= ?`, since,
	).Scan(&m.Searches, &m.ConvertedSearch)
	if err != nil {
		return m, fmt.Errorf("doctor: searches: %w", err)
	}
	if m.Searches > 0 {
		m.ZeroCatRate = 1 - float64(m.ConvertedSearch)/float64(m.Searches)
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(cat_count), 0) FROM recall_stats`,
	).Scan(&m.TotalCats); err != nil {
		return m, fmt.Errorf("doctor: total cats: %w", err)
	}
	if m.TotalCats > 0 {
		if err := s.db.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(cat_count), 0) FROM (
				SELECT cat_count FROM recall_stats
				WHERE heat > 0
				ORDER BY heat DESC LIMIT ?
			)`, HeatAlarmTopN,
		).Scan(&m.TopCats); err != nil {
			return m, fmt.Errorf("doctor: top heats: %w", err)
		}
		m.HeatConcentration = float64(m.TopCats) / float64(m.TotalCats)
		m.HeatAlarm = m.HeatConcentration > HeatAlarmShare
	}
	return m, nil
}
