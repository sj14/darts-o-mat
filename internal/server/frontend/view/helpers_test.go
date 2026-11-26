package view

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/common/i18n"
	"github.com/sj14/darts-o-mat/internal/server/domain/board"
	"github.com/sj14/darts-o-mat/internal/server/domain/bot"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/cricket"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

func TestDefaultBoard(t *testing.T) {
	t.Parallel()
	view := func(id int64, used time.Duration) board.Board {
		b := board.Board{ID: id}
		if used != 0 {
			at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(used)
			b.LastUsedAt = &at
		}
		return b
	}
	tests := []struct {
		name   string
		boards []board.Board
		want   int64
	}{
		{"no boards", nil, 0},
		{"never used: the first", []board.Board{view(3, 0), view(5, 0)}, 3},
		{"the last used", []board.Board{view(3, time.Hour), view(5, 2*time.Hour), view(7, 0)}, 5},
		{"a used one over unused ones before it", []board.Board{view(3, 0), view(5, time.Hour)}, 5},
	}
	for _, tt := range tests {
		if got := defaultBoard(tt.boards); got != tt.want {
			t.Errorf("%s: defaultBoard = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestHandEntry(t *testing.T) {
	t.Parallel()
	board1, board2 := int64(1), int64(2)
	const anna, ben, guest = int64(10), int64(20), int64(30)
	progress := func(current int64) progress {
		return progress{State: turn.State{CurrentPlayerID: &current}}
	}
	players := func(boards ...*int64) service.GameView {
		ids := []int64{anna, guest, ben}
		v := service.GameView{}
		for i, b := range boards {
			v.Game.Participants = append(v.Game.Participants, game.Participant{PlayerID: ids[i], Status: game.InvitationAccepted, BoardID: b})
		}
		return v
	}
	bot := func(v service.GameView) service.GameView { // the guest is a bot
		v.Game.Participants[1].BotSpreadUM = 20_000
		return v
	}
	with := func(v service.GameView, mine ...int64) service.GameView {
		v.Mine = mine
		return v
	}

	tests := []struct {
		name      string
		v         service.GameView
		current   int64
		hand, add bool
	}{
		{"no board, my turn", with(players(nil, nil, nil), anna, guest), guest, true, false},
		{"no board, a friend's turn", with(players(nil, nil, nil), anna, guest), ben, false, false},
		{"at a board, my turn", with(players(&board1, &board1, nil), anna, guest), guest, false, true},
		{"on my own board", with(players(nil, nil, &board2), ben), ben, false, true},
		{"joined without a board next to a host with one", with(players(&board1, &board1, nil), ben), ben, true, false},
		{"watching a friend who has a board", with(players(nil, nil, &board2), anna, guest), ben, false, false},
		{"my bot's turn without boards", with(bot(players(nil, nil, nil)), anna, guest), guest, true, false},
		{"my bot's turn next to my board", with(bot(players(&board1, nil, nil)), anna, guest), guest, false, false},
	}
	for _, tt := range tests {
		tt.v.CanThrow = true
		p := progress(tt.current)
		if got := handEntry(tt.v, p); got != tt.hand {
			t.Errorf("%s: handEntry = %v, want %v", tt.name, got, tt.hand)
		}
		if got := mayAddDart(tt.v, p); got != tt.add {
			t.Errorf("%s: mayAddDart = %v, want %v", tt.name, got, tt.add)
		}
	}

	// An account whose player was skipped out doesn't enter darts for its bot.
	left := with(bot(players(nil, nil, nil)), anna, guest)
	if handEntry(left, progress(guest)) {
		t.Error("keypad for the bot of an account that can't throw anymore")
	}
}

func TestBoardAction(t *testing.T) {
	t.Parallel()
	board := int64(1)
	const anna, ben, robo = int64(10), int64(20), int64(30)
	v := service.GameView{CanThrow: true, Mine: []int64{anna, robo}}
	v.Game.Participants = []game.Participant{
		{PlayerID: anna, Status: game.InvitationAccepted},
		{PlayerID: ben, Status: game.InvitationAccepted, BoardID: &board},
		{PlayerID: robo, Status: game.InvitationAccepted, BotSpreadUM: 20_000},
	}
	withCamera := v
	withCamera.Mine = []int64{ben}
	turnOf := func(current int64) progress {
		return progress{State: turn.State{CurrentPlayerID: &current}}
	}
	bullOff := turnOf(anna)
	bullOff.BullingOff = true
	takeout := turnOf(anna)
	takeout.WaitingForTakeout = true

	watching := v
	watching.CanThrow = false
	const enter, correct = "/games/0/darts", "/games/0/darts/7"
	dart := &throw.Dart{ID: 7}

	tests := []struct {
		name    string
		v       service.GameView
		p       progress
		editing *throw.Dart
		want    string
	}{
		{"my turn by hand", v, turnOf(anna), nil, enter},
		{"my turn with cameras", withCamera, turnOf(ben), nil, enter},
		{"another account's turn", v, turnOf(ben), nil, ""},
		{"my bot's turn", v, turnOf(robo), nil, ""},
		{"bull-off", v, bullOff, nil, ""},
		{"takeout pending", v, takeout, nil, ""},
		{"correcting a dart", v, turnOf(ben), dart, correct},
		{"correcting in the bull-off", v, bullOff, dart, correct},
		{"correcting without being able to throw", watching, turnOf(anna), dart, ""},
	}
	for _, tt := range tests {
		if got := boardAction(tt.v, tt.p, tt.editing); got != tt.want {
			t.Errorf("%s: boardAction = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestLastTurnOf(t *testing.T) {
	t.Parallel()
	st := x01.State{Turns: []x01.Turn{
		{PlayerID: 1, Round: 1, Points: 125, Complete: true},
		{PlayerID: 2, Round: 1, Points: 0, Bust: true, Complete: true},
		{PlayerID: 1, Round: 2, Points: 60, Complete: true},
		{PlayerID: 2, Round: 2, Points: 20}, // still throwing
	}}
	v := service.GameView{State: st}
	texts := textsOf(i18n.English)

	if got, ok := lastTurnOf(texts, v, 1); !ok || got.Value != "60" || got.Strong || got.Bust {
		t.Errorf("player 1 = %+v, %v; want the 60", got, ok)
	}
	if got, ok := lastTurnOf(texts, v, 2); !ok || !got.Bust || got.Value != texts.Busted() {
		t.Errorf("player 2 = %+v, %v; want the bust (the unfinished turn doesn't count)", got, ok)
	}
	if _, ok := lastTurnOf(texts, v, 3); ok {
		t.Error("a player who didn't throw has a last turn")
	}
}

// visitView is an X01 game of the players after the darts.
func visitView(t *testing.T, s x01.Settings, players []int64, log turn.Log) service.GameView {
	t.Helper()
	log.PlayerIDs = players
	st, err := x01.PlayLog(s, players, log)
	if err != nil {
		t.Fatal(err)
	}
	return service.GameView{State: st, Darts: log.Entries}
}

func TestVisitScore(t *testing.T) {
	t.Parallel()
	dart := func(id int64, segment dartboard.Segment, number uint8) throw.Dart {
		return throw.Dart{ID: id, PlayerID: 1, Number: number, Segment: segment}
	}
	s := x01.Settings{StartScore: 40, OutMode: x01.OutModeDouble}
	two := []int64{1, 2}
	texts := textsOf(i18n.English)
	score := func(v service.GameView) (lastTurn, bool) {
		return visitScore(texts, v, progressOf(v))
	}

	fives := []throw.Dart{dart(1, dartboard.Single, 5), dart(2, dartboard.Single, 5), dart(3, dartboard.Single, 5)}
	if got, ok := score(visitView(t, s, two, turn.Log{Entries: fives})); !ok || got.Value != "15" || got.Bust {
		t.Errorf("a visit = %+v, %v; want the 15", got, ok)
	}
	if got, ok := score(visitView(t, s, two, turn.Log{Entries: fives, WaitForTakeout: true})); !ok || got.Value != "15" {
		t.Errorf("waiting for the takeout = %+v, %v; want the 15", got, ok)
	}
	if _, ok := score(visitView(t, s, two, turn.Log{Entries: fives[:2]})); ok {
		t.Error("a visit that isn't finished shows a score")
	}
	fourth := dart(4, dartboard.Single, 5)
	fourth.PlayerID = 2
	if _, ok := score(visitView(t, s, two, turn.Log{Entries: append(slices.Clone(fives), fourth)})); ok {
		t.Error("the score of the visit shows after the next player's dart")
	}

	bust := visitView(t, s, two, turn.Log{Entries: []throw.Dart{dart(1, dartboard.Single, 20), dart(2, dartboard.Single, 20)}})
	if got, ok := score(bust); !ok || !got.Bust || got.Value != texts.Busted() {
		t.Errorf("a bust = %+v, %v; want Busted", got, ok)
	}

	big := visitView(t, x01.Settings{StartScore: 501}, two, turn.Log{Entries: []throw.Dart{dart(1, dartboard.Triple, 20), dart(2, dartboard.Triple, 20), dart(3, dartboard.Triple, 20)}})
	if got, ok := score(big); !ok || got.Value != "180!" || !got.Max {
		t.Errorf("a 180 = %+v, %v; want 180!", got, ok)
	}

	won := visitView(t, x01.Settings{StartScore: 100, OutMode: x01.OutModeDouble}, two, turn.Log{Entries: []throw.Dart{dart(1, dartboard.Triple, 20), dart(2, dartboard.Double, 20)}, Legs: 2})
	if _, ok := score(won); ok {
		t.Error("a won leg has its own banner")
	}
}

func TestCricketMark(t *testing.T) {
	t.Parallel()
	for marks, want := range []struct{ lines, circles int }{{0, 0}, {1, 0}, {2, 0}, {2, 1}, {2, 1}} {
		var b strings.Builder
		if err := cricketMark(uint8(marks)).Render(context.Background(), &b); err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(b.String(), "<line "); got != want.lines {
			t.Errorf("%d marks draw %d lines, want %d", marks, got, want.lines)
		}
		if got := strings.Count(b.String(), "<circle "); got != want.circles {
			t.Errorf("%d marks draw %d circles, want %d", marks, got, want.circles)
		}
	}
}

func TestCricketDartsWithoutMarksMiss(t *testing.T) {
	t.Parallel()
	scored := throw.Dart{ID: 1, PlayerID: 1, Number: 20, Segment: dartboard.Triple}
	dead := throw.Dart{ID: 2, PlayerID: 1, Number: 20, Segment: dartboard.Single}
	// Alone in the game, the player's triple closes the 20 and nothing more scores on it.
	st, err := cricket.Play(cricket.Settings{Variant: cricket.VariantStandard}, []int64{1}, []throw.Dart{scored, dead})
	if err != nil {
		t.Fatal(err)
	}
	v := service.GameView{State: st}
	v.Game.Mode = game.ModeCricket
	v.Darts = []throw.Dart{scored}

	if hit, counted := targetResult(v, scored); !hit || !counted {
		t.Errorf("a dart with marks = %v, %v; want a counted hit", hit, counted)
	}
	if hit, counted := targetResult(v, dead); hit || !counted {
		t.Errorf("a dart without marks = %v, %v; want a counted miss", hit, counted)
	}
	if got := soundKind(v); got != "triple" {
		t.Errorf("sound of a triple that scored = %q, want triple", got)
	}
	v.Darts = []throw.Dart{scored, dead}
	if got := soundKind(v); got != "miss" {
		t.Errorf("sound of a dart without marks = %q, want miss", got)
	}
}

func TestLastTurnOfCricket(t *testing.T) {
	t.Parallel()
	st := cricket.State{Variant: cricket.VariantStandard, Turns: []cricket.Turn{
		{PlayerID: 1, Marks: []int{3, 1, 0}, Points: 40, Complete: true},
		{PlayerID: 2, Marks: []int{2, 0, 0}, Complete: true},
		{PlayerID: 1, Marks: []int{1}}, // still throwing
	}}
	v := service.GameView{State: st}
	texts := textsOf(i18n.English)

	if got, ok := lastTurnOf(texts, v, 1); !ok || got.Value != "4 marks · 40" {
		t.Errorf("player 1 = %+v, %v; want marks and points", got, ok)
	}
	if got, ok := lastTurnOf(texts, v, 2); !ok || got.Value != "2 marks" {
		t.Errorf("player 2 = %+v, %v; want only the marks", got, ok)
	}
	// In Cut-Throat the points went to the opponents.
	st.Variant = cricket.VariantCutThroat
	v = service.GameView{State: st}
	if got, ok := lastTurnOf(texts, v, 1); !ok || got.Value != "4 marks" {
		t.Errorf("cut-throat: player 1 = %+v, %v; want only the marks", got, ok)
	}
}

func TestBoardTargetsOfCricket(t *testing.T) {
	t.Parallel()
	// Both players close the 20, only player 1 the 19: the 20 is dead, the 19 still open.
	darts := []throw.Dart{
		{ID: 1, PlayerID: 1, Number: 20, Segment: dartboard.Triple},
		{ID: 2, PlayerID: 1, Number: 19, Segment: dartboard.Triple},
		{ID: 3, PlayerID: 1, Number: 0, Segment: dartboard.Miss},
		{ID: 4, PlayerID: 2, Number: 20, Segment: dartboard.Triple},
	}
	st, err := cricket.Play(cricket.Settings{Variant: cricket.VariantStandard}, []int64{1, 2}, darts)
	if err != nil {
		t.Fatal(err)
	}
	v := service.GameView{State: st}

	want := []uint8{19, 18, 17, 16, 15, dartboard.BullNumber}
	if got := boardTargetsOf(v, progressOf(v)); !slices.Equal(got, want) {
		t.Errorf("targets = %v, want %v", got, want)
	}
	if !isBoardTarget(want, "19") || isBoardTarget(want, "20") {
		t.Error("isBoardTarget doesn't match the numbers printed around the board")
	}
}

// A bot's spread is shown with a tenth of a mm in the language's number format, also next to
// the slider that chooses it, which starts at the default.
func TestBotSpreadText(t *testing.T) {
	t.Parallel()
	en, de := textsOf(i18n.English), textsOf(i18n.German)
	if got := spreadMM(en, 12_500); got != "12.5 mm" {
		t.Errorf("English %q, want 12.5 mm", got)
	}
	if got := spreadMM(de, 12_500); got != "12,5 mm" {
		t.Errorf("German %q, want 12,5 mm", got)
	}
	got := botSliderValue(en, game.ModeX01, bot.Position(bot.DefaultSpreadUM))
	if want := "20.0 mm " + botStrength(en, game.ModeX01, bot.DefaultSpreadUM); got != want {
		t.Errorf("slider at the default shows %q, want %q", got, want)
	}
	if got := botSliderValue(en, game.ModeX01, 0); !strings.HasPrefix(got, "80.0 mm") {
		t.Errorf("slider at the left end shows %q, want the weakest, 80 mm", got)
	}
}
