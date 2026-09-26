package vless

import (
	"strings"
	"testing"
)

const uuid = "b831381d-6324-4d53-ad4f-8cda48b30811"

func TestParseReality(t *testing.T) {
	c, err := Parse("vless://" + uuid + "@example.com:443?encryption=none&flow=xtls-rprx-vision&security=reality&sni=www.microsoft.com&fp=chrome&pbk=SbVKOEMjK0sIlbwg4akyBg5mL5KZwwB-ed4eEE7YnRc&sid=6ba85179e30d4fc2&type=tcp#Frankfurt")
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "Frankfurt" || c.Host != "example.com" || c.Port != 443 || c.Flow != "xtls-rprx-vision" {
		t.Fatalf("basic fields: %+v", c)
	}
	if c.Security != "reality" || c.SNI != "www.microsoft.com" || c.PublicKey == "" || c.ShortID != "6ba85179e30d4fc2" || c.Transport != "tcp" {
		t.Fatalf("reality fields: %+v", c)
	}
}

func TestParseWSEarlyData(t *testing.T) {
	c, err := Parse("vless://" + uuid + "@1.2.3.4:8443?security=tls&type=ws&host=cdn.example.com&path=%2Fws%3Fed%3D2048")
	if err != nil {
		t.Fatal(err)
	}
	if c.Transport != "ws" || c.Path != "/ws" || c.EarlyData != 2048 || c.HostHeader != "cdn.example.com" {
		t.Fatalf("ws fields: %+v", c)
	}
}

func TestParseErrors(t *testing.T) {
	for name, link := range map[string]string{
		"scheme":      "vmess://" + uuid + "@h:1",
		"uuid":        "vless://nope@h:443",
		"port":        "vless://" + uuid + "@h",
		"encryption":  "vless://" + uuid + "@h:443?encryption=aes",
		"reality pbk": "vless://" + uuid + "@h:443?security=reality",
		"xhttp":       "vless://" + uuid + "@h:443?type=xhttp",
	} {
		if _, err := Parse(link); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestStringHidesUUID(t *testing.T) {
	c, _ := Parse("vless://" + uuid + "@h:443")
	if strings.Contains(c.String(), uuid) {
		t.Fatal("uuid leaked into String()")
	}
}
