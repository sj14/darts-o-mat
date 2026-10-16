package service

// Subscribe returns a channel that receives a value whenever the game changes, e.g. to
// push the new board to open game pages, and a function to stop the subscription.
// Changes that happen while the receiver is busy are merged into one notification.
func (s *Service) Subscribe(gameID int64) (<-chan struct{}, func()) {
	return s.games.Subscribe(gameID)
}

// notify tells the game's subscribers that it changed.
func (s *Service) notify(gameID int64) {
	s.games.Notify(gameID)
}
