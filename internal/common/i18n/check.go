package i18n

import (
	"errors"
	"reflect"
	"slices"
	"time"
)

// Empty returns the methods of the texts interface T whose text in base (English) is
// empty. For tests of the texts of an interface (see view.Texts).
func Empty[T any](base T) []string {
	var empty []string
	v := reflect.ValueOf(base)
	for m := range reflect.TypeFor[T]().Methods() {
		if call(v.MethodByName(m.Name)) == "" {
			empty = append(empty, m.Name)
		}
	}
	return empty
}

// Untranslated returns the methods of the texts interface T whose text in other is the
// same as in base: not translated yet, unless listed in same (the same in every language
// on purpose, e.g. "Bull"). Methods with parameters are called with example arguments.
func Untranslated[T any](base, other T, same []string) []string {
	var missing []string
	b, o := reflect.ValueOf(base), reflect.ValueOf(other)
	for m := range reflect.TypeFor[T]().Methods() {
		if !slices.Contains(same, m.Name) && call(b.MethodByName(m.Name)) == call(o.MethodByName(m.Name)) {
			missing = append(missing, m.Name)
		}
	}
	return missing
}

var (
	errorType = reflect.TypeFor[error]()
	timeType  = reflect.TypeFor[time.Time]()
)

// call calls a text method with example arguments and returns its text.
func call(m reflect.Value) string {
	var args []reflect.Value
	for in := range m.Type().Ins() {
		switch {
		case in == errorType:
			args = append(args, reflect.ValueOf(errors.New("example")))
		case in == timeType:
			args = append(args, reflect.ValueOf(time.Date(2026, time.October, 30, 20, 0, 0, 0, time.UTC)))
		case in.Kind() == reflect.String:
			args = append(args, reflect.ValueOf("Anna"))
		case in.Kind() == reflect.Int:
			args = append(args, reflect.ValueOf(2))
		case in.Kind() == reflect.Float64:
			args = append(args, reflect.ValueOf(0.5))
		default:
			panic("no example argument for " + in.String())
		}
	}
	return m.Call(args)[0].String()
}
