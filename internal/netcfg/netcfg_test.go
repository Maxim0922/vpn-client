package netcfg

import (
	"context"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	xexec "github.com/max-tsx/max-vpn/internal/exec"
)

func mustPrefix(s string) netip.Prefix { return netip.MustParsePrefix(s) }
func mustAddr(s string) netip.Addr     { return netip.MustParseAddr(s) }

func TestAddressPlan(t *testing.T) {
	plan := AddressPlan("utun5", []netip.Prefix{mustPrefix("10.0.0.2/32"), mustPrefix("fd00::2/128")})
	got := lines(plan)
	want := []string{
		"ifconfig utun5 inet 10.0.0.2 10.0.0.2 alias",
		"ifconfig utun5 inet6 fd00::2 prefixlen 128 alias",
		"ifconfig utun5 up",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestFullTunnelPlan_V4Only_BlocksV6(t *testing.T) {
	plan, blocked := FullTunnelPlan("utun5", false)
	if !blocked {
		t.Errorf("expected ipv6 blocked when no v6")
	}
	got := strings.Join(lines(plan), "\n")
	for _, w := range []string{
		"route -n add -inet 0.0.0.0/1 -interface utun5",
		"route -n add -inet 128.0.0.0/1 -interface utun5",
		"route -n add -inet6 ::/1 ::1 -blackhole",
		"route -n add -inet6 8000::/1 ::1 -blackhole",
	} {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
}

func TestFullTunnelPlan_WithV6_RoutesV6(t *testing.T) {
	plan, blocked := FullTunnelPlan("utun5", true)
	if blocked {
		t.Errorf("should not block v6 when tunnel has v6")
	}
	got := strings.Join(lines(plan), "\n")
	if !strings.Contains(got, "route -n add -inet6 ::/1 -interface utun5") {
		t.Errorf("expected v6 routed via iface:\n%s", got)
	}
	if strings.Contains(got, "blackhole") {
		t.Errorf("should not blackhole when v6 present")
	}
}

func TestEndpointRoutePlan(t *testing.T) {
	got := lines(EndpointRoutePlan("203.0.113.5", "192.168.1.1"))
	want := []string{"route -n add -host 203.0.113.5 192.168.1.1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
}

func TestDNSPlan(t *testing.T) {
	got := lines(DNSPlan([]string{"Wi-Fi", "Ethernet"}, []netip.Addr{mustAddr("1.1.1.1"), mustAddr("1.0.0.1")}))
	want := []string{
		"networksetup -setdnsservers Wi-Fi 1.1.1.1 1.0.0.1",
		"networksetup -setdnsservers Ethernet 1.1.1.1 1.0.0.1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
}

func TestParseGateway(t *testing.T) {
	out := `   route to: default
destination: default
       mask: default
    gateway: 192.168.1.254
  interface: en0
`
	d := xexec.NewDryRunner()
	d.Outputs["route"] = [][]byte{[]byte(out)}
	gw, err := DefaultGatewayV4(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if gw != "192.168.1.254" {
		t.Errorf("got %q", gw)
	}
}

func TestActiveServicesSkipsDisabled(t *testing.T) {
	out := "An asterisk (*) denotes that a network service is disabled.\nWi-Fi\n*Bluetooth PAN\nEthernet\n"
	d := xexec.NewDryRunner()
	d.Outputs["networksetup"] = [][]byte{[]byte(out)}
	svcs, err := ActiveServices(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(svcs, []string{"Wi-Fi", "Ethernet"}) {
		t.Errorf("got %v", svcs)
	}
}

func TestCurrentDNS(t *testing.T) {
	d := xexec.NewDryRunner()
	d.Outputs["networksetup"] = [][]byte{
		[]byte("8.8.8.8\n8.8.4.4\n"),
		[]byte("There aren't any DNS Servers set on Ethernet.\n"),
	}
	m, err := CurrentDNS(context.Background(), d, []string{"Wi-Fi", "Ethernet"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m["Wi-Fi"], []string{"8.8.8.8", "8.8.4.4"}) {
		t.Errorf("wifi dns: %v", m["Wi-Fi"])
	}
	if len(m["Ethernet"]) != 0 {
		t.Errorf("ethernet should be empty (Empty), got %v", m["Ethernet"])
	}
}

func lines(plan [][]string) []string {
	out := make([]string, len(plan))
	for i, c := range plan {
		out[i] = strings.Join(c, " ")
	}
	return out
}
