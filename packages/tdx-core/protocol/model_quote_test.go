package protocol

import (
	"testing"
)

func Test_quote_Frame(t *testing.T) {
	//0c0000000001130013003e050500000000000000010000303030303031
	f, err := MQuote.Frame("sz000001")
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(f.Bytes().HEX())
}

func TestDecodeQuoteRate(t *testing.T) {
	tests := []struct {
		name string
		raw  uint16
		want float64
	}{
		{name: "positive", raw: 18, want: 0.18},
		{name: "negative", raw: 65528, want: -0.08},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeQuoteRate(tt.raw); got != tt.want {
				t.Fatalf("decodeQuoteRate(%d) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
