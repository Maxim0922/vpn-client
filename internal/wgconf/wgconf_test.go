package wgconf

import (
	"fmt"
	"strings"
	"testing"
)

const (
	privB64 = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	pubB64  = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="
	pskB64  = "AgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgI="
)

const sample = `
[Interface]
PrivateKey = ` + privB64 + `
Address = 10.0.0.2/32, fd00::2/128
DNS = 1.1.1.1, 1.0.0.1
MTU = 1420

[Peer]
PublicKey = ` + pubB64 + `
PresharedKey = ` + pskB64 + `
Endpoint = 203.0.113.5:51820
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
`

func TestParseValid(t *testing.T) {
	c, err := Parse(sample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Interface.PrivateKey != privB64 {
		t.Errorf("private key mismatch")
	}
	if len(c.Interface.Address) != 2 {
		t.Fatalf("want 2 addresses, got %d", len(c.Interface.Address))
	}
	if !c.Interface.HasIPv6() {
		t.Errorf("expected HasIPv6 true")
	}
	if len(c.Interface.DNS) != 2 {
		t.Errorf("want 2 DNS, got %d", len(c.Interface.DNS))
	}
	if len(c.Peers) != 1 {
		t.Fatalf("want 1 peer, got %d", len(c.Peers))
	}
	p := c.Peers[0]
	if p.Endpoint != "203.0.113.5:51820" {
		t.Errorf("endpoint: %q", p.Endpoint)
	}
	if p.Keepalive != 25 {
		t.Errorf("keepalive: %d", p.Keepalive)
	}
	if len(p.AllowedIPs) != 2 {
		t.Errorf("want 2 allowedips, got %d", len(p.AllowedIPs))
	}
}

func TestEndpoint(t *testing.T) {
	c, _ := Parse(sample)
	h, port, err := c.Endpoint()
	if err != nil {
		t.Fatal(err)
	}
	if h != "203.0.113.5" || port != "51820" {
		t.Errorf("got %s:%s", h, port)
	}
}

func TestIPCSetHex(t *testing.T) {
	c, _ := Parse(sample)
	ipc, err := c.IPCSet()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(ipc, "private_key=0000000000000000000000000000000000000000000000000000000000000000") {
		t.Errorf("private_key not hex-encoded:\n%s", ipc)
	}
	if !strings.Contains(ipc, "endpoint=203.0.113.5:51820") {
		t.Errorf("missing endpoint")
	}
	if !strings.Contains(ipc, "persistent_keepalive_interval=25") {
		t.Errorf("missing keepalive")
	}
	if !strings.Contains(ipc, "allowed_ip=0.0.0.0/0") || !strings.Contains(ipc, "allowed_ip=::/0") {
		t.Errorf("missing allowed_ip")
	}

	if strings.Contains(ipc, privB64) || strings.Contains(ipc, pubB64) {
		t.Errorf("base64 key leaked into IPC string")
	}
}

func TestMasking(t *testing.T) {
	c, _ := Parse(sample)
	s := c.String()
	if strings.Contains(s, privB64) {
		t.Errorf("private key leaked in String(): %s", s)
	}
	if strings.Contains(s, pskB64) {
		t.Errorf("preshared key leaked in String(): %s", s)
	}
	if !strings.Contains(s, "<redacted>") {
		t.Errorf("expected redaction marker")
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"no private key": "[Interface]\nAddress = 10.0.0.2/32\n[Peer]\nPublicKey = " + pubB64 + "\nEndpoint = 1.2.3.4:1\n",
		"no address":     "[Interface]\nPrivateKey = " + privB64 + "\n[Peer]\nPublicKey = " + pubB64 + "\nEndpoint = 1.2.3.4:1\n",
		"no peers":       "[Interface]\nPrivateKey = " + privB64 + "\nAddress = 10.0.0.2/32\n",
		"bad key len":    "[Interface]\nPrivateKey = QUJD\nAddress = 10.0.0.2/32\n[Peer]\nPublicKey = " + pubB64 + "\nEndpoint = 1.2.3.4:1\n",
		"bad endpoint":   "[Interface]\nPrivateKey = " + privB64 + "\nAddress = 10.0.0.2/32\n[Peer]\nPublicKey = " + pubB64 + "\nEndpoint = noport\n",
		"unknown key":    "[Interface]\nPrivateKey = " + privB64 + "\nAddress = 10.0.0.2/32\nBogus = x\n[Peer]\nPublicKey = " + pubB64 + "\nEndpoint = 1.2.3.4:1\n",
		"key no section": "PrivateKey = " + privB64 + "\n",
	}
	for name, txt := range cases {
		if _, err := Parse(txt); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

func TestBareAddressBecomesHostPrefix(t *testing.T) {
	txt := "[Interface]\nPrivateKey = " + privB64 + "\nAddress = 10.0.0.9\n[Peer]\nPublicKey = " + pubB64 + "\nEndpoint = 1.2.3.4:1\nAllowedIPs = 10.0.0.0/24\n"
	c, err := Parse(txt)
	if err != nil {
		t.Fatal(err)
	}
	if c.Interface.Address[0].Bits() != 32 {
		t.Errorf("bare v4 address should become /32, got /%d", c.Interface.Address[0].Bits())
	}
}

func TestStringDoesNotMutate(t *testing.T) {
	c, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.String()
	_ = fmt.Sprintf("%v", c)
	if _, err := c.IPCSet(); err != nil {
		t.Fatalf("IPCSet after String: %v", err)
	}
}
