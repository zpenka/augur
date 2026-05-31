package augur

import (
	"testing"
	"time"
)

func TestShort8(t *testing.T) {
	if short8("abcdefghij") != "abcdefgh" {
		t.Error("should truncate to 8 chars")
	}
	if short8("abc") != "abc" {
		t.Error("should not truncate short strings")
	}
	if short8("") != "" {
		t.Error("should handle empty string")
	}
	if short8("12345678") != "12345678" {
		t.Error("exactly 8 chars should be unchanged")
	}
}

func TestHumanTime_Ranges(t *testing.T) {
	now := time.Now()
	cases := []struct {
		t    time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5 minutes ago"},
		{now.Add(-3 * time.Hour), "3 hours ago"},
		{now.Add(-5 * 24 * time.Hour), "5 days ago"},
		{now.Add(-3 * 7 * 24 * time.Hour), "3 weeks ago"},
		{now.Add(-3 * 30 * 24 * time.Hour), "3 months ago"},
	}
	for _, c := range cases {
		got := humanTime(c.t)
		if got != c.want {
			t.Errorf("humanTime(%v ago) = %q, want %q", time.Since(c.t).Round(time.Second), got, c.want)
		}
	}
}
