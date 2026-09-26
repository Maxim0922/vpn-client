package firewall

import (
	"context"
	"strings"
	"testing"

	xexec "github.com/max-tsx/max-vpn/internal/exec"
)

func TestGenerateNoLAN(t *testing.T) {
	rs := Rules{Iface: "utun6", EndpointIP: "203.0.113.5", EndpointPort: "51820"}
	out := rs.Generate()
	for _, want := range []string{
		"block all",
		"pass quick on lo0 all",
		"pass quick on utun6 all",
		"pass out quick proto udp to 203.0.113.5 port 51820",
		"pass out quick proto udp from any port 68 to any port 67",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "10.0.0.0/8") {
		t.Errorf("LAN rules present without AllowLAN:\n%s", out)
	}
}

func TestGenerateTCPEndpoint(t *testing.T) {
	out := Rules{Iface: "utun7", EndpointIP: "203.0.113.5", EndpointPort: "443", EndpointProto: "tcp"}.Generate()
	if !strings.Contains(out, "pass out quick proto tcp to 203.0.113.5 port 443") {
		t.Errorf("missing tcp endpoint rule in:\n%s", out)
	}
	if strings.Contains(out, "proto udp to 203.0.113.5") {
		t.Errorf("unexpected udp endpoint rule in:\n%s", out)
	}
}

func TestGenerateWithLAN_V4andV6(t *testing.T) {
	rs := Rules{Iface: "utun6", EndpointIP: "203.0.113.5", EndpointPort: "51820", AllowLAN: true}
	out := rs.Generate()
	for _, want := range []string{
		"inet from any to { 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 169.254.0.0/16, 224.0.0.0/4 }",
		"inet from { 10.0.0.0/8",
		"inet6 from any to { fe80::/10, fc00::/7, ff00::/8 }",
		"inet6 from { fe80::/10",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing LAN rule %q in:\n%s", want, out)
		}
	}
}

func TestGenerateV6Endpoint(t *testing.T) {
	rs := Rules{Iface: "utun6", EndpointIP: "2001:db8::1", EndpointPort: "51820"}
	out := rs.Generate()
	if !strings.Contains(out, "pass out quick proto udp to 2001:db8::1 port 51820") {
		t.Errorf("v6 endpoint rule wrong:\n%s", out)
	}
}

func TestEnableParsesToken(t *testing.T) {
	d := xexec.NewDryRunner()
	d.Outputs["pfctl"] = [][]byte{
		nil,
		[]byte("pf enabled\nToken : 1234567890\n"),
	}
	tok, err := Enable(context.Background(), d, Rules{Iface: "utun6", EndpointIP: "1.2.3.4", EndpointPort: "51820"})
	if err != nil {
		t.Fatal(err)
	}
	if tok != "1234567890" {
		t.Errorf("token: %q", tok)
	}

	if len(d.Stdin) < 1 || !strings.Contains(string(d.Stdin[0]), "block all") {
		t.Errorf("ruleset not fed on stdin: %v", d.Stdin)
	}
	lines := strings.Join(d.Lines(), "\n")
	if !strings.Contains(lines, "pfctl -a com.apple/maxvpn -f -") {
		t.Errorf("missing anchor load cmd:\n%s", lines)
	}
}

func TestEnableNoToken(t *testing.T) {
	d := xexec.NewDryRunner()
	d.Outputs["pfctl"] = [][]byte{nil, []byte("pf already enabled\n")}
	if _, err := Enable(context.Background(), d, Rules{}); err == nil {
		t.Errorf("expected error when no token in output")
	}
}

func TestDisable(t *testing.T) {
	d := xexec.NewDryRunner()
	if err := Disable(context.Background(), d, "42"); err != nil {
		t.Fatal(err)
	}
	lines := strings.Join(d.Lines(), "\n")
	if !strings.Contains(lines, "pfctl -X 42") {
		t.Errorf("missing token release:\n%s", lines)
	}
	if !strings.Contains(lines, "pfctl -a com.apple/maxvpn -F all") {
		t.Errorf("missing anchor flush:\n%s", lines)
	}
}
