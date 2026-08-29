package main

import "testing"

func TestNormalizeSymbol(t *testing.T) {
	tests := []struct {
		input    string
		symbol   string
		tdxCode  string
		market   string
		exchange string
	}{
		{input: "600519", symbol: "600519.SH", tdxCode: "sh600519", market: MarketCNA, exchange: "SH"},
		{input: "SZ000001", symbol: "000001.SZ", tdxCode: "sz000001", market: MarketCNA, exchange: "SZ"},
		{input: "000001.SZ", symbol: "000001.SZ", tdxCode: "sz000001", market: MarketCNA, exchange: "SZ"},
		{input: "sh510050", symbol: "510050.SH", tdxCode: "sh510050", market: MarketETF, exchange: "SH"},
		{input: "399001.SZ", symbol: "399001.SZ", tdxCode: "sz399001", market: MarketIndex, exchange: "SZ"},
	}

	for _, test := range tests {
		got, err := NormalizeSymbol(test.input)
		if err != nil {
			t.Fatalf("NormalizeSymbol(%q): %v", test.input, err)
		}
		if got.Symbol != test.symbol || got.TDXCode != test.tdxCode ||
			got.Market != test.market || got.Exchange != test.exchange {
			t.Fatalf("NormalizeSymbol(%q) = %+v", test.input, got)
		}
	}
}

func TestNormalizeSymbolRejectsUnsupportedExchange(t *testing.T) {
	if _, err := NormalizeSymbol("00700.HK"); err == nil {
		t.Fatal("expected unsupported exchange error")
	}
}
