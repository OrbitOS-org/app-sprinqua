package config

import "testing"

func TestZoneEffectivePulseSecsDefault(t *testing.T) {
	z := Zone{MaxSecs: 1800}
	if got := z.EffectivePulseSecs(); got != DefaultPulseSecs {
		t.Fatalf("expected default %d, got %d", DefaultPulseSecs, got)
	}
}

func TestZoneEffectivePulseSecsCappedAtMax(t *testing.T) {
	z := Zone{PulseSecs: 600, MaxSecs: 300}
	if got := z.EffectivePulseSecs(); got != 300 {
		t.Fatalf("expected cap at max 300, got %d", got)
	}
}

func TestZoneEffectivePulseSecsCustom(t *testing.T) {
	z := Zone{PulseSecs: 600, MaxSecs: 1800}
	if got := z.EffectivePulseSecs(); got != 600 {
		t.Fatalf("expected 600, got %d", got)
	}
}
