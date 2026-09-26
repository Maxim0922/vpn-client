package daemon

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	xexec "github.com/max-tsx/max-vpn/internal/exec"
	"github.com/max-tsx/max-vpn/internal/firewall"
	"github.com/max-tsx/max-vpn/internal/logbuf"
	"github.com/max-tsx/max-vpn/internal/netcfg"
	"github.com/max-tsx/max-vpn/internal/state"
	"github.com/max-tsx/max-vpn/internal/stats"
	"github.com/max-tsx/max-vpn/internal/tunnel"
	"github.com/max-tsx/max-vpn/internal/vless"
	"github.com/max-tsx/max-vpn/internal/wgconf"

	"golang.zx2c4.com/wireguard/device"
)

const (
	StateDisconnected = "disconnected"
	StateConnecting   = "connecting"
	StateConnected    = "connected"
	StateError        = "error"
)

var HandshakeTimeout = 10 * time.Second

type Status struct {
	State    string `json:"state"`
	Server   string `json:"server"`
	PublicIP string `json:"publicIP,omitempty"`
	Since    string `json:"since,omitempty"`
	Error    string `json:"error,omitempty"`
}

type Manager struct {
	runner xexec.Runner
	log    func(string)

	ring          *logbuf.Ring
	mu            sync.Mutex
	tun           link
	status        Status
	since         time.Time
	settings      *Settings
	endpointIP    string
	endpointGW    string
	currentServer string
	sup           *superviseCtx
	rate          stats.RateCalc
}

func NewManager(runner xexec.Runner, log func(string)) *Manager {
	if log == nil {
		log = func(string) {}
	}
	ring := logbuf.New(1000)
	wrapped := func(s string) {
		ring.Add("info", s)
		log(s)
	}
	st, err := LoadSettings()
	if err != nil {
		wrapped(fmt.Sprintf("load settings: %v (using defaults)", err))
		st = &Settings{PublicIPService: "https://api.ipify.org"}
	}
	return &Manager{runner: runner, log: wrapped, ring: ring, status: Status{State: StateDisconnected}, settings: st}
}

func (m *Manager) Logs(since time.Time) []logbuf.Entry { return m.ring.Since(since) }

const (
	ProtoWireGuard = "wireguard"
	ProtoVLESS     = "vless"
)

type ServerInfo struct {
	ID       string `json:"id"`
	Favorite bool   `json:"favorite"`
	Protocol string `json:"protocol"`
}

func (m *Manager) ToggleFavorite(id string) error {
	s := m.GetSettings()
	found := false
	out := s.Favorites[:0]
	for _, f := range s.Favorites {
		if f == id {
			found = true
			continue
		}
		out = append(out, f)
	}
	if !found {
		out = append(out, id)
	}
	s.Favorites = out
	return m.SetSettings(s)
}

func (m *Manager) isFavorite(id string) bool {
	for _, f := range m.GetSettings().Favorites {
		if f == id {
			return true
		}
	}
	return false
}

func (m *Manager) GetSettings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return *m.settings
}

func (m *Manager) SetSettings(s Settings) error {
	if s.PublicIPService == "" {
		s.PublicIPService = "https://api.ipify.org"
	}
	if err := SaveSettings(&s); err != nil {
		return err
	}
	m.mu.Lock()
	m.settings = &s
	m.mu.Unlock()
	return nil
}

func (m *Manager) logf(f string, a ...any) { m.log(fmt.Sprintf(f, a...)) }

func serversDir() string { return filepath.Join(state.DefaultDir, "servers") }

var serverExt = map[string]string{".conf": ProtoWireGuard, ".vless": ProtoVLESS}

func serverPath(id, ext string) string { return filepath.Join(serversDir(), id+ext) }

func validID(id string) error {
	if id == "" || len(id) > 200 || strings.HasPrefix(id, ".") || strings.ContainsAny(id, "/\\\x00") {
		return fmt.Errorf("invalid server name %q", id)
	}
	return nil
}

