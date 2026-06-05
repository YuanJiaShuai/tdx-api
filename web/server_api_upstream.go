package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/injoyai/tdx"
	"github.com/injoyai/tdx/protocol"
)

func handleGetCallAuction(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		errorResponse(w, "code 为必填参数")
		return
	}

	resp, err := client.GetCallAuction(code)
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取集合竞价失败: %v", err))
		return
	}
	successResponse(w, resp)
}

func handleGetGbbq(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		errorResponse(w, "code 为必填参数")
		return
	}

	resp, err := client.GetGbbq(code)
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取股本变迁失败: %v", err))
		return
	}
	successResponse(w, resp)
}

func handleGetFinance(w http.ResponseWriter, r *http.Request) {
	exchange, number, err := parseExchangeCodeParam(r)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}

	resp, err := client.GetFinanceInfo(exchange, number)
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取财务信息失败: %v", err))
		return
	}
	successResponse(w, resp)
}

func handleGetCompanyCategories(w http.ResponseWriter, r *http.Request) {
	exchange, number, err := parseExchangeCodeParam(r)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}

	resp, err := client.GetCompanyCategory(exchange, number)
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取公司资料目录失败: %v", err))
		return
	}
	successResponse(w, resp)
}

func handleGetCompanyContent(w http.ResponseWriter, r *http.Request) {
	exchange, number, err := parseExchangeCodeParam(r)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}

	filename := strings.TrimSpace(r.URL.Query().Get("filename"))
	if filename == "" {
		errorResponse(w, "filename 为必填参数")
		return
	}
	start, err := parseUint32Query(r, "start", 0)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	length, err := parseUint32Query(r, "length", 0)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	if length == 0 {
		errorResponse(w, "length 必须大于 0")
		return
	}

	resp, err := client.GetCompanyContent(exchange, number, filename, start, length)
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取公司资料内容失败: %v", err))
		return
	}
	successResponse(w, map[string]interface{}{
		"content": resp,
	})
}

func handleGetBlockData(w http.ResponseWriter, r *http.Request) {
	file := resolveBlockFile(r.URL.Query().Get("file"))
	withIndex := parseBool(r.URL.Query().Get("with_index"))

	var (
		resp []*protocol.Block
		err  error
	)
	if withIndex {
		resp, err = client.GetBlockDataWithIndex(file)
	} else {
		resp, err = client.GetBlockData(file)
	}
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取板块数据失败: %v", err))
		return
	}
	successResponse(w, map[string]interface{}{
		"file":       file,
		"with_index": withIndex,
		"count":      len(resp),
		"list":       resp,
	})
}

func handleGetTdxHy(w http.ResponseWriter, r *http.Request) {
	resp, err := client.GetTdxHy()
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取行业归属失败: %v", err))
		return
	}
	successResponse(w, map[string]interface{}{"count": len(resp), "list": resp})
}

func handleGetTdxStat(w http.ResponseWriter, r *http.Request) {
	resp, err := client.GetTdxStat()
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取个股统计失败: %v", err))
		return
	}
	successResponse(w, map[string]interface{}{"count": len(resp), "list": resp})
}

func handleGetTdxStat2(w http.ResponseWriter, r *http.Request) {
	resp, err := client.GetTdxStat2()
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取资金流向统计失败: %v", err))
		return
	}
	successResponse(w, map[string]interface{}{"count": len(resp), "list": resp})
}

func handleGetXgsg(w http.ResponseWriter, r *http.Request) {
	resp, err := client.GetXgsg()
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取新股申购失败: %v", err))
		return
	}
	successResponse(w, map[string]interface{}{"count": len(resp), "list": resp})
}

func handleExHqMarkets(w http.ResponseWriter, r *http.Request) {
	ex, err := tdx.DialExHqDefault(tdx.WithDebug(false))
	if err != nil {
		errorResponse(w, fmt.Sprintf("连接扩展行情失败: %v", err))
		return
	}
	defer ex.Close()

	resp, err := ex.ExMarkets()
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取扩展市场失败: %v", err))
		return
	}
	successResponse(w, resp)
}

func handleExHqCount(w http.ResponseWriter, r *http.Request) {
	ex, err := tdx.DialExHqDefault(tdx.WithDebug(false))
	if err != nil {
		errorResponse(w, fmt.Sprintf("连接扩展行情失败: %v", err))
		return
	}
	defer ex.Close()

	resp, err := ex.ExCount()
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取扩展品种数量失败: %v", err))
		return
	}
	successResponse(w, map[string]int{"count": resp})
}

