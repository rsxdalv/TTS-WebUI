package extprocs

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrCapReached means MaxRunning engines are running and every one is busy.
var ErrCapReached = errors.New("too many engines are running and all of them are busy")

// Activity marks one request to the engine behind the route prefix ("/<pkg>")
// as started and returns the function that marks it finished. The router calls
// it for every proxied request, so an engine that is generating (its request
// stays open) is never idle.
func (m *Manager) Activity(prefix string) func() {
	pkg := strings.TrimPrefix(prefix, "/")
	m.mu.Lock()
	e := m.ents[pkg]
	if e == nil || e.dead {
		m.mu.Unlock()
		return func() {}
	}
	e.inflight++
	e.lastUsed = time.Now()
	m.mu.Unlock()
	return func() {
		m.mu.Lock()
		if e.inflight > 0 {
			e.inflight--
		}
		e.lastUsed = time.Now()
		m.mu.Unlock()
	}
}

// makeRoomLocked is called by Start with m.mu held and pkg not running. When
// the cap is reached it stops the least recently used engine that is not
// busy. m.mu is released while the victim stops.
func (m *Manager) makeRoomLocked(pkg string) error {
	if m.opts.MaxRunning <= 0 {
		return nil
	}
	for {
		running := 0
		var victim *entry
		for _, e := range m.ents {
			if e.dead || e.stopping {
				continue
			}
			running++
			if e.inflight == 0 && (victim == nil || e.lastUsed.Before(victim.lastUsed)) {
				victim = e
			}
		}
		if running < m.opts.MaxRunning {
			return nil
		}
		if victim == nil {
			return fmt.Errorf("%w (cap %d); starting %s needs one to finish", ErrCapReached, m.opts.MaxRunning, pkg)
		}
		m.logf("extprocs: cap %d reached, stopping %s (used longest ago) to start %s", m.opts.MaxRunning, victim.pkg, pkg)
		m.mu.Unlock()
		err := m.Stop(victim.pkg)
		m.mu.Lock()
		if err != nil && !victim.dead {
			return fmt.Errorf("stopping %s to make room: %w", victim.pkg, err)
		}
	}
}

// reapIdle stops ready engines that are past IdleTimeout and not busy. It
// returns the packages it stopped.
func (m *Manager) reapIdle(now time.Time) []string {
	if m.opts.IdleTimeout <= 0 {
		return nil
	}
	m.mu.Lock()
	var stale []string
	for _, e := range m.ents {
		if !e.dead && !e.stopping && e.state == StateReady && e.inflight == 0 && now.Sub(e.lastUsed) >= m.opts.IdleTimeout {
			stale = append(stale, e.pkg)
		}
	}
	m.mu.Unlock()
	for _, p := range stale {
		m.logf("extprocs: stopping %s after %s without use", p, m.opts.IdleTimeout)
		_ = m.Stop(p)
	}
	return stale
}

// RunIdleStop checks every interval until stop is closed. It does nothing when
// IdleTimeout is zero.
func (m *Manager) RunIdleStop(interval time.Duration, stop <-chan struct{}) {
	if m.opts.IdleTimeout <= 0 {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-t.C:
			m.reapIdle(now)
		}
	}
}
