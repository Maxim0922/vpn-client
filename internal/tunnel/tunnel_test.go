package tunnel

import "testing"

func TestParseStats(t *testing.T) {
	uapi := `private_key=0000000000000000000000000000000000000000000000000000000000000000
public_key=1111111111111111111111111111111111111111111111111111111111111111
endpoint=203.0.113.5:51820
last_handshake_time_sec=1700000000
last_handshake_time_nsec=0
tx_bytes=2048
rx_bytes=4096
errno=0
`
	s := parseStats(uapi)
	if s.RxBytes != 4096 {
		t.Errorf("rx: %d", s.RxBytes)
	}
	if s.TxBytes != 2048 {
		t.Errorf("tx: %d", s.TxBytes)
	}
	if s.LastHandshake.IsZero() {
		t.Errorf("expected handshake time set")
	}
	if s.LastHandshake.Unix() != 1700000000 {
		t.Errorf("handshake: %v", s.LastHandshake)
	}
}

func TestParseStatsNoHandshake(t *testing.T) {
	uapi := "rx_bytes=0\ntx_bytes=0\nlast_handshake_time_sec=0\nerrno=0\n"
	s := parseStats(uapi)
	if !s.LastHandshake.IsZero() {
		t.Errorf("expected zero handshake, got %v", s.LastHandshake)
	}
}
