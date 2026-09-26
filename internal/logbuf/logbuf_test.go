package logbuf

import (
	"testing"
	"time"
)

func TestRingOverwrite(t *testing.T) {
	r := New(3)
	for i := 0; i < 5; i++ {
		r.Add("info", string(rune('a'+i)))
	}
	all := r.All()
	if len(all) != 3 {
		t.Fatalf("want 3 retained, got %d", len(all))
	}

	if all[0].Msg != "c" || all[2].Msg != "e" {
		t.Errorf("wrong retention order: %v", []string{all[0].Msg, all[1].Msg, all[2].Msg})
	}
}

func TestRingSince(t *testing.T) {
	r := New(10)
	r.Add("info", "old")
	time.Sleep(5 * time.Millisecond)
	cut := time.Now()
	time.Sleep(5 * time.Millisecond)
	r.Add("info", "new")
	got := r.Since(cut)
	if len(got) != 1 || got[0].Msg != "new" {
		t.Errorf("since filter wrong: %+v", got)
	}
}

func TestRingNotFull(t *testing.T) {
	r := New(5)
	r.Add("info", "a")
	r.Add("warn", "b")
	all := r.All()
	if len(all) != 2 || all[0].Msg != "a" || all[1].Msg != "b" {
		t.Errorf("got %+v", all)
	}
}
