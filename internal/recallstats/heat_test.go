package recallstats

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/db"
)

func openStatsDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "rmb.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// fakeClock returns a settable clock.
func fakeClock(start time.Time) (func() time.Time, func(time.Time)) {
	cur := start
	now := func() time.Time { return cur }
	set := func(t time.Time) { cur = t }
	return now, set
}

func TestDecayHeat_math(t *testing.T) {
	const day = 24 * time.Hour
	// First use: no prior use, heat = w.
	if got := DecayHeat(0, 0, 1_000_000, HeatTau, WeightCat); got != WeightCat {
		t.Fatalf("first use: got %v want %v", got, WeightCat)
	}
	// Half-life-ish check: after τ the decayed part is multiplied by e^-1.
	base := 10.0
	lastUse := int64(1)
	nowMS := int64(2_592_000_001) // lastUse + 30d in ms
	got := DecayHeat(base, lastUse, nowMS, HeatTau, 0)
	want := base * 0.36787944117144233 // 10 * e^-1
	if diff := got - want; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("decay after tau: got %v want %v", got, want)
	}
	// Zero elapsed time: heat + w exactly.
	if got := DecayHeat(base, 5, 5, HeatTau, WeightCat); got != base+WeightCat {
		t.Fatalf("no elapsed: got %v", got)
	}
}

func TestHeatWeights_catMeta(t *testing.T) {
	database := openStatsDB(t)
	svc := NewService(database)
	start := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	now, set := fakeClock(start)
	svc.SetClock(now)
	ctx := context.Background()

	if err := svc.RecordCat(ctx, "rmb://entities/jump-hs99-vip"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordMeta(ctx, "rmb://events/2026-08-01-thing"); err != nil {
		t.Fatal(err)
	}

	var catHeat, metaHeat float64
	if err := database.QueryRow(`SELECT heat FROM recall_stats WHERE uri = 'rmb://entities/jump-hs99-vip'`).Scan(&catHeat); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT heat FROM recall_stats WHERE uri = 'rmb://events/2026-08-01-thing'`).Scan(&metaHeat); err != nil {
		t.Fatal(err)
	}
	if catHeat != WeightCat {
		t.Fatalf("cat heat = %v want %v", catHeat, WeightCat)
	}
	if metaHeat != WeightMeta {
		t.Fatalf("meta heat = %v want %v", metaHeat, WeightMeta)
	}

	// Decay between uses: after 30 days a second cat decays the old weight.
	set(start.Add(HeatTau))
	if err := svc.RecordCat(ctx, "rmb://entities/jump-hs99-vip"); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT heat FROM recall_stats WHERE uri = 'rmb://entities/jump-hs99-vip'`).Scan(&catHeat); err != nil {
		t.Fatal(err)
	}
	want := WeightCat*mathE1() + WeightCat
	if diff := catHeat - want; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("decayed cat heat = %v want %v", catHeat, want)
	}
}

func mathE1() float64 {
	return 0.36787944117144233
}

func TestSearchNeverUpdatesHeat_skillPollutionCase(t *testing.T) {
	// The audit's rich-get-richer trap: an irrelevant skill that shows up in
	// every search result list must stay heat=0 from exposure alone.
	database := openStatsDB(t)
	svc := NewService(database)
	now, _ := fakeClock(time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC))
	svc.SetClock(now)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		if err := svc.RecordQuery(ctx, "deploy prod", nil, 5, []string{
			"rmb://skills/draft-aliyun-procurement-ticket",
			"rmb://entities/jump-hs99-vip",
		}); err != nil {
			t.Fatal(err)
		}
		if err := svc.RecordSearch(ctx, []string{
			"rmb://skills/draft-aliyun-procurement-ticket",
			"rmb://entities/jump-hs99-vip",
		}); err != nil {
			t.Fatal(err)
		}
	}

	var skillHeat float64
	var skillSearches int64
	if err := database.QueryRow(
		`SELECT heat, search_count FROM recall_stats WHERE uri = 'rmb://skills/draft-aliyun-procurement-ticket'`,
	).Scan(&skillHeat, &skillSearches); err != nil {
		t.Fatal(err)
	}
	if skillHeat != 0 {
		t.Fatalf("skill heat = %v, want 0 (search exposure must never heat)", skillHeat)
	}
	if skillSearches != 20 {
		t.Fatalf("search_count = %d, want 20 (exposure counter still tracked)", skillSearches)
	}
}

