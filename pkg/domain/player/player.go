package player

import "gorm.io/gorm"

// type PlayerID uint64

type Player struct {
	gorm.Model
	// ID   PlayerID
	Name string
	// State PlayerState
}

// type PlayerState struct {
// 	Counter uint64
// 	Throws  []throws.Dart
// }
