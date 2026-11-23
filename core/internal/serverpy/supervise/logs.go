package supervise

// SubscribeLogs streams the live output of the named child (NameGradio,
// "react-ui") from the current generation. It returns
// ok=false when that child is not running or its runner cannot stream. The
// cancel func must be called when done. A Restart starts new runners, so
// subscribe again after one.
func (s *Supervisor) SubscribeLogs(name string) (lines <-chan string, cancel func(), ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.cur {
		if m.name != name || m.r == nil {
			continue
		}
		sub, can := m.r.(interface {
			Subscribe() (<-chan string, func())
		})
		if !can {
			return nil, nil, false
		}
		ch, c := sub.Subscribe()
		return ch, c, true
	}
	return nil, nil, false
}
