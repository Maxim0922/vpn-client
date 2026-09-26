package stats

import (
	"testing"
	"time"
)

func TestRateCalc(t *testing.T) {
	var r RateCalc
	t0 := time.Unix(1000, 0)

	rx, tx := r.Update(1000, 500, t0)
	if rx != 0 || tx != 0 {
		t.Errorf("first sample should be 0,0 got %d,%d", rx, tx)
	}

	rx, tx = r.Update(3000, 1500, t0.Add(time.Second))
	if rx != 2000 || tx != 1000 {
		t.Errorf("got %d,%d want 2000,1000", rx, tx)
	}
}

func TestRateCalcCounterReset(t *testing.T) {
	var r RateCalc
	t0 := time.Unix(0, 0)
	r.Update(5000, 5000, t0)

	rx, tx := r.Update(100, 100, t0.Add(time.Second))
	if rx != 0 || tx != 0 {
		t.Errorf("counter reset should give 0,0 got %d,%d", rx, tx)
	}
}

func TestParsePing(t *testing.T) {
	out := `PING 1.2.3.4 (1.2.3.4): 56 data bytes
64 bytes from 1.2.3.4: icmp_seq=0 ttl=54 time=12.3 ms

--- 1.2.3.4 ping statistics ---`
	if got := parsePing(out); got != 12 {
		t.Errorf("got %d want 12", got)
	}
}

func TestParsePingFail(t *testing.T) {
	if got := parsePing("Request timeout for icmp_seq 0\n"); got != -1 {
		t.Errorf("got %d want -1", got)
	}
}
