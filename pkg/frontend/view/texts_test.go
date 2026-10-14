package view

import (
	"reflect"
	"slices"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/i18n"
)

// sameInAllLanguages are texts that are the same in every language on purpose; they
// don't count as untranslated.
var sameInAllLanguages = []string{
	"NavBoards", "AroundTheClock", "SetN", "LegN", "Checkout", "Single", "Triple", "Bull", "Bullseye", "Darts",
}

// complete are the languages that must have every text translated; for the others the
// test only lists what is missing (and falls back to English).
var complete = []i18n.Lang{i18n.German}

// TestTranslations calls every text in every language and compares it with English: a
// text that comes out the same isn't translated yet.
func TestTranslations(t *testing.T) {
	en := reflect.ValueOf(textsOf(i18n.English))
	typ := reflect.TypeFor[Texts]()
	for i := range typ.NumMethod() {
		m := typ.Method(i)
		if out := call(en.MethodByName(m.Name)); out == "" {
			t.Errorf("English %s is empty", m.Name)
		}
	}
	for _, lang := range i18n.Supported[1:] {
		texts := reflect.ValueOf(textsOf(lang))
		var missing []string
		for i := range typ.NumMethod() {
			name := typ.Method(i).Name
			if slices.Contains(sameInAllLanguages, name) {
				continue
			}
			if call(texts.MethodByName(name)) == call(en.MethodByName(name)) {
				missing = append(missing, name)
			}
		}
		switch {
		case len(missing) > 0 && slices.Contains(complete, lang):
			t.Errorf("%s: not translated: %v", lang, missing)
		case len(missing) > 0:
			t.Logf("%s: not translated yet (English instead): %v", lang, missing)
		}
	}
}

// call calls a text method with example arguments.
func call(m reflect.Value) string {
	var args []reflect.Value
	for i := range m.Type().NumIn() {
		switch m.Type().In(i).Kind() {
		case reflect.String:
			args = append(args, reflect.ValueOf("Anna"))
		case reflect.Int:
			args = append(args, reflect.ValueOf(2))
		case reflect.Float64:
			args = append(args, reflect.ValueOf(0.5))
		default:
			panic("no example argument for " + m.Type().In(i).String())
		}
	}
	return m.Call(args)[0].String()
}
