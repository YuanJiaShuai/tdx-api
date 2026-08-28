package main

import (
	"sort"
	"strings"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

const MarketPoolPrefix = "market-"

type MarketPoolDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func marketPoolDefinitions() []MarketPoolDefinition {
	return []MarketPoolDefinition{
		{ID: "market-all-a", Name: "全部A股", Description: "沪深京全部A股，首版策略运行会做数量保护。"},
		{ID: "market-sh-main", Name: "沪市主板", Description: "上海证券交易所主板股票。"},
		{ID: "market-star", Name: "科创板", Description: "科创板，通常为688、689开头。"},
		{ID: "market-sz-main", Name: "深市主板", Description: "深圳证券交易所主板股票，包含原中小板。"},
		{ID: "market-gem", Name: "创业板", Description: "创业板，通常为300、301开头。"},
		{ID: "market-bj", Name: "北交所", Description: "北京证券交易所股票。"},
		{ID: "market-old-sme", Name: "原中小板", Description: "历史中小板识别，通常为002开头，现已并入深市主板。"},
	}
}

func isMarketPoolID(id string) bool {
	return strings.HasPrefix(strings.TrimSpace(id), MarketPoolPrefix)
}

func listMarketStockPools() []StockPool {
	defs := marketPoolDefinitions()
	items := make([]StockPool, 0, len(defs))
	for _, def := range defs {
		items = append(items, StockPool{
			ID:          def.ID,
			Name:        def.Name,
			Description: def.Description,
			Symbols:     marketPoolSymbols(def.ID),
			System:      true,
			Readonly:    true,
			Category:    "market",
		})
	}
	return items
}

func getMarketStockPool(id string) (StockPool, bool) {
	for _, def := range marketPoolDefinitions() {
		if def.ID == id {
			return StockPool{
				ID:          def.ID,
				Name:        def.Name,
				Description: def.Description,
				Symbols:     marketPoolSymbols(def.ID),
				System:      true,
				Readonly:    true,
				Category:    "market",
			}, true
		}
	}
	return StockPool{}, false
}

func marketPoolSymbols(poolID string) []string {
	models, err := getAllCodeModels()
	if err != nil {
		return nil
	}
	symbols := make([]string, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		fullCode := model.FullCode()
		if !protocol.IsStock(fullCode) {
			continue
		}
		code := normalizeMarketCode(model.Code)
		if code == "" || !marketPoolMatches(poolID, strings.ToLower(model.Exchange), code) {
			continue
		}
		symbols = append(symbols, code)
	}
	symbols = normalizeSymbols(symbols)
	sort.Strings(symbols)
	return symbols
}

func marketPoolMatches(poolID, exchange, code string) bool {
	switch poolID {
	case "market-all-a":
		return exchange == "sh" || exchange == "sz" || exchange == "bj"
	case "market-sh-main":
		return exchange == "sh" && (strings.HasPrefix(code, "600") || strings.HasPrefix(code, "601") || strings.HasPrefix(code, "603") || strings.HasPrefix(code, "605"))
	case "market-star":
		return exchange == "sh" && (strings.HasPrefix(code, "688") || strings.HasPrefix(code, "689"))
	case "market-sz-main":
		return exchange == "sz" && (strings.HasPrefix(code, "000") || strings.HasPrefix(code, "001") || strings.HasPrefix(code, "002") || strings.HasPrefix(code, "003"))
	case "market-gem":
		return exchange == "sz" && (strings.HasPrefix(code, "300") || strings.HasPrefix(code, "301"))
	case "market-bj":
		return exchange == "bj"
	case "market-old-sme":
		return exchange == "sz" && strings.HasPrefix(code, "002")
	default:
		return false
	}
}

func normalizeMarketCode(code string) string {
	code = strings.TrimSpace(strings.ToUpper(code))
	code = strings.TrimPrefix(code, "SH")
	code = strings.TrimPrefix(code, "SZ")
	code = strings.TrimPrefix(code, "BJ")
	code = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(code, ".SH"), ".SZ"), ".BJ")
	return code
}

func limitedMarketPoolSymbols(poolID string, maxCodes int) []string {
	symbols := marketPoolSymbols(poolID)
	if maxCodes > 0 && len(symbols) > maxCodes {
		return symbols[:maxCodes]
	}
	if len(symbols) == 0 && poolID == "market-all-a" && tdx.DefaultCodes != nil {
		return normalizeSymbols(tdx.DefaultCodes.GetStocks(maxCodes))
	}
	return symbols
}
