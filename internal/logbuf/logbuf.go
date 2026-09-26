package logbuf

import (
	"sync"
	"time"
)

type Entry struct {
	TS    time.Time `json:"ts"`
	Level string    `json:"level"`
	Msg   string    `json:"msg"`
}

type Ring struct {
	mu   sync.Mutex
	buf  []Entry
	next int
	full bool
}

func New(size int) *Ring {
	if size <= 0 {
		size = 1000
	}
	return &Ring{buf: make([]Entry, size)}
}

func (r *Ring) Add(level, msg string) {
	r.mu.Lock()
	r.buf[r.next] = Entry{TS: time.Now(), Level: level, Msg: msg}
	r.next = (r.next + 1) % len(r.buf)
	if r.next == 0 {
		r.full = true
	}
	r.mu.Unlock()
}

func (r *Ring) Since(t time.Time) []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Entry
	n := len(r.buf)
	start := 0
	count := r.next
	if r.full {
		start = r.next
		count = n
	}
	for i := 0; i < count; i++ {
		e := r.buf[(start+i)%n]
		if e.TS.After(t) {
			out = append(out, e)
		}
	}
	return out
}

func (r *Ring) All() []Entry { return r.Since(time.Time{}) }
