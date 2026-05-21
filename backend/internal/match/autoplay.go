package match

func (s *Service) runAutoplay(id string) {
	defer s.finishAutoplay(id)

	snapshot, ok := s.GetMatch(id)
	if !ok {
		return
	}

	if _, err := s.runUntilPause(id, snapshot); err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		if current, ok := s.matches[id]; ok {
			current.Warning = "纯 AI 自动推进失败：" + err.Error()
			s.matches[id] = current
		}
	}
}