func (m *Manager) ImportConfig(id, confText string) error {
	if err := validID(id); err != nil {
		return err
	}
	ext, data := ".conf", confText
	if vless.IsLink(confText) {
		if _, err := vless.Parse(confText); err != nil {
			return fmt.Errorf("invalid vless link: %w", err)
		}
		ext, data = ".vless", strings.TrimSpace(confText)+"\n"
	} else if _, err := wgconf.Parse(confText); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	if err := os.MkdirAll(serversDir(), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(serverPath(id, ext), []byte(data), 0o600); err != nil {
		return err
	}
	for other := range serverExt {
		if other != ext {
			_ = os.Remove(serverPath(id, other))
		}
	}
	m.logf("imported %s server %q", serverExt[ext], id)
	return nil
}

func (m *Manager) ListServers() ([]ServerInfo, error) {
	entries, err := os.ReadDir(serversDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []ServerInfo
	for _, e := range entries {
		name := e.Name()
		ext := filepath.Ext(name)
		if proto, ok := serverExt[ext]; ok {
			id := strings.TrimSuffix(name, ext)
			out = append(out, ServerInfo{ID: id, Favorite: m.isFavorite(id), Protocol: proto})
		}
	}
	return out, nil
}

func (m *Manager) RemoveServer(id string) error {
	if err := validID(id); err != nil {
		return err
	}
	for ext := range serverExt {
		if err := os.Remove(serverPath(id, ext)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

type server struct {
	wg *wgconf.Config
	vl *vless.Config
}

func (s server) String() string {
	if s.vl != nil {
		return s.vl.String()
	}
	return s.wg.String()
}

func (s server) endpoint() (host, port, proto string, err error) {
	if s.vl != nil {
		host, port = s.vl.Endpoint()
		return host, port, "tcp", nil
	}
	host, port, err = s.wg.Endpoint()
	return host, port, "udp", err
}

func (m *Manager) loadServer(id string) (server, error) {
	if err := validID(id); err != nil {
		return server{}, err
	}
	if data, err := os.ReadFile(serverPath(id, ".vless")); err == nil {
		c, err := vless.Parse(string(data))
		if err != nil {
			return server{}, fmt.Errorf("server %q: %w", id, err)
		}
		return server{vl: c}, nil
	}
	data, err := os.ReadFile(serverPath(id, ".conf"))
	if err != nil {
		return server{}, fmt.Errorf("read server %q: %w", id, err)
	}
	c, err := wgconf.Parse(string(data))
	if err != nil {
		return server{}, err
	}
	return server{wg: c}, nil
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.status
	if !m.since.IsZero() {
		s.Since = m.since.Format(time.RFC3339)
	}
	return s
}

func (m *Manager) setStatus(s Status) {
	m.mu.Lock()
	m.status = s
	m.mu.Unlock()
}

func (m *Manager) Connect(ctx context.Context, id string) error {
	m.mu.Lock()
	if m.tun != nil {
		m.mu.Unlock()
		return fmt.Errorf("already connected")
	}
	m.mu.Unlock()

	srv, err := m.loadServer(id)
	if err != nil {
		return err
	}
	if srv.vl != nil && StartVLESS == nil {
		return fmt.Errorf("this build has no VLESS support")
	}
	set := m.GetSettings()
	m.setStatus(Status{State: StateConnecting, Server: id})
	m.logf("connecting to %q (%v)", id, srv)

	host, port, proto, err := srv.endpoint()
	if err != nil {
		return m.fail(id, "endpoint parse", err, nil)
	}
	endpointIP, err := resolveIP(ctx, host)
	if err != nil {
		return m.fail(id, "resolve endpoint", err, nil)
	}

	st := &state.State{EndpointIP: endpointIP}
	gw4, err := netcfg.DefaultGatewayV4(ctx, m.runner)
	if err != nil {
		return m.fail(id, "default gateway", err, st)
	}
	st.OrigGWv4 = gw4
	st.OrigGWv6, _ = netcfg.DefaultGatewayV6(ctx, m.runner)

	services, err := netcfg.ActiveServices(ctx, m.runner)
	if err != nil {
		return m.fail(id, "list services", err, st)
	}
	st.OrigDNS, err = netcfg.CurrentDNS(ctx, m.runner, services)
	if err != nil {
		return m.fail(id, "read dns", err, st)
	}
	if err := state.Save(st); err != nil {
		return m.fail(id, "save state", err, st)
	}

	var dns []netip.Addr
	if srv.wg != nil {
		dns = srv.wg.Interface.DNS
	}
	if len(set.CustomDNS) > 0 {
		dns = nil
		for _, s := range set.CustomDNS {
			if a, err := netip.ParseAddr(s); err == nil {
				dns = append(dns, a)
			}
		}
	}

	var hasV6 bool
	if srv.vl != nil {
		if len(dns) == 0 {
			dns = []netip.Addr{netip.MustParseAddr("1.1.1.1")}
		}
		if err := m.upVLESS(srv.vl, endpointIP, dns[0], st); err != nil {
			return m.fail(id, "start vless", err, st)
		}
	} else {
		if err := m.upWireGuard(ctx, srv.wg, endpointIP, st); err != nil {
			return m.fail(id, "start wireguard", err, st)
		}
		hasV6 = srv.wg.Interface.HasIPv6()
	}
	m.mu.Lock()
	m.endpointIP = endpointIP
	m.endpointGW = gw4
	m.rate.Reset()
	m.mu.Unlock()

	if err := netcfg.Apply(ctx, m.runner, netcfg.EndpointRoutePlan(endpointIP, gw4)); err != nil {
		return m.fail(id, "endpoint route", err, st)
	}
	st.EndpointRouteInstalled = true
	_ = state.Save(st)

	plan, v6blocked := netcfg.FullTunnelPlan(st.Iface, hasV6)
	if err := netcfg.Apply(ctx, m.runner, plan); err != nil {
		return m.fail(id, "full tunnel routes", err, st)
	}
	st.IPv6Blocked = v6blocked
	_ = state.Save(st)

	if len(dns) > 0 {
		if err := netcfg.Apply(ctx, m.runner, netcfg.DNSPlan(services, dns)); err != nil {
			return m.fail(id, "set dns", err, st)
		}
		st.DNSApplied = services
		_ = state.Save(st)
	}

	if set.KillSwitch {
		tok, err := firewall.Enable(ctx, m.runner, firewall.Rules{
			Iface:         st.Iface,
			EndpointIP:    endpointIP,
			EndpointPort:  port,
			EndpointProto: proto,
			AllowLAN:      set.AllowLAN,
			HasV6:         hasV6,
		})
		if err != nil {
			return m.fail(id, "kill switch", err, st)
		}
		st.PFToken = tok
		_ = state.Save(st)
	}

	if err := m.waitReady(ctx); err != nil {
		return m.fail(id, "handshake", err, st)
	}

	now := time.Now()
	m.mu.Lock()
	m.status = Status{State: StateConnected, Server: id}
	m.since = now
	m.mu.Unlock()
	m.logf("connected to %q on %s", id, st.Iface)

	set.LastServer = id
	_ = SaveSettings(&set)

	m.startSupervision(id)

	go m.refreshPublicIP(set.PublicIPService)
	return nil
}

func (m *Manager) refreshPublicIP(service string) {
	ip, err := fetchPublicIP(service)
	if err != nil {
		m.logf("public IP lookup failed: %v", err)
		return
	}
	m.mu.Lock()
	if m.status.State == StateConnected {
		m.status.PublicIP = ip
	}
	m.mu.Unlock()
}

func (m *Manager) upWireGuard(ctx context.Context, cfg *wgconf.Config, endpointIP string, st *state.State) error {
	mtu := cfg.Interface.MTU
	if mtu == 0 {
		mtu = 1420
	}
	tun, err := tunnel.Create("utun", mtu, newLogger(m.log))
	if err != nil {
		return fmt.Errorf("create tun: %w", err)
	}
	m.setLink(tun, st)

	dev := *cfg
	dev.Peers = append([]wgconf.Peer(nil), cfg.Peers...)
	for i := range dev.Peers {
		host, port, err := net.SplitHostPort(dev.Peers[i].Endpoint)
		if err != nil {
			return fmt.Errorf("peer %d endpoint: %w", i, err)
		}
		ip := endpointIP
		if i > 0 {
			if ip, err = resolveIP(ctx, host); err != nil {
				return fmt.Errorf("resolve peer %d endpoint: %w", i, err)
			}
		}
		dev.Peers[i].Endpoint = net.JoinHostPort(ip, port)
	}
	uapi, err := dev.IPCSet()
	if err != nil {
		return fmt.Errorf("build uapi: %w", err)
	}
	if err := tun.Configure(uapi); err != nil {
		return fmt.Errorf("configure device: %w", err)
	}
	if err := tun.Up(); err != nil {
		return fmt.Errorf("device up: %w", err)
	}
	if err := netcfg.Apply(ctx, m.runner, netcfg.AddressPlan(st.Iface, cfg.Interface.Address)); err != nil {
		return fmt.Errorf("assign addresses: %w", err)
	}
	return nil
}

func (m *Manager) upVLESS(cfg *vless.Config, endpointIP string, dns netip.Addr, st *state.State) error {
	iface, err := nextUtun()
	if err != nil {
		return err
	}
	t, err := StartVLESS(cfg, vless.Options{Iface: iface, MTU: 1500, ServerIP: endpointIP, DNS: dns})
	if err != nil {
		return err
	}
	m.setLink(vlessLink{VLESSTunnel: t, runner: m.runner}, st)
	return nil
}

func (m *Manager) setLink(l link, st *state.State) {
	st.Iface = l.Name()
	_ = state.Save(st)
	m.mu.Lock()
	m.tun = l
	m.mu.Unlock()
}

func (m *Manager) waitReady(ctx context.Context) error {
	m.mu.Lock()
	l := m.tun
	m.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, HandshakeTimeout)
	defer cancel()

	if v, ok := l.(vlessLink); ok {
		var err error
		for {
			pctx, pcancel := context.WithTimeout(ctx, 5*time.Second)
			err = v.Probe(pctx)
			pcancel()
			if err == nil {
				return nil
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("no response through proxy within %s: %w", HandshakeTimeout, err)
			case <-time.After(500 * time.Millisecond):
			}
		}
	}

	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		if s, err := l.Stats(); err == nil && !s.LastHandshake.IsZero() {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("no handshake within %s", HandshakeTimeout)
		case <-tick.C:
		}
	}
}

func (m *Manager) fail(id, step string, cause error, st *state.State) error {
	m.logf("connect failed at %s: %v — rolling back", step, cause)
	m.teardownLocked(context.Background(), st)
	m.setStatus(Status{State: StateError, Server: id, Error: fmt.Sprintf("%s: %v", step, cause)})
	return fmt.Errorf("%s: %w", step, cause)
}

func (m *Manager) Disconnect(ctx context.Context) error {
	m.stopSupervision()
	return m.teardownOnly(ctx)
}

func (m *Manager) teardownOnly(ctx context.Context) error {
	st, _ := state.Load()
	err := m.teardownLocked(ctx, st)
	m.mu.Lock()
	m.status = Status{State: StateDisconnected}
	m.since = time.Time{}
	m.endpointIP = ""
	m.endpointGW = ""
	m.rate.Reset()
	m.mu.Unlock()
	return err
}

func (m *Manager) Stats(ctx context.Context) stats.Snapshot {
	m.mu.Lock()
	t := m.tun
	endpoint := m.endpointIP
	connected := m.status.State == StateConnected
	m.mu.Unlock()

	snap := stats.Snapshot{LatencyMs: -1}
	if t == nil || !connected {
		return snap
	}
	s, err := t.Stats()
	if err != nil {
		return snap
	}
	snap.RxBytes, snap.TxBytes = s.RxBytes, s.TxBytes
	snap.RxRate, snap.TxRate = m.rate.Update(s.RxBytes, s.TxBytes, time.Now())
	if !s.LastHandshake.IsZero() {
		snap.LastHandshake = s.LastHandshake.Unix()
	}
	if endpoint != "" {
		snap.LatencyMs = stats.PingLatency(ctx, m.runner, endpoint)
	}
	return snap
}

func (m *Manager) teardownLocked(ctx context.Context, st *state.State) error {
	m.mu.Lock()
	t := m.tun
	m.tun = nil
	m.mu.Unlock()
	if t != nil {
		_ = t.Close()
	}
	return state.Teardown(ctx, m.runner, st, m.log)
}

func (m *Manager) RecoverFromCrash(ctx context.Context) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	if st == nil {
		return nil
	}
	m.logf("stale state file found (%s) — cleaning up after previous crash", st.Iface)
	return state.Teardown(ctx, m.runner, st, m.log)
}

func resolveIP(ctx context.Context, host string) (string, error) {
	if _, err := netip.ParseAddr(host); err == nil {
		return host, nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", err
	}

	for _, a := range addrs {
		if a.IP.To4() != nil {
			return a.IP.String(), nil
		}
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("no addresses for %q", host)
	}
	return addrs[0].IP.String(), nil
}

func newLogger(log func(string)) *device.Logger {
	return &device.Logger{
		Verbosef: func(f string, a ...any) { log("wg: " + fmt.Sprintf(f, a...)) },
		Errorf:   func(f string, a ...any) { log("wg-error: " + fmt.Sprintf(f, a...)) },
	}
}
