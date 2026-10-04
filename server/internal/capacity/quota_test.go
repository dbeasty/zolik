package capacity

import "testing"

func TestCPUMaxParsing(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want float64
		ok   bool
	}{
		{"100000 100000\n", 1, true},
		{"150000 100000", 1.5, true},
		{"50000 100000", 0.5, true},
		{"max 100000", 0, false},
		{"garbage", 0, false},
	} {
		got, ok := parseCPUMax(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseCPUMax(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
	if _, ok := parseQuotaPeriod("-1", "100000"); ok {
		t.Error("cgroup v1's -1 is no limit")
	}
}

func TestCPUQuotaIsPositive(t *testing.T) {
	if q := CPUQuota(); q <= 0 {
		t.Fatalf("CPUQuota = %v", q)
	}
}
