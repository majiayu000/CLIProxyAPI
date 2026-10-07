package auth

import (
	"context"
	"testing"
	"time"
)

func TestQuotaThresholdDistinguishesUnknownFromExhausted(t *testing.T) {
	previous := GetMinQuotaFraction()
	t.Cleanup(func() { SetMinQuotaFraction(previous) })
	SetMinQuotaFraction(0.05)
	now := time.Now()
	reset := now.Add(time.Hour)
	cases := []struct {
		name     string
		fraction float64
		checked  bool
		blocked  bool
	}{
		{name: "unknown", fraction: 0},
		{name: "unknown positive", fraction: 0.01},
		{name: "exhausted", fraction: 0, checked: true, blocked: true},
		{name: "below threshold", fraction: 0.04, checked: true, blocked: true},
		{name: "at threshold", fraction: 0.05, checked: true},
		{name: "above threshold", fraction: 0.06, checked: true},
		{name: "full", fraction: 1, checked: true},
	}
	for _, scope := range []string{"account", "model"} {
		for _, tc := range cases {
			t.Run(scope+"/"+tc.name, func(t *testing.T) {
				quota := QuotaState{RemainingFraction: tc.fraction, ResetTime: reset}
				if tc.checked {
					quota.LastCheckedAt = now
				}
				candidate := &Auth{ID: "quota-test", Status: StatusActive}
				if scope == "account" {
					candidate.Quota = quota
				} else {
					candidate.ModelStates = map[string]*ModelState{
						"test-model": {Status: StatusActive, Quota: quota},
					}
				}
				blocked, reason, next := isAuthBlockedForModel(candidate, "test-model", now)
				if blocked != tc.blocked {
					t.Fatalf("blocked = %t, want %t", blocked, tc.blocked)
				}
				if tc.blocked && (reason != blockReasonQuotaLow || !next.Equal(reset)) {
					t.Fatalf("reason = %v, next = %v; want quota-low until %v", reason, next, reset)
				}
			})
		}
	}
	SetMinQuotaFraction(0)
	blocked, _, _ := isAuthBlockedForModel(&Auth{
		Quota: QuotaState{RemainingFraction: 0, LastCheckedAt: now},
	}, "", now)
	if blocked {
		t.Fatal("disabled threshold blocked a known exhausted account")
	}
}

type quotaPollingExecutor struct {
	ProviderExecutor
}

func (e *quotaPollingExecutor) Identifier() string { return "quota-test" }

func (e *quotaPollingExecutor) FetchQuota(context.Context, *Auth) (map[string]QuotaInfo, error) {
	return nil, nil
}

func (e *quotaPollingExecutor) UpdateAuthQuota(*Auth, map[string]QuotaInfo) {}

func TestQuotaPollingInterval(t *testing.T) {
	m := NewManager(nil, nil, nil)
	m.RegisterExecutor(&quotaPollingExecutor{})
	candidate := &Auth{ID: "quota-test", Provider: "quota-test"}
	now := time.Now()
	m.SetQuotaCheckInterval(300)
	if !m.shouldCheckQuota(candidate, now) {
		t.Fatal("first quota check should be due")
	}
	m.lastQuotaCheck.Store(candidate.ID, now)
	if m.shouldCheckQuota(candidate, now.Add(299*time.Second)) {
		t.Fatal("quota check was due before the interval elapsed")
	}
	if !m.shouldCheckQuota(candidate, now.Add(300*time.Second)) {
		t.Fatal("quota check was not due at the interval boundary")
	}
	for _, interval := range []int{0, -1} {
		m.SetQuotaCheckInterval(interval)
		if m.QuotaCheckInterval() != 0 || m.shouldCheckQuota(candidate, now.Add(time.Hour)) {
			t.Fatalf("interval %d did not disable quota polling", interval)
		}
	}
}
