package game

import (
	"encoding/json"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
)

func TestRulesMustFitTheMode(t *testing.T) {
	t.Parallel()
	ok := Settings{Mode: ModeX01, Legs: 1, Sets: 1, Rules: &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]Settings{
		"rules of another mode": {Mode: ModeX01, Legs: 1, Sets: 1, Rules: &aroundtheclock.Settings{}},
		"no rules":              {Mode: ModeX01, Legs: 1, Sets: 1},
		"unknown mode":          {Mode: "darts-bowling", Legs: 1, Sets: 1, Rules: &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble}},
		"invalid rules":         {Mode: ModeX01, Legs: 1, Sets: 1, Rules: &x01.Settings{}},
	} {
		if err := s.Validate(); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// The rules are written as "rules", and read into the type of the mode once it is prepared.
func TestRulesJSON(t *testing.T) {
	t.Parallel()
	out, err := json.Marshal(Settings{Mode: ModeAroundTheClock, Rules: &aroundtheclock.Settings{Bull: true}})
	if err != nil {
		t.Fatal(err)
	}
	var s Settings
	s.Rules = NewRules(ModeAroundTheClock)
	if err := json.Unmarshal(out, &s); err != nil {
		t.Fatal(err)
	}
	if r, ok := s.Rules.(*aroundtheclock.Settings); !ok || !r.Bull {
		t.Errorf("rules read back = %#v from %s", s.Rules, out)
	}
	if NewRules("nope") != nil {
		t.Error("rules of a mode that doesn't exist")
	}
}