func TestSearchToCatJoin_tenMinuteWindow(t *testing.T) {
	database := openStatsDB(t)
	svc := NewService(database)
	start := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	now, set := fakeClock(start)
	svc.SetClock(now)
	ctx := context.Background()

	target := "rmb://skills/jump-hs99-vip"
	if err := svc.RecordQuery(ctx, "bastion ssh", []string{"memory", "skill"}, 5, []string{
		"rmb://entities/rmbd", target,
	}); err != nil {
		t.Fatal(err)
	}

	// Cat 9 minutes later: joins.
	set(start.Add(9 * time.Minute))
	if err := svc.RecordCat(ctx, target); err != nil {
		t.Fatal(err)
	}
	var catted sql.NullString
	if err := database.QueryRow(`SELECT catted_uri FROM search_queries ORDER BY id DESC LIMIT 1`).Scan(&catted); err != nil {
		t.Fatal(err)
	}
	if !catted.Valid || catted.String != target {
		t.Fatalf("expected join to %s, got %+v", target, catted)
	}

	// Cat 12 minutes after a fresh search: does NOT join.
	if err := svc.RecordQuery(ctx, "fresh query", nil, 5, []string{target}); err != nil {
		t.Fatal(err)
	}
	set(start.Add(9*time.Minute + 12*time.Minute))
	if err := svc.RecordCat(ctx, target); err != nil {
		t.Fatal(err)
	}
	var catted2 sql.NullString
	if err := database.QueryRow(`SELECT catted_uri FROM search_queries WHERE query = 'fresh query'`).Scan(&catted2); err != nil {
		t.Fatal(err)
	}
	if catted2.Valid {
		t.Fatalf("cat outside 10-min window must not join, got %v", catted2.String)
	}
}

func TestDoctorMetrics_zeroCatRateAndConcentration(t *testing.T) {
	database := openStatsDB(t)
	svc := NewService(database)
	start := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	now, set := fakeClock(start)
	svc.SetClock(now)
	ctx := context.Background()

	// 4 searches; 2 get cats (within window), 2 do not → zero-cat rate 0.5.
	for i, converted := range []bool{true, false, true, false} {
		uri := "rmb://entities/hot-" + string(rune('a'+i))
		if err := svc.RecordQuery(ctx, "q", nil, 5, []string{uri}); err != nil {
			t.Fatal(err)
		}
		if converted {
			set(start.Add(time.Duration(i) * time.Minute))
			if err := svc.RecordCat(ctx, uri); err != nil {
				t.Fatal(err)
			}
		}
	}
	set(start.Add(time.Hour))
	m, err := svc.DoctorMetrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if m.Searches != 4 || m.ConvertedSearch != 2 {
		t.Fatalf("searches=%d converted=%d, want 4/2", m.Searches, m.ConvertedSearch)
	}
	if m.ZeroCatRate != 0.5 {
		t.Fatalf("zero-cat rate = %v want 0.5", m.ZeroCatRate)
	}
	if m.TotalCats != 2 {
		t.Fatalf("total cats = %d want 2", m.TotalCats)
	}
	// Both cats are on the two hottest (only) memories → concentration 1.0.
	if m.HeatConcentration != 1.0 || !m.HeatAlarm {
		t.Fatalf("concentration = %v alarm=%v, want 1.0/true", m.HeatConcentration, m.HeatAlarm)
	}
}
