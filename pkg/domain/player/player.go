package player

// type PlayerID uint64

type Player struct {
	// gorm.Model
	ID   uint64
	Name string
	// State PlayerState
}

// type PlayerState struct {
// 	Counter uint64
// 	Throws  []throws.Dart
// }
