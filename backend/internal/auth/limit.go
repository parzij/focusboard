package auth

import (
	"net"
	"net/netip"
	"sync"
	"time"
)

type limiter struct {
	mu      sync.Mutex
	counts  map[string]int
	reset   time.Time
	window  time.Duration
	maxKeys int
	now     func() time.Time
}

func newLimiter(window time.Duration, maxKeys int) *limiter {
	return &limiter{counts: make(map[string]int), window: window, maxKeys: maxKeys, now: time.Now}
}

func (l *limiter) allow(key string, limit int) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if !now.Before(l.reset) {
		clear(l.counts)
		l.reset = now.Add(l.window)
	}
	count, exists := l.counts[key]
	if count >= limit || !exists && len(l.counts) >= l.maxKeys {
		return false, l.reset.Sub(now)
	}
	l.counts[key] = count + 1
	return true, 0
}

func peerIP(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return "unknown"
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	address = address.Unmap().WithZone("")
	if address.Is6() {
		return netip.PrefixFrom(address, 64).Masked().String()
	}
	return address.String()
}
