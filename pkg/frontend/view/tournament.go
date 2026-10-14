package view

import (
	"strconv"
	"strings"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/tournament"
	"github.com/sj14/darts-o-mat/pkg/service"
)

func tournamentURL(id int64, suffix string) string {
	return "/tournaments/" + strconv.FormatInt(id, 10) + suffix
}

func formatName(f tournament.Format) string {
	if f == tournament.Knockout {
		return "K.-o.-Turnier"
	}
	return "Jeder gegen jeden"
}

func tournamentLabel(t tournament.Tournament) string {
	return formatName(t.Format) + " · " + modeLabel(t.Settings)
}

func tournamentPlayerName(v service.TournamentView, id int64) string {
	for _, p := range v.Players {
		if p.ID == id {
			return p.Name
		}
	}
	return "?"
}

// tournamentPending names the invited players who haven't accepted yet.
func tournamentPending(v service.TournamentView) string {
	var names []string
	for _, inv := range v.Invitations {
		if inv.Status == game.InvitationPending {
			names = append(names, tournamentPlayerName(v, inv.PlayerID))
		}
	}
	return strings.Join(names, ", ")
}

// roundName names a round of a knockout bracket with rounds rounds.
func roundName(rounds, r int) string {
	switch rounds - r {
	case 0:
		return "Finale"
	case 1:
		return "Halbfinale"
	case 2:
		return "Viertelfinale"
	case 3:
		return "Achtelfinale"
	}
	return "Runde " + strconv.Itoa(r)
}

// scoreUnit is what the score of a match counts: sets, or legs in games without sets.
func scoreUnit(g game.Game) string {
	if g.Sets > 1 {
		return "Sets"
	}
	return "Legs"
}

// signed writes a difference with its sign, e.g. +3.
func signed(n int) string {
	if n > 0 {
		return "+" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
