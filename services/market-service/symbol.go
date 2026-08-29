package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/injoyai/tdx/protocol"
)

const (
	MarketCNA   = "CN_A"
	MarketETF   = "CN_ETF"
	MarketIndex = "CN_INDEX"
)

// Symbol describes the canonical identity used between market-service clients.
// TDXCode is kept separate because the upstream protocol uses a lowercase
// exchange prefix (for example: sh600519).
type Symbol struct {
	Symbol   string `json:"symbol"`
	Code     string `json:"code"`
	Exchange string `json:"exchange"`
	Market   string `json:"market"`
	TDXCode  string `json:"tdx_code"`
}

var sixDigitCode = regexp.MustCompile(`^\d{6}$`)

// NormalizeSymbol accepts common user forms such as 600519, SZ000001 and
// 600519.SH, and returns a canonical symbol plus its TDX representation.
func NormalizeSymbol(raw string) (Symbol, error) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, "_", ".")
	value = strings.ReplaceAll(value, "-", ".")
	value = strings.ReplaceAll(value, ":", ".")
	value = strings.ReplaceAll(value, " ", "")

	var exchange, code string
	switch {
	case len(value) == 9 && sixDigitCode.MatchString(value[:6]) && value[6] == '.' && len(value[7:]) == 2:
		code, exchange = value[:6], value[7:]
	case len(value) == 8 && sixDigitCode.MatchString(value[2:]):
		exchange, code = value[:2], value[2:]
	case sixDigitCode.MatchString(value):
		tdxCode := protocol.AddPrefix(strings.ToLower(value))
		if len(tdxCode) != 8 {
			return Symbol{}, fmt.Errorf("无法识别股票代码: %s", raw)
		}
		exchange, code = strings.ToUpper(tdxCode[:2]), tdxCode[2:]
	default:
		return Symbol{}, fmt.Errorf("股票代码格式无效: %s", raw)
	}

	switch exchange {
	case "SH", "SZ", "BJ":
	default:
		return Symbol{}, fmt.Errorf("暂不支持交易所: %s", exchange)
	}

	tdxCode := strings.ToLower(exchange + code)
	market := MarketCNA
	if protocol.IsETF(tdxCode) {
		market = MarketETF
	} else if protocol.IsIndex(tdxCode) {
		market = MarketIndex
	}

	return Symbol{
		Symbol:   code + "." + exchange,
		Code:     code,
		Exchange: exchange,
		Market:   market,
		TDXCode:  tdxCode,
	}, nil
}

func normalizeSymbols(values []string) ([]Symbol, error) {
	result := make([]Symbol, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		symbol, err := NormalizeSymbol(value)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[symbol.Symbol]; ok {
			continue
		}
		seen[symbol.Symbol] = struct{}{}
		result = append(result, symbol)
	}
	return result, nil
}

func normalizeCodeParam(param string) ([]Symbol, error) {
	parts := strings.Split(param, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return normalizeSymbols(values)
}
