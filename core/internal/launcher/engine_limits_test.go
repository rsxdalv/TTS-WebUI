package launcher

import (
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestEngineLimitsDefaultsAreLenient(t *testing.T) {
	if got := engineIdleTimeout(env(nil)); got != 0 {
		t.Fatalf("idle stop on by default: %v", got)
	}
	for _, v := range []string{"", "x", "-5", "0"} {
		if got := engineIdleTimeout(env(map[string]string{"TTS_ENGINE_IDLE_MINUTES": v})); got != 0 {
			t.Fatalf("idle %q: %v, want off", v, got)
		}
	}
	if got := engineIdleTimeout(env(map[string]string{"TTS_ENGINE_IDLE_MINUTES": "30"})); got != 30*time.Minute {
		t.Fatalf("idle 30: %v", got)
	}
	cases := map[string]int{"": 4, "junk": 4, "-1": 4, "0": 0, "2": 2}
	for v, want := range cases {
		if got := engineMaxRunning(env(map[string]string{"TTS_ENGINE_MAX_RUNNING": v})); got != want {
			t.Fatalf("max %q: %d, want %d", v, got, want)
		}
	}
}
