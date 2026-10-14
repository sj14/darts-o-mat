package throw

import "testing"

func TestString(t *testing.T) {
	tests := []struct {
		dart Dart
		want string
	}{
		{Dart{Number: 20, Multiplier: 3}, "T20"},
		{Dart{Number: Bull, Multiplier: 2}, "Bullseye"},
		{Dart{Miss: true, Number: 7}, "M7"},
		{Dart{Miss: true}, "Daneben"},
	}
	for _, tt := range tests {
		if got := tt.dart.String(); got != tt.want {
			t.Errorf("%+v: got %q, want %q", tt.dart, got, tt.want)
		}
	}
}

func TestValidateMiss(t *testing.T) {
	if err := (Dart{Miss: true, Number: 20}).Validate(); err != nil {
		t.Errorf("miss beside the 20: %v", err)
	}
	if err := (Dart{Miss: true, Number: Bull}).Validate(); err == nil {
		t.Error("miss beside the bull: no error")
	}
}
