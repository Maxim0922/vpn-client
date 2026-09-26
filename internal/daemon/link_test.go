package daemon

import "testing"

func TestParseNetstatBytes(t *testing.T) {
	out := []byte(`Name       Mtu   Network       Address            Ipkts Ierrs     Ibytes    Opkts Oerrs     Obytes  Coll
utun7      1500  <Link#20>                          120     0      45678       80     0       9012     0
utun7      1500  172.19.0/30   172.19.0.1           120     -      45678       80     -       9012     -
`)
	rx, tx, err := parseNetstatBytes(out, "utun7")
	if err != nil {
		t.Fatal(err)
	}
	if rx != 45678 || tx != 9012 {
		t.Fatalf("rx=%d tx=%d", rx, tx)
	}
	if _, _, err := parseNetstatBytes(out, "utun8"); err == nil {
		t.Fatal("expected error for missing iface")
	}
}

func TestValidID(t *testing.T) {
	for _, ok := range []string{"fra1", "DE Frankfurt", "de-fra_1.2"} {
		if err := validID(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", ".", "..", "../etc", "a/b", `a\b`, ".hidden", "a\x00b"} {
		if validID(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
