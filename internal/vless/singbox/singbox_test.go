package singbox

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
	sjson "github.com/sagernet/sing/common/json"

	"github.com/max-tsx/max-vpn/internal/vless"
)

const uuid = "b831381d-6324-4d53-ad4f-8cda48b30811"

func TestBuildConfigParses(t *testing.T) {
	for _, link := range []string{
		"vless://" + uuid + "@example.com:443?security=reality&pbk=SbVKOEMjK0sIlbwg4akyBg5mL5KZwwB-ed4eEE7YnRc&sid=ab&flow=xtls-rprx-vision",
		"vless://" + uuid + "@example.com:443?security=tls&type=ws&path=%2Fws%3Fed%3D2048&host=a.b",
		"vless://" + uuid + "@example.com:443?security=tls&type=grpc&serviceName=svc&alpn=h2",
		"vless://" + uuid + "@example.com:80?type=httpupgrade&path=%2Fu",
	} {
		c, err := vless.Parse(link)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := buildConfig(c, vless.Options{Iface: "utun9", MTU: 1500, ServerIP: "1.2.3.4", DNS: netip.MustParseAddr("1.1.1.1")})
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(raw) {
			t.Fatal("invalid json")
		}
		if _, err := sjson.UnmarshalExtendedContext[option.Options](boxContext(context.Background()), raw); err != nil {
			t.Fatalf("%s: sing-box rejected config: %v", c.Transport, err)
		}
	}
}

func TestBoxStartsWithoutTun(t *testing.T) {
	c, err := vless.Parse("vless://" + uuid + "@example.com:443?security=tls&type=ws&path=%2Fws")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := buildConfig(c, vless.Options{Iface: "utun9", MTU: 1500, ServerIP: "192.0.2.1", DNS: netip.MustParseAddr("1.1.1.1")})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	delete(m, "inbounds")
	raw, _ = json.Marshal(m)

	ctx, cancel := context.WithCancel(boxContext(context.Background()))
	defer cancel()
	opts, err := sjson.UnmarshalExtendedContext[option.Options](ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	b, err := box.New(box.Options{Context: ctx, Options: opts})
	if err != nil {
		t.Fatalf("box.New: %v", err)
	}
	if err := b.Start(); err != nil {
		t.Fatalf("box.Start: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("box.Close: %v", err)
	}
}
