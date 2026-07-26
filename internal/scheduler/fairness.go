package scheduler

import (
	"sort"

	"github.com/buildplane/buildplane/internal/domain"
)

type Candidate struct {
	ID            string
	TenantID      string
	TenantWeight  int
	Priority      domain.Priority
	QueuedOrdinal int
	CPUMillis     int
	TenantActive  int
	TenantLimit   int
}

type Decision struct {
	Candidate Candidate
	Admitted  bool
	Reason    string
}

type Limits struct {
	GlobalActive         int
	GlobalActiveLimit    int
	MaxCPUMillisInFlight int
	CPUMillisInFlight    int
}

type FairScheduler struct {
	deficit map[string]int
	cursor  int
}

func NewFairScheduler() *FairScheduler {
	return &FairScheduler{deficit: map[string]int{}}
}

func (s *FairScheduler) Choose(candidates []Candidate, limits Limits) Decision {
	if limits.GlobalActive >= limits.GlobalActiveLimit {
		return Decision{Reason: "global concurrency saturated"}
	}
	if len(candidates) == 0 {
		return Decision{Reason: "no candidates"}
	}
	priorityCycle := []domain.Priority{
		domain.PriorityCritical, domain.PriorityCritical, domain.PriorityCritical, domain.PriorityCritical,
		domain.PriorityCritical, domain.PriorityCritical, domain.PriorityCritical, domain.PriorityCritical,
		domain.PriorityHigh, domain.PriorityHigh, domain.PriorityHigh, domain.PriorityHigh,
		domain.PriorityNormal, domain.PriorityNormal,
		domain.PriorityLow,
	}
	for round := 0; round < len(priorityCycle); round++ {
		priority := priorityCycle[s.cursor%len(priorityCycle)]
		s.cursor++
		filtered := make([]Candidate, 0, len(candidates))
		for _, c := range candidates {
			if c.Priority == priority {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			continue
		}
		sort.SliceStable(filtered, func(i, j int) bool {
			if filtered[i].TenantID != filtered[j].TenantID {
				return filtered[i].TenantID < filtered[j].TenantID
			}
			return filtered[i].QueuedOrdinal < filtered[j].QueuedOrdinal
		})
		var best *Candidate
		bestKey := ""
		bestDeficit := -1
		for i := range filtered {
			c := filtered[i]
			if c.TenantLimit > 0 && c.TenantActive >= c.TenantLimit {
				continue
			}
			if limits.CPUMillisInFlight+c.CPUMillis > limits.MaxCPUMillisInFlight {
				continue
			}
			if c.TenantWeight <= 0 {
				c.TenantWeight = 1
			}
			key := string(c.Priority) + ":" + c.TenantID
			s.deficit[key] += c.TenantWeight
			if s.deficit[key] > bestDeficit {
				best = &c
				bestKey = key
				bestDeficit = s.deficit[key]
			}
		}
		if best == nil {
			continue
		}
		c := *best
		if c.TenantWeight <= 0 {
			c.TenantWeight = 1
		}
		cost := c.CPUMillis / 1000
		if cost < 1 {
			cost = 1
		}
		if s.deficit[bestKey] >= cost {
			s.deficit[bestKey] -= cost
			return Decision{Candidate: *best, Admitted: true}
		}
	}
	return Decision{Reason: "no tenant has enough deficit or resources"}
}
