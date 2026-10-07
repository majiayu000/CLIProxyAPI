package api

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestUpdateClientsReloadsQuotaPollingInterval(t *testing.T) {
	previous := auth.GetMinQuotaFraction()
	t.Cleanup(func() { auth.SetMinQuotaFraction(previous) })
	server := newTestServer(t)
	cfg := *server.cfg
	// Config reload must not download the management UI during this test.
	cfg.RemoteManagement.DisableControlPanel = true
	cases := []struct {
		name     string
		fraction float64
		interval int
		want     int
	}{
		{name: "enable threshold", fraction: 0.05, want: 300},
		{name: "change threshold", fraction: 0.10, want: 300},
		{name: "disable threshold", want: 0},
		{name: "explicit interval", interval: 60, want: 60},
		{name: "enable with explicit interval", fraction: 0.05, interval: 60, want: 60},
		{name: "disable with explicit interval", interval: 60, want: 60},
		{name: "disable polling", want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg.QuotaExceeded.MinFraction = tc.fraction
			cfg.QuotaExceeded.CheckInterval = tc.interval
			server.UpdateClients(&cfg)
			if got := server.handlers.AuthManager.QuotaCheckInterval(); got != tc.want {
				t.Fatalf("quota interval = %d, want %d", got, tc.want)
			}
			if got := auth.GetMinQuotaFraction(); got != tc.fraction {
				t.Fatalf("quota threshold = %v, want %v", got, tc.fraction)
			}
		})
	}
}