func handleExHqInstruments(w http.ResponseWriter, r *http.Request) {
	start, err := parseUint32Query(r, "start", 0)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	count, err := parseUint16Query(r, "count", 100)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}

	ex, err := tdx.DialExHqDefault(tdx.WithDebug(false))
	if err != nil {
		errorResponse(w, fmt.Sprintf("连接扩展行情失败: %v", err))
		return
	}
	defer ex.Close()

	resp, err := ex.ExInstruments(start, count)
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取扩展品种失败: %v", err))
		return
	}
	successResponse(w, map[string]interface{}{"count": len(resp), "list": resp})
}

func handleExHqQuote(w http.ResponseWriter, r *http.Request) {
	market, code, err := parseExMarketCode(r)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}

	ex, err := tdx.DialExHqDefault(tdx.WithDebug(false))
	if err != nil {
		errorResponse(w, fmt.Sprintf("连接扩展行情失败: %v", err))
		return
	}
	defer ex.Close()

	resp, err := ex.ExQuote(market, code)
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取扩展行情失败: %v", err))
		return
	}
	successResponse(w, resp)
}

func handleExHqBars(w http.ResponseWriter, r *http.Request) {
	market, code, err := parseExMarketCode(r)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	category, err := parseUint8Query(r, "category", protocol.TypeKlineDay)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	start, err := parseUint16Query(r, "start", 0)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	count, err := parseUint16Query(r, "count", 100)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}

	ex, err := tdx.DialExHqDefault(tdx.WithDebug(false))
	if err != nil {
		errorResponse(w, fmt.Sprintf("连接扩展行情失败: %v", err))
		return
	}
	defer ex.Close()

	resp, err := ex.ExBars(category, market, code, start, count)
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取扩展K线失败: %v", err))
		return
	}
	successResponse(w, map[string]interface{}{"count": len(resp), "list": resp})
}

func handleExHqTrade(w http.ResponseWriter, r *http.Request) {
	market, code, err := parseExMarketCode(r)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	start, err := parseUint16Query(r, "start", 0)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	count, err := parseUint16Query(r, "count", 200)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}

	ex, err := tdx.DialExHqDefault(tdx.WithDebug(false))
	if err != nil {
		errorResponse(w, fmt.Sprintf("连接扩展行情失败: %v", err))
		return
	}
	defer ex.Close()

	resp, err := ex.ExTrade(market, code, start, count)
	if err != nil {
		errorResponse(w, fmt.Sprintf("获取扩展分笔成交失败: %v", err))
		return
	}
	successResponse(w, map[string]interface{}{"count": len(resp), "list": resp})
}

func parseExchangeCodeParam(r *http.Request) (protocol.Exchange, string, error) {
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		return 0, "", fmt.Errorf("code 为必填参数")
	}
	exchange, number, err := protocol.DecodeCode(protocol.AddPrefix(code))
	if err != nil {
		return 0, "", fmt.Errorf("code 参数错误: %w", err)
	}
	return exchange, number, nil
}

func parseExMarketCode(r *http.Request) (uint8, string, error) {
	market, err := parseUint8Query(r, "market", 0)
	if err != nil {
		return 0, "", err
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		return 0, "", fmt.Errorf("code 为必填参数")
	}
	return market, code, nil
}

func resolveBlockFile(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "gn", "concept":
		return protocol.BlockFileGN
	case "fg", "style", "region":
		return protocol.BlockFileFG
	case "zs", "index":
		return protocol.BlockFileZS
	case "hy", "industry":
		return protocol.BlockFileHY
	case "block":
		return protocol.BlockFile
	default:
		return value
	}
}

func parseUint8Query(r *http.Request, key string, fallback uint8) (uint8, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.ParseUint(value, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("%s 参数无效", key)
	}
	return uint8(n), nil
}

func parseUint16Query(r *http.Request, key string, fallback uint16) (uint16, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("%s 参数无效", key)
	}
	return uint16(n), nil
}

func parseUint32Query(r *http.Request, key string, fallback uint32) (uint32, error) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s 参数无效", key)
	}
	return uint32(n), nil
}
