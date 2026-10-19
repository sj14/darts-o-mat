// Package i18n chooses the language of the web interfaces of darts-o-mat and darts-o-cam
// and carries it in the request context. The texts themselves live with the interface
// that shows them (e.g. view.Texts): an interface with one method per text, implemented
// completely in English, and per further language a type embedding the English one, so
// whatever isn't translated yet falls back to English.
package i18n

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// Lang is a language, as the primary subtag of a language tag (en, de).
type Lang string

const (
	English Lang = "en"
	German  Lang = "de"
)

// Default is the language when neither the account nor the browser asks for a
// supported one (user's decision, 2026-10-14).
const Default = English

// Supported are the languages there are texts for, the default first.
var Supported = []Lang{English, German}

// Parse returns the supported language of a tag (de, de-AT, EN), false if there is none.
func Parse(tag string) (Lang, bool) {
	primary, _, _ := strings.Cut(strings.TrimSpace(tag), "-")
	l := Lang(strings.ToLower(primary))
	return l, slices.Contains(Supported, l)
}

// Negotiate picks the supported language the browser prefers most (Accept-Language,
// e.g. "de-DE,de;q=0.9,en;q=0.8"), Default if it prefers none.
func Negotiate(acceptLanguage string) Lang {
	type option struct {
		lang Lang
		q    float64
	}
	var options []option
	for part := range strings.SplitSeq(acceptLanguage, ",") {
		tag, params, _ := strings.Cut(part, ";")
		lang, ok := Parse(tag)
		if !ok {
			continue
		}
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				q = f
			}
		}
		if q > 0 {
			options = append(options, option{lang, q})
		}
	}
	// The first of the highest quality; stable keeps the browser's order for equal ones.
	slices.SortStableFunc(options, func(a, b option) int { return cmp.Compare(b.q, a.q) })
	if len(options) == 0 {
		return Default
	}
	return options[0].lang
}

type ctxKey struct{}

// With returns ctx with the language.
func With(ctx context.Context, l Lang) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// From returns the language of the request, Default if none was set.
func From(ctx context.Context) Lang {
	if l, ok := ctx.Value(ctxKey{}).(Lang); ok {
		return l
	}
	return Default
}

// Middleware sets the language the browser asks for. Handlers that know better, e.g.
// from the account's setting, set it again.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(With(r.Context(), Negotiate(r.Header.Get("Accept-Language")))))
	})
}
