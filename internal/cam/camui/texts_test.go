package camui

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/i18n"
)

// sameInAllLanguages are texts that are the same in every language on purpose.
var sameInAllLanguages = []string{"Server", "PositionMM"}

// TestTranslations checks that English is complete and German fully translated (see
// view.TestTranslations).
func TestTranslations(t *testing.T) {
	en := textsOf(i18n.English)
	if empty := i18n.Empty(en); len(empty) > 0 {
		t.Errorf("English texts are empty: %v", empty)
	}
	if missing := i18n.Untranslated(en, textsOf(i18n.German), sameInAllLanguages); len(missing) > 0 {
		t.Errorf("de: not translated: %v", missing)
	}
}

// TestLanguage checks that the pages follow the browser's language.
func TestLanguage(t *testing.T) {
	_, srv := newUI(t)
	for lang, want := range map[string]string{
		"":               "Detection is waiting for a calibrated camera.",
		"de-DE,de;q=0.9": "Erkennung wartet auf eine kalibrierte Kamera.",
		"fr-FR,fr;q=0.9": "Detection is waiting for a calibrated camera.",
	} {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
		req.Header.Set("Accept-Language", lang)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !strings.Contains(string(body), want) {
			t.Errorf("Accept-Language %q: %q missing", lang, want)
		}
	}
}
