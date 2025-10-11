package throw

type Dart struct {
	// gorm.Model
	ID            uint64
	GameID        int
	Number        uint8
	Multiplicator uint8
	Miss          bool
}

func (t *Dart) Value() (uint8, bool) {
	if t.Multiplicator == 0 {
		t.Multiplicator = 1
	}
	return t.Number * t.Multiplicator, t.Miss
}

// func (t *Dart) Valid() bool {
// 	return !t.Miss
// }
