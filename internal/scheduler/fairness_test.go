package scheduler

import (
	"testing"

	"github.com/buildplane/buildplane/internal/domain"
)

func TestCriticalReceivesMoreOpportunities(t *testing.T) {
	fs := NewFairScheduler()
	limits := Limits{GlobalActiveLimit: 100, MaxCPUMillisInFlight: 1_000_000}
	counts := map[domain.Priority]int{}
	for i := 0; i < 30; i++ {
		d := fs.Choose([]Candidate{
			{ID: "critical", TenantID: "a", TenantWeight: 1, Priority: domain.PriorityCritical, CPUMillis: 1000},
			{ID: "low", TenantID: "b", TenantWeight: 1, Priority: domain.PriorityLow, CPUMillis: 1000},
		}, limits)
		if !d.Admitted {
			t.Fatal(d.Reason)
		}
		counts[d.Candidate.Priority]++
	}
	if counts[domain.PriorityCritical] <= counts[domain.PriorityLow] {
		t.Fatalf("critical should win more often: %#v", counts)
	}
	if counts[domain.PriorityLow] == 0 {
		t.Fatal("low priority starved")
	}
}

func TestTenantWeightAffectsLongTermShare(t *testing.T) {
	fs := NewFairScheduler()
	limits := Limits{GlobalActiveLimit: 100, MaxCPUMillisInFlight: 1_000_000}
	counts := map[string]int{}
	for i := 0; i < 50; i++ {
		d := fs.Choose([]Candidate{
			{ID: "a", TenantID: "a", TenantWeight: 1, Priority: domain.PriorityNormal, CPUMillis: 1000},
			{ID: "b", TenantID: "b", TenantWeight: 3, Priority: domain.PriorityNormal, CPUMillis: 1000},
		}, limits)
		if !d.Admitted {
			t.Fatal(d.Reason)
		}
		counts[d.Candidate.TenantID]++
	}
	if counts["b"] <= counts["a"] {
		t.Fatalf("higher-weight tenant should receive more allocation: %#v", counts)
	}
}

func TestConcurrencyLimitsRespected(t *testing.T) {
	fs := NewFairScheduler()
	d := fs.Choose([]Candidate{{ID: "a", TenantID: "a", TenantWeight: 1, Priority: domain.PriorityNormal, CPUMillis: 1000}}, Limits{
		GlobalActive: 1, GlobalActiveLimit: 1, MaxCPUMillisInFlight: 1000,
	})
	if d.Admitted {
		t.Fatal("expected saturated global limit to reject")
	}
}
