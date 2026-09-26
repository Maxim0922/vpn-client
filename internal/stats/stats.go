package stats

import (
	"context"
	"sync"
	"time"

	xexec "github.com/max-tsx/max-vpn/internal/exec"
)

type Snapshot struct {
	RxBytes       uint64 `json:"rxBytes"`
	TxBytes       uint64 `json:"txBytes"`
	RxRate        uint64 `json:"rxRate"`
	TxRate        uint64 `json:"txRate"`
	LastHandshake int64  `json:"lastHandshake"`
	LatencyMs     int64  `json:"latencyMs"`
}

type RateCalc struct {
	mu       sync.Mutex
	lastRx   uint64
	lastTx   uint64
	lastTime time.Time
}

func (r *RateCalc) Update(rx, tx uint64, now time.Time) (rxRate, txRate uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.lastTime.IsZero() {
		dt := now.Sub(r.lastTime).Seconds()
		if dt > 0 {
			if rx >= r.lastRx {
				rxRate = uint64(float64(rx-r.lastRx) / dt)
			}
			if tx >= r.lastTx {
				txRate = uint64(float64(tx-r.lastTx) / dt)
			}
		}
	}
	r.lastRx, r.lastTx, r.lastTime = rx, tx, now
	return rxRate, txRate
}

func (r *RateCalc) Reset() {
	r.mu.Lock()
	r.lastRx, r.lastTx, r.lastTime = 0, 0, time.Time{}
	r.mu.Unlock()
}

func PingLatency(ctx context.Context, run xexec.Runner, host string) int64 {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := run.Run(ctx, "ping", "-c", "1", "-t", "2", host)
	if err != nil {
		return -1
	}
	return parsePing(string(out))
}
