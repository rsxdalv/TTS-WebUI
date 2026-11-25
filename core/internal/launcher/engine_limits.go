package launcher

import (
	"strconv"
	"strings"
	"time"
)

// Engine limits (D31). Idle stop is off unless TTS_ENGINE_IDLE_MINUTES is a
// positive number, because most people want an engine to stay loaded.
// TTS_ENGINE_MAX_RUNNING caps the engines that run at once; unset it is 4, and
// 0 means no cap. A start over the cap stops the engine used longest ago that
// is not busy; a busy engine is never stopped.
const defaultMaxRunningEngines = 4

func engineIdleTimeout(getenv func(string) string) time.Duration {
	n, err := strconv.Atoi(strings.TrimSpace(getenv("TTS_ENGINE_IDLE_MINUTES")))
	if err != nil || n <= 0 {
		return 0
	}
	return time.Duration(n) * time.Minute
}

func engineMaxRunning(getenv func(string) string) int {
	v := strings.TrimSpace(getenv("TTS_ENGINE_MAX_RUNNING"))
	if v == "" {
		return defaultMaxRunningEngines
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return defaultMaxRunningEngines
	}
	return n
}
