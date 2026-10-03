package daemon

import (
	"testing"

	"github.com/max-tsx/max-vpn/internal/state"
)

func TestParseNetstatBytes(t *testing.T) {
	out := []byte(`Name       Mtu   Network       Address            Ipkts Ierrs     Ibytes    Opkts Oerrs     Obytes  Coll
utun7      1500  <Link#20>                          120     0      45678       80     0       9012     0
utun7      1500  172.19.0/30   172.19.0.1           120     -      45678       80     -       9012     -
`)
	rx, tx, err := parseNetstatBytes(out, "utun7")
	if err != nil {
		t.Fatal(err)
	}
	if rx != 45678 || tx != 9012 {
		t.Fatalf("rx=%d tx=%d", rx, tx)
	}
	if _, _, err := parseNetstatBytes(out, "utun8"); err == nil {
		t.Fatal("expected error for missing iface")
	}
}

func TestValidID(t *testing.T) {
	for _, ok := range []string{"fra1", "DE Frankfurt", "de-fra_1.2"} {
		if err := validID(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", ".", "..", "../etc", "a/b", `a\b`, ".hidden", "a\x00b"} {
		if validID(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestImportAndListOpenVPN(t *testing.T) {
	old := state.DefaultDir
	state.DefaultDir = t.TempDir()
	defer func() { state.DefaultDir = old }()

	m := NewManager(nil, nil)
	ovpnText := `
client
dev tun
proto udp
remote 198.51.100.1 1194
`
	if err := m.ImportConfig("ovpn1", ovpnText); err != nil {
		t.Fatalf("import openvpn: %v", err)
	}

	servers, err := m.ListServers()
	if err != nil {
		t.Fatalf("list servers: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].ID != "ovpn1" || servers[0].Protocol != ProtoOpenVPN {
		t.Errorf("unexpected server info: %+v", servers[0])
	}

	srv, err := m.loadServer("ovpn1")
	if err != nil {
		t.Fatalf("loadServer: %v", err)
	}
	if srv.ov == nil {
		t.Fatalf("expected srv.ov to be non-nil")
	}
	host, port, proto, err := srv.endpoint()
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	if host != "198.51.100.1" || port != "1194" || proto != "udp" {
		t.Errorf("got %s:%s %s", host, port, proto)
	}

	if err := m.RemoveServer("ovpn1"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	servers, err = m.ListServers()
	if err != nil {
		t.Fatalf("list servers after remove: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
}

func TestRemoveServerCleansSettings(t *testing.T) {
	old := state.DefaultDir
	state.DefaultDir = t.TempDir()
	defer func() { state.DefaultDir = old }()

	m := NewManager(nil, nil)
	vlessLink := "vless://b831381d-6324-4d53-ad4f-8cda48b30811@example.com:443#Frankfurt"
	if err := m.ImportConfig("Frankfurt", vlessLink); err != nil {
		t.Fatalf("import vless: %v", err)
	}

	_ = m.SetSettings(Settings{
		LastServer: "Frankfurt",
		Favorites:  []string{"Frankfurt", "other"},
	})

	if err := m.RemoveServer("Frankfurt"); err != nil {
		t.Fatalf("remove server: %v", err)
	}

	st := m.GetSettings()
	if st.LastServer != "" {
		t.Errorf("expected LastServer to be cleared, got %q", st.LastServer)
	}
	if len(st.Favorites) != 1 || st.Favorites[0] != "other" {
		t.Errorf("expected Favorites to only contain 'other', got %v", st.Favorites)
	}
}

