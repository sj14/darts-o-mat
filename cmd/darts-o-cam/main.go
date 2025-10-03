package main

import (
	"log"

	"github.com/sj14/darts-o-mat/pkg/domain/board"
)

func main() {
	b := board.Board{ID: 1, Token: "asd"}
	log.Println(b)
	for {
		// dart := b.CheckImage()
		// err := dart.Send(b.Server, b.Token)
	}
}
