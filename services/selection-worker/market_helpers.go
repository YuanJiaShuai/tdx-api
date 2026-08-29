package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/injoyai/tdx/extend"
	"github.com/injoyai/tdx/protocol"
)

func getQfqKlineDay(code string) (*protocol.KlineResp, error) {
	klines, err := extend.GetTHSDayKline(code, extend.THS_QFQ)
	if err != nil {
		return nil, fmt.Errorf("获取前复权数据失败: %w", err)
	}
	if len(klines) == 0 {
		return nil, fmt.Errorf("同花顺前复权数据为空")
	}

	resp := &protocol.KlineResp{
		Count: uint16(len(klines)),
		List:  make([]*protocol.Kline, 0, len(klines)),
	}
	for i, k := range klines {
		pk := &protocol.Kline{
			Time:   time.Unix(k.Date, 0),
			Open:   k.Open,
			High:   k.High,
			Low:    k.Low,
			Close:  k.Close,
			Volume: k.Volume,
			Amount: k.Amount,
		}
		if i > 0 {
			pk.Last = klines[i-1].Close
		}
		resp.List = append(resp.List, pk)
	}
	return resp, nil
}

func convertToWeekKline(dayKline *protocol.KlineResp) *protocol.KlineResp {
	if len(dayKline.List) == 0 {
		return dayKline
	}

	weekResp := &protocol.KlineResp{List: make([]*protocol.Kline, 0)}
	var currentWeek *protocol.Kline
	var lastWeekDay time.Time

	for _, k := range dayKline.List {
		year, week := k.Time.ISOWeek()
		if currentWeek == nil || lastWeekDay.Year() != year || getISOWeek(lastWeekDay) != week {
			if currentWeek != nil {
				weekResp.List = append(weekResp.List, currentWeek)
			}
			currentWeek = &protocol.Kline{
				Time:   k.Time,
				Last:   k.Last,
				Open:   k.Open,
				High:   k.High,
				Low:    k.Low,
				Close:  k.Close,
				Volume: k.Volume,
				Amount: k.Amount,
			}
		} else {
			if k.High > currentWeek.High {
				currentWeek.High = k.High
			}
			if k.Low < currentWeek.Low || currentWeek.Low == 0 {
				currentWeek.Low = k.Low
			}
			currentWeek.Close = k.Close
			currentWeek.Volume += k.Volume
			currentWeek.Amount += k.Amount
			currentWeek.Time = k.Time
		}
		lastWeekDay = k.Time
	}
	if currentWeek != nil {
		weekResp.List = append(weekResp.List, currentWeek)
	}
	weekResp.Count = uint16(len(weekResp.List))
	return weekResp
}

func convertToMonthKline(dayKline *protocol.KlineResp) *protocol.KlineResp {
	if len(dayKline.List) == 0 {
		return dayKline
	}

	monthResp := &protocol.KlineResp{List: make([]*protocol.Kline, 0)}
	var currentMonth *protocol.Kline
	var lastMonthKey string

	for _, k := range dayKline.List {
		monthKey := k.Time.Format("200601")
		if currentMonth == nil || lastMonthKey != monthKey {
			if currentMonth != nil {
				monthResp.List = append(monthResp.List, currentMonth)
			}
			currentMonth = &protocol.Kline{
				Time:   k.Time,
				Last:   k.Last,
				Open:   k.Open,
				High:   k.High,
				Low:    k.Low,
				Close:  k.Close,
				Volume: k.Volume,
				Amount: k.Amount,
			}
		} else {
			if k.High > currentMonth.High {
				currentMonth.High = k.High
			}
			if k.Low < currentMonth.Low || currentMonth.Low == 0 {
				currentMonth.Low = k.Low
			}
			currentMonth.Close = k.Close
			currentMonth.Volume += k.Volume
			currentMonth.Amount += k.Amount
			currentMonth.Time = k.Time
		}
		lastMonthKey = monthKey
	}
	if currentMonth != nil {
		monthResp.List = append(monthResp.List, currentMonth)
	}
	monthResp.Count = uint16(len(monthResp.List))
	return monthResp
}

func getISOWeek(t time.Time) int {
	_, week := t.ISOWeek()
	return week
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
