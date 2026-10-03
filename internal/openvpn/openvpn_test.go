package openvpn

import (
	"net/netip"
	"testing"
)

const sampleOvpn = `
client
dev tun
proto udp
remote vpn.example.com 1194
resolv-retry infinite
nobind
persist-key
persist-tun
dhcp-option DNS 1.1.1.1
dhcp-option DNS 8.8.8.8
cipher AES-256-GCM
<ca>
-----BEGIN CERTIFICATE-----
MIIB...
-----END CERTIFICATE-----
</ca>
<cert>
-----BEGIN CERTIFICATE-----
MIIC...
-----END CERTIFICATE-----
</cert>
<key>
-----BEGIN PRIVATE KEY-----
MIIE...
-----END PRIVATE KEY-----
</key>
`

func TestIsConfig(t *testing.T) {
	if !IsConfig(sampleOvpn) {
		t.Errorf("expected sampleOvpn to be recognized as OpenVPN config")
	}

	wgSample := `
[Interface]
PrivateKey = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
Address = 10.0.0.2/32
`
	if IsConfig(wgSample) {
		t.Errorf("wireguard config should not be recognized as OpenVPN config")
	}

	vlessSample := "vless://b831381d-6324-4d53-ad4f-8cda48b30811@example.com:443"
	if IsConfig(vlessSample) {
		t.Errorf("vless link should not be recognized as OpenVPN config")
	}
}

func TestParse(t *testing.T) {
	cfg, err := Parse(sampleOvpn)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if cfg.RemoteHost != "vpn.example.com" {
		t.Errorf("expected RemoteHost to be 'vpn.example.com', got %q", cfg.RemoteHost)
	}
	if cfg.RemotePort != "1194" {
		t.Errorf("expected RemotePort to be '1194', got %q", cfg.RemotePort)
	}
	if cfg.Proto != "udp" {
		t.Errorf("expected Proto to be 'udp', got %q", cfg.Proto)
	}
	if len(cfg.DNS) != 2 {
		t.Fatalf("expected 2 DNS addresses, got %d", len(cfg.DNS))
	}
	if cfg.DNS[0] != netip.MustParseAddr("1.1.1.1") || cfg.DNS[1] != netip.MustParseAddr("8.8.8.8") {
		t.Errorf("unexpected DNS addresses: %v", cfg.DNS)
	}

	host, port, proto, err := cfg.Endpoint()
	if err != nil {
		t.Fatalf("endpoint error: %v", err)
	}
	if host != "vpn.example.com" || port != "1194" || proto != "udp" {
		t.Errorf("endpoint mismatch: %s %s %s", host, port, proto)
	}
}

func TestParseTcpAndIpv6(t *testing.T) {
	raw := `
client
dev tun
proto tcp-client
remote 198.51.100.1
port 443
tun-ipv6
`
	cfg, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if cfg.Proto != "tcp" {
		t.Errorf("expected tcp, got %s", cfg.Proto)
	}
	if cfg.RemotePort != "443" {
		t.Errorf("expected 443, got %s", cfg.RemotePort)
	}
	if !cfg.HasIPv6 {
		t.Errorf("expected HasIPv6 true")
	}
}
