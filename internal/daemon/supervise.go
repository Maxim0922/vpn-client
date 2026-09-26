package daemon

import (
	"context"
	"time"

	"github.com/max-tsx/max-vpn/internal/netcfg"
	"github.com/max-tsx/max-vpn/internal/state"
)

var staleHandshake = 180 * time.Second

var backoffSchedule = []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second}

func backoffAt(n int) time.Duration {
	if n < 0 {
		n = 0
	}
	if n >= len(backoffSchedule) {
		return backoffSchedule[len(backoffSchedule)-1]
	}
	return backoffSchedule[n]
}

type superviseCtx struct {
	cancel context.CancelFunc
}

func (m *Manager) startSupervision(server string) {
	m.stopSupervision()
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.sup = &superviseCtx{cancel: cancel}
	m.currentServer = server
	m.mu.Unlock()
	go m.watchdog(ctx, server)
}

func (m *Manager) stopSupervision() {
	m.mu.Lock()
	sup := m.sup
	m.sup = nil
	m.currentServer = ""
	m.mu.Unlock()
	if sup != nil {
		sup.cancel()
	}
}

func (m *Manager) watchdog(ctx context.Context, server string) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.mu.Lock()
			tun := m.tun
			connected := m.status.State == StateConnected
			m.mu.Unlock()
			if tun == nil || !connected {
				continue
			}
			s, err := tun.Stats()
			if err != nil {
				continue
			}
			if s.LastHandshake.IsZero() || time.Since(s.LastHandshake) > staleHandshake {
				m.logf("handshake stale (%s) — reconnecting", time.Since(s.LastHandshake).Round(time.Second))
				m.reconnect(ctx, server)
				return
			}
		}
	}
}

func (m *Manager) reconnect(ctx context.Context, server string) {
	_ = m.teardownOnly(context.Background())
	for attempt := 0; ; attempt++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoffAt(attempt)):
		}
		if err := m.Connect(context.Background(), server); err != nil {
			m.logf("reconnect attempt %d failed: %v", attempt+1, err)
			continue
		}
		m.logf("reconnected to %q", server)
		return
	}
}

func (m *Manager) OnNetworkChange(ctx context.Context) {
	m.mu.Lock()
	connected := m.status.State == StateConnected
	endpointIP := m.endpointIP
	oldGW := m.endpointGW
	m.mu.Unlock()
	if !connected || endpointIP == "" {
		return
	}

	newGW, err := netcfg.DefaultGatewayV4(ctx, m.runner)
	if err != nil || newGW == "" || newGW == oldGW {
		return
	}
	m.logf("default gateway changed %s -> %s, re-pinning endpoint route", oldGW, newGW)

	_, _ = m.runner.Run(ctx, "route", "-n", "delete", "-host", endpointIP)
	if err := netcfg.Apply(ctx, m.runner, netcfg.EndpointRoutePlan(endpointIP, newGW)); err != nil {
		m.logf("re-pin endpoint route failed: %v", err)
		return
	}
	m.mu.Lock()
	m.endpointGW = newGW
	m.mu.Unlock()

	if st, _ := state.Load(); st != nil {
		st.EndpointRouteInstalled = true
		_ = state.Save(st)
	}
}
