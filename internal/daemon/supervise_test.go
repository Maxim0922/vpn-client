package daemon

import (
	"context"
	"strings"
	"testing"
	"time"

	xexec "github.com/max-tsx/max-vpn/internal/exec"
	"github.com/max-tsx/max-vpn/internal/state"
)

func TestBackoffAt(t *testing.T) {
	if backoffAt(0) != time.Second {
		t.Errorf("attempt 0: %v", backoffAt(0))
	}
	if backoffAt(-5) != time.Second {
		t.Errorf("negative should clamp to first")
	}
	last := backoffSchedule[len(backoffSchedule)-1]
	if backoffAt(100) != last {
		t.Errorf("overflow should return last (%v), got %v", last, backoffAt(100))
	}
}

func TestOnNetworkChangeRepins(t *testing.T) {
	old := state.DefaultDir
	state.DefaultDir = t.TempDir()
	defer func() { state.DefaultDir = old }()

	d := xexec.NewDryRunner()

	d.Outputs["route"] = [][]byte{[]byte("    gateway: 10.9.9.1\n")}

	m := NewManager(d, nil)

	m.mu.Lock()
	m.status = Status{State: StateConnected, Server: "s1"}
	m.endpointIP = "203.0.113.5"
	m.endpointGW = "192.168.1.1"
	m.mu.Unlock()
	_ = state.Save(&state.State{EndpointIP: "203.0.113.5", EndpointRouteInstalled: true})

	m.OnNetworkChange(context.Background())

	lines := strings.Join(d.Lines(), "\n")
	if !strings.Contains(lines, "route -n delete -host 203.0.113.5") {
		t.Errorf("expected stale route delete:\n%s", lines)
	}
	if !strings.Contains(lines, "route -n add -host 203.0.113.5 10.9.9.1") {
		t.Errorf("expected re-pin via new gw:\n%s", lines)
	}
	if m.endpointGW != "10.9.9.1" {
		t.Errorf("endpointGW not updated: %s", m.endpointGW)
	}
}

func TestOnNetworkChangeNoopWhenDisconnected(t *testing.T) {
	d := xexec.NewDryRunner()
	m := NewManager(d, nil)
	m.OnNetworkChange(context.Background())
	if len(d.Lines()) != 0 {
		t.Errorf("should do nothing when disconnected, ran: %v", d.Lines())
	}
}

func TestOnNetworkChangeSameGWNoop(t *testing.T) {
	d := xexec.NewDryRunner()
	d.Outputs["route"] = [][]byte{[]byte("    gateway: 192.168.1.1\n")}
	m := NewManager(d, nil)
	m.mu.Lock()
	m.status = Status{State: StateConnected}
	m.endpointIP = "203.0.113.5"
	m.endpointGW = "192.168.1.1"
	m.mu.Unlock()
	m.OnNetworkChange(context.Background())
	for _, l := range d.Lines() {
		if strings.Contains(l, "delete") || strings.Contains(l, "add") {
			t.Errorf("no route change expected when gw unchanged: %s", l)
		}
	}
}
