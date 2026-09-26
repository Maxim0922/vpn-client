package state

import (
	"context"
	"strings"
	"testing"

	xexec "github.com/max-tsx/max-vpn/internal/exec"
)

func useTempDir(t *testing.T) {
	t.Helper()
	old := DefaultDir
	DefaultDir = t.TempDir()
	t.Cleanup(func() { DefaultDir = old })
}

func TestSaveLoadClear(t *testing.T) {
	useTempDir(t)

	if s, err := Load(); err != nil || s != nil {
		t.Fatalf("empty load: want nil,nil got %v,%v", s, err)
	}

	in := &State{
		Iface:                  "utun7",
		EndpointIP:             "203.0.113.5",
		EndpointRouteInstalled: true,
		OrigGWv4:               "192.168.1.1",
		OrigDNS:                map[string][]string{"Wi-Fi": {"1.1.1.1"}, "Ethernet": {}},
		DNSApplied:             []string{"Wi-Fi", "Ethernet"},
	}
	if err := Save(in); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Iface != "utun7" || got.EndpointIP != "203.0.113.5" || !got.EndpointRouteInstalled {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	if s, _ := Load(); s != nil {
		t.Errorf("expected nil after clear")
	}
	if err := Clear(); err != nil {
		t.Errorf("double clear should be nil, got %v", err)
	}
}

func TestTeardownFull(t *testing.T) {
	useTempDir(t)
	d := xexec.NewDryRunner()
	s := &State{
		Iface:                  "utun7",
		EndpointIP:             "203.0.113.5",
		EndpointRouteInstalled: true,
		OrigDNS:                map[string][]string{"Wi-Fi": {"1.1.1.1"}, "Ethernet": {}},
		DNSApplied:             []string{"Wi-Fi", "Ethernet"},
		PFToken:                "42",
		IPv6Blocked:            true,
	}
	_ = Save(s)
	if err := Teardown(context.Background(), d, s, nil); err != nil {
		t.Fatalf("teardown: %v", err)
	}
	lines := strings.Join(d.Lines(), "\n")
	for _, want := range []string{
		"pfctl -X 42",
		"pfctl -a com.apple/maxvpn -F all",
		"route -n delete -host 203.0.113.5",
		"route -n delete -inet6 ::/1",
		"networksetup -setdnsservers Wi-Fi 1.1.1.1",
		"networksetup -setdnsservers Ethernet Empty",
		"ifconfig utun7 destroy",
	} {
		if !strings.Contains(lines, want) {
			t.Errorf("missing step %q in:\n%s", want, lines)
		}
	}

	if got, _ := Load(); got != nil {
		t.Errorf("state not cleared after teardown")
	}
}

func TestTeardownPartial_NoPF_NoRoute(t *testing.T) {
	useTempDir(t)
	d := xexec.NewDryRunner()

	s := &State{
		OrigDNS:    map[string][]string{"Wi-Fi": {}},
		DNSApplied: []string{"Wi-Fi"},
	}
	if err := Teardown(context.Background(), d, s, nil); err != nil {
		t.Fatalf("teardown: %v", err)
	}
	lines := strings.Join(d.Lines(), "\n")
	if strings.Contains(lines, "pfctl") {
		t.Errorf("should not touch pf when no token")
	}
	if strings.Contains(lines, "route -n delete -host") {
		t.Errorf("should not delete endpoint route when not installed")
	}
	if !strings.Contains(lines, "networksetup -setdnsservers Wi-Fi Empty") {
		t.Errorf("expected DNS restore to Empty")
	}
}

func TestTeardownNil(t *testing.T) {
	if err := Teardown(context.Background(), xexec.NewDryRunner(), nil, nil); err != nil {
		t.Errorf("nil state teardown should be nil, got %v", err)
	}
}

func TestTeardownContinuesPastErrors(t *testing.T) {
	useTempDir(t)
	d := xexec.NewDryRunner()
	d.Errors["route"] = []error{errFake, errFake}
	s := &State{
		EndpointIP:             "203.0.113.5",
		EndpointRouteInstalled: true,
		IPv6Blocked:            true,
		OrigDNS:                map[string][]string{"Wi-Fi": {"9.9.9.9"}},
		DNSApplied:             []string{"Wi-Fi"},
	}
	_ = Save(s)
	err := Teardown(context.Background(), d, s, nil)
	if err == nil {
		t.Fatalf("expected joined error")
	}

	if !strings.Contains(strings.Join(d.Lines(), "\n"), "networksetup -setdnsservers Wi-Fi 9.9.9.9") {
		t.Errorf("DNS restore skipped after route error")
	}
}

var errFake = fakeErr("boom")

type fakeErr string

func (e fakeErr) Error() string { return string(e) }
