package throw

import (
	"encoding/json"
	"testing"
)

func TestString(t *testing.T) {
	tests := []struct {
		dart Dart
		want string
	}{
		{Dart{Number: 20, Segment: Triple}, "T20"},
		{Dart{Number: 20, Segment: Inner}, "20"},
		{Dart{Number: BullNumber, Segment: Bullseye}, "Bullseye"},
		{Dart{Segment: Miss, Number: 7}, "M7"},
		{Dart{Segment: Miss}, "Miss"},
	}
	for _, tt := range tests {
		if got := tt.dart.String(); got != tt.want {
			t.Errorf("%+v: got %q, want %q", tt.dart, got, tt.want)
		}
	}
}

func TestScore(t *testing.T) {
	tests := []struct {
		dart Dart
		want uint64
	}{
		{Dart{Number: 20, Segment: Triple}, 60},
		{Dart{Number: 20, Segment: Outer}, 20},
		{Dart{Number: 5, Segment: Double}, 10},
		{Dart{Number: BullNumber, Segment: Bull}, 25},
		{Dart{Number: BullNumber, Segment: Bullseye}, 50},
		{Dart{Number: 7, Segment: Miss}, 0},
	}
	for _, tt := range tests {
		if got := tt.dart.Score(); got != tt.want {
			t.Errorf("%v: %d points, want %d", tt.dart, got, tt.want)
		}
	}
}

func TestValidate(t *testing.T) {
	valid := []Dart{
		{Segment: Miss, Number: 20},
		{Segment: Miss},
		{Segment: Single, Number: 1},
		{Segment: Bullseye, Number: BullNumber},
	}
	for _, d := range valid {
		if err := d.Validate(); err != nil {
			t.Errorf("%+v: %v", d, err)
		}
	}
	invalid := []Dart{
		{Segment: Miss, Number: BullNumber},   // beside the bull
		{Segment: Triple, Number: BullNumber}, // no triple bull
		{Segment: Bull, Number: 20},           // the bull is 25
		{Segment: Double, Number: 21},
		{Number: 20},                       // no segment
		{Segment: Bullseye + 1, Number: 1}, // unknown segment
	}
	for _, d := range invalid {
		if err := d.Validate(); err == nil {
			t.Errorf("%+v: no error", d)
		}
	}
}

func TestSegmentJSON(t *testing.T) {
	data, err := json.Marshal(Dart{Number: 20, Segment: Outer})
	if err != nil {
		t.Fatal(err)
	}
	var d Dart
	if err := json.Unmarshal(data, &d); err != nil || d.Segment != Outer {
		t.Errorf("%s: got %+v, %v", data, d, err)
	}
	if err := json.Unmarshal([]byte(`{"number":20,"segment":"quadruple"}`), &d); err == nil {
		t.Error("unknown segment: no error")
	}
	// A takeout has no segment, and none in its JSON.
	if data, _ := json.Marshal(Dart{Takeout: true}); string(data) != `{"id":0,"number":0,"takeout":true}` {
		t.Errorf("takeout: %s", data)
	}
}
