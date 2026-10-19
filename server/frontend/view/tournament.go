package view

import (
	"context"
	"strconv"
	"strings"

	"github.com/sj14/darts-o-mat/server/domain/game"
	"github.com/sj14/darts-o-mat/server/domain/tournament"
	"github.com/sj14/darts-o-mat/server/service"
)

func tournamentURL(id int64, suffix string) string {
	return "/tournaments/" + strconv.FormatInt(id, 10) + suffix
}

func formatName(t Texts, f tournament.Format) string {
	if f == tournament.Knockout {
		return t.Knockout()
	}
	return t.RoundRobin()
}

func tournamentLabel(texts Texts, t tournament.Tournament) string {
	return formatName(texts, t.Format) + " · " + modeLabel(texts, t.Settings)
}

func tournamentPlayerName(ctx context.Context, v service.TournamentView, id int64) string {
	for _, p := range v.Players {
		if p.ID == id {
			return displayName(ctx, p)
		}
	}
	return "?"
}

// tournamentPending names the invited players who haven't accepted yet.
func tournamentPending(ctx context.Context, v service.TournamentView) string {
	var names []string
	for _, p := range v.Tournament.Participants {
		if p.Status == game.InvitationPending {
			names = append(names, tournamentPlayerName(ctx, v, p.PlayerID))
		}
	}
	return strings.Join(names, ", ")
}

// scoreUnit is what the score of a match counts: sets, or legs in games without sets.
func scoreUnit(t Texts, g game.Settings) string {
	if g.Sets > 1 {
		return t.Sets()
	}
	return t.Legs()
}

// signed writes a difference with its sign, e.g. +3.
func signed(n int) string {
	if n > 0 {
		return "+" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
