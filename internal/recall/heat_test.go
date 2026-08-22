package recall

import (
	"math"
	"testing"
	"time"
)

func TestHeatRankBoost_bounded(t *testing.T) {
	now := int64(1_750_000_000_000)
	day := int64(24 * time.Hour.Milliseconds())

	// Fresh, zero heat: boost is exactly the cold-start novelty term β.
	if got := heatRankBoost(0, now, now); math.Abs(got-heatBetaDefault) > 1e-12 {
		t.Fatalf("boost(heat=0, age=0) = %v, want β=%v", got, heatBetaDefault)
	}

	// Fresh, heavy heat: popularity term added to β.
	hot := heatRankBoost(100, now, now)
	wantPop := heatAlpha * math.Log(101)
	if math.Abs(hot-(betaOfAge(0)+wantPop)) > 1e-12 {
		t.Fatalf("boost(heat=100, age=0) = %v, want %v", hot, betaOfAge(0)+wantPop)
	}

	// Boundedness: the combined boost must stay well below a rank-1 RRF score
	// (≈1/61≈0.0164), so a boosted memory can never overtake a genuinely
	// relevant top hit — even at pathological heat (1e6 cats) the log term
	// grows slowly and stays under ~0.4× a rank-1 RRF.
	if max := heatRankBoost(1_000_000, now, now); max > 0.7*(1.0/61.0) {
		t.Fatalf("max boost %v must stay below 0.7× top-1 RRF", max)
	}
	// At realistic-heavy heat (≈250, far above the ~30 steady-state for daily
	// use given τ=30d), the total is on the order of 10-15% of top-1 RRF.
	if realistic := heatRankBoost(250, now, now); realistic > 0.3*(1.0/61.0) {
		t.Fatalf("realistic-heat boost %v exceeds 30%% of top-1 RRF", realistic)
	}

	// Monotonic increasing in heat.
	if heatRankBoost(50, now, now) <= heatRankBoost(10, now, now) {
		t.Fatal("boost must increase with heat")
	}

	// Monotonic decreasing in age (cold-start novelty decays).
	fresh := heatRankBoost(5, now-1*day, now)
	stale := heatRankBoost(5, now-400*day, now)
	if fresh <= stale {
		t.Fatalf("boost must decay with age: fresh=%v stale=%v", fresh, stale)
	}
	// Age is clamped at 0 (future clocks don't inflate the novelty term).
	if heatRankBoost(0, now+day, now) != heatRankBoost(0, now, now) {
		t.Fatal("negative age must be clamped to 0")
	}
}

func TestHeatRankBoost_ageDecayExact(t *testing.T) {
	now := int64(1_750_000_000_000)
	day := int64(24 * time.Hour.Milliseconds())
	// At exactly 14 days the novelty term is β·e^(−1) ≈ 0.0016·0.3679.
	got := heatRankBoost(0, now-14*day, now)
	want := heatBetaDefault * math.Exp(-1)
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("boost at 14d = %v, want %v", got, want)
	}
}

func TestEnvParsing(t *testing.T) {
	t.Setenv("RMB_HEAT_TEST_BOOL", "true")
	if !envBool("RMB_HEAT_TEST_BOOL", false) {
		t.Fatal("envBool true parse failed")
	}
	t.Setenv("RMB_HEAT_TEST_BOOL", "garbage")
	if !envBool("RMB_HEAT_TEST_BOOL", true) {
		t.Fatal("envBool should fall back to its default (true) on garbage")
	}
	t.Setenv("RMB_HEAT_TEST_FLOAT", "0.5")
	if envFloat("RMB_HEAT_TEST_FLOAT", 0) != 0.5 {
		t.Fatal("envFloat parse failed")
	}
	t.Setenv("RMB_HEAT_TEST_FLOAT", "nope")
	if envFloat("RMB_HEAT_TEST_FLOAT", 0.25) != 0.25 {
		t.Fatal("envFloat should fall back to default on garbage")
	}
}

func betaOfAge(ageDays float64) float64 {
	return heatBeta * math.Exp(-ageDays/heatAgeHalfLifeDays)
}
