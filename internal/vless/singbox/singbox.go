package singbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter/certificate"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/direct"
	"github.com/sagernet/sing-box/protocol/tun"
	protovless "github.com/sagernet/sing-box/protocol/vless"
	sjson "github.com/sagernet/sing/common/json"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/max-tsx/max-vpn/internal/vless"
)

type Tunnel struct {
	box   *box.Box
	name  string
	proxy func(ctx context.Context, network string, dest M.Socksaddr) (net.Conn, error)

	mu        sync.Mutex
	lastAlive time.Time
	stop      context.CancelFunc
}

func buildConfig(c *vless.Config, o vless.Options) ([]byte, error) {
	out := map[string]any{
		"type":            "vless",
		"tag":             "proxy",
		"server":          o.ServerIP,
		"server_port":     c.Port,
		"uuid":            c.UUID,
		"packet_encoding": "xudp",
	}
	if c.Flow != "" {
		out["flow"] = c.Flow
	}

	if c.Security != "none" {
		sni := c.SNI
		if sni == "" && c.Security == "tls" {
			sni = c.Host
		}
		t := map[string]any{"enabled": true, "server_name": sni, "insecure": c.Insecure}
		if len(c.ALPN) > 0 {
			t["alpn"] = c.ALPN
		}
		fp := c.Fingerprint
		if fp == "" && c.Security == "reality" {
			fp = "chrome"
		}
		if fp != "" {
			t["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
		}
		if c.Security == "reality" {
			t["reality"] = map[string]any{"enabled": true, "public_key": c.PublicKey, "short_id": c.ShortID}
		}
		out["tls"] = t
	}

	switch c.Transport {
	case "ws":
		t := map[string]any{"type": "ws", "path": c.Path}
		if c.HostHeader != "" {
			t["headers"] = map[string]any{"Host": c.HostHeader}
		}
		if c.EarlyData > 0 {
			t["max_early_data"] = c.EarlyData
			t["early_data_header_name"] = "Sec-WebSocket-Protocol"
		}
		out["transport"] = t
	case "grpc":
		out["transport"] = map[string]any{"type": "grpc", "service_name": c.ServiceName}
	case "http":
		t := map[string]any{"type": "http", "path": c.Path}
		if c.HostHeader != "" {
			t["host"] = []string{c.HostHeader}
		}
		out["transport"] = t
	case "httpupgrade":
		out["transport"] = map[string]any{"type": "httpupgrade", "host": c.HostHeader, "path": c.Path}
	}

	cfg := map[string]any{
		"log": map[string]any{"level": "warn"},
		"dns": map[string]any{
			"servers": []any{map[string]any{"type": "tcp", "tag": "remote", "server": o.DNS.String(), "detour": "proxy"}},
		},
		"inbounds": []any{map[string]any{
			"type":           "tun",
			"tag":            "tun-in",
			"interface_name": o.Iface,
			"mtu":            o.MTU,
			"address":        []string{vless.TunAddress.String()},
			"stack":          "gvisor",
			"auto_route":     false,
		}},
		"outbounds": []any{out, map[string]any{"type": "direct", "tag": "direct"}},
		"route": map[string]any{
			"rules": []any{
				map[string]any{"action": "sniff"},
				map[string]any{"protocol": "dns", "action": "hijack-dns"},
			},
			"final": "proxy",
		},
	}
	return json.Marshal(cfg)
}

func boxContext(ctx context.Context) context.Context {
	in := inbound.NewRegistry()
	tun.RegisterInbound(in)
	out := outbound.NewRegistry()
	protovless.RegisterOutbound(out)
	direct.RegisterOutbound(out)
	dnsReg := dns.NewTransportRegistry()
	transport.RegisterTCP(dnsReg)
	transport.RegisterUDP(dnsReg)
	return box.Context(ctx, in, out, endpoint.NewRegistry(), dnsReg, service.NewRegistry(), certificate.NewRegistry())
}

func Start(c *vless.Config, o vless.Options) (*Tunnel, error) {
	if o.MTU == 0 {
		o.MTU = 1500
	}
	raw, err := buildConfig(c, o)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(boxContext(context.Background()))
	opts, err := sjson.UnmarshalExtendedContext[option.Options](ctx, raw)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("sing-box options: %w", err)
	}
	b, err := box.New(box.Options{Context: ctx, Options: opts})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("sing-box: %w", err)
	}
	if err := b.Start(); err != nil {
		_ = b.Close()
		cancel()
		return nil, fmt.Errorf("sing-box start: %w", err)
	}
	proxy, ok := b.Outbound().Outbound("proxy")
	if !ok {
		_ = b.Close()
		cancel()
		return nil, fmt.Errorf("sing-box: proxy outbound missing")
	}
	t := &Tunnel{box: b, name: o.Iface, proxy: proxy.DialContext, stop: cancel}
	go t.keepalive(ctx)
	return t, nil
}

func (t *Tunnel) Name() string { return t.name }

const probeURL = "http://cp.cloudflare.com/generate_204"

func (t *Tunnel) Probe(ctx context.Context) error {
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return t.proxy(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	t.mu.Lock()
	t.lastAlive = time.Now()
	t.mu.Unlock()
	return nil
}

func (t *Tunnel) keepalive(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			_ = t.Probe(pctx)
			cancel()
		}
	}
}

func (t *Tunnel) LastAlive() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastAlive
}

func (t *Tunnel) Close() error {
	t.stop()
	return t.box.Close()
}
