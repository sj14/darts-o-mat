package player

type Player struct {
	ID   int64  `bun:",pk,autoincrement" json:"id"`
	Name string `bun:",notnull" json:"name"`
}
