package player

import (
	"slices"
	"testing"
)

func TestAccent(t *testing.T) {
	t.Parallel()
	p := Player{Name: "Anna"}
	if got := p.Accent(); !slices.Contains(Colors, got) {
		t.Fatalf("accent of %q = %q, want one of the colors", p.Name, got)
	}
	if got, want := (Player{Name: "ANNA"}).Accent(), p.Accent(); got != want {
		t.Errorf("accent depends on the case of the name: %q vs %q", got, want)
	}
	if got := (Player{Name: "Anna", Color: "pink"}).Accent(); got != "pink" {
		t.Errorf("chosen color: got %q, want pink", got)
	}
	if got := (Player{Name: "Anna", Color: "nope"}).Accent(); got != p.Accent() {
		t.Errorf("unknown color: got %q, want the default %q", got, p.Accent())
	}
	seen := map[string]bool{}
	for _, name := range []string{"Anna", "Ben", "Carla", "David", "Eva", "Felix", "Gina", "Hans"} {
		seen[Player{Name: name}.Accent()] = true
	}
	if len(seen) < 3 {
		t.Errorf("8 names got only %d different colors", len(seen))
	}
}
