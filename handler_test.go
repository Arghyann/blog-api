package main

import (
	"testing"
	"time"
)

func TestTokenTTL(t *testing.T) {
	cases := map[int]time.Duration{
		-5:  time.Hour,
		0:   time.Hour,
		1:   24 * time.Hour,
		30:  30 * 24 * time.Hour,
		31:  30 * 24 * time.Hour,
		999: 30 * 24 * time.Hour,
	}
	for days, want := range cases {
		if got := tokenTTL(days); got != want {
			t.Errorf("tokenTTL(%d) = %v, want %v", days, got, want)
		}
	}
}
