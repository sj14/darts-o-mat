package i18n

import "testing"

func TestNegotiate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		header string
		want   Lang
	}{
		{"", English},
		{"de-DE,de;q=0.9,en;q=0.8", German},
		{"de-AT", German},
		{"fr-FR,fr;q=0.9", English},          // not supported: the default
		{"fr-FR,de;q=0.5,en;q=0.7", English}, // the highest supported quality
		{"fr-FR,en;q=0.5,de;q=0.7", German},
		{"en-US,en;q=0.9,de;q=0.9", English}, // equal quality: the browser's order
		{"de;q=0, en;q=0.1", English},        // q=0 means not wanted
		{"  DE-ch ; q=0.8 , xx", German},     // spaces and case
		{"de;q=garbage", German},             // a broken quality counts as 1
	}
	for _, tt := range tests {
		if got := Negotiate(tt.header); got != tt.want {
			t.Errorf("Negotiate(%q) = %s, want %s", tt.header, got, tt.want)
		}
	}
}
