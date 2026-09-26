package stats

import (
	"regexp"
	"strconv"
)

var pingRe = regexp.MustCompile(`time=([0-9.]+)\s*ms`)

func parsePing(out string) int64 {
	m := pingRe.FindStringSubmatch(out)
	if m == nil {
		return -1
	}
	f, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return -1
	}
	return int64(f + 0.5)
}
