package view

import (
	"slices"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/i18n"
)

// sameInAllLanguages are texts that are the same in every language on purpose; they
// don't count as untranslated.
var sameInAllLanguages = []string{
	"NavBoards", "AroundTheClock", "SetN", "LegN", "Checkout", "Single", "Triple", "Bull", "Bullseye", "Darts",
	"Board", "ColorName", "Difference", "Finish", "Legs", "Name", "Server", "Sets",
}

// complete are the languages that must have every text translated; for the others the
// test only lists what is missing (and falls back to English).
var complete = []i18n.Lang{i18n.German}

// TestTranslations calls every text in every language and compares it with English: a
// text that comes out the same isn't translated yet.
func TestTranslations(t *testing.T) {
	en := textsOf(i18n.English)
	if empty := i18n.Empty(en); len(empty) > 0 {
		t.Errorf("English texts are empty: %v", empty)
	}
	for _, lang := range i18n.Supported[1:] {
		missing := i18n.Untranslated(en, textsOf(lang), sameInAllLanguages)
		switch {
		case len(missing) > 0 && slices.Contains(complete, lang):
			t.Errorf("%s: not translated: %v", lang, missing)
		case len(missing) > 0:
			t.Logf("%s: not translated yet (English instead): %v", lang, missing)
		}
	}
}
