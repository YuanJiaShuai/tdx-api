package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	workbench "workbench-core"
)

func handleMacroEventOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	events, err := appStore.ListMacroEvents("", "", "", "")
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	settings, err := appStore.GetMacroAlertSettings()
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	state, _ := appStore.GetTradingSystemState()
	watchPool, _ := appStore.GetStockPool(DecisionWatchPoolID)
	holdingSet := map[string]bool{}
	for _, trade := range state.Trades {
		if strings.EqualFold(trade.Direction, "sell") || trade.Status != "active" {
			continue
		}
		code := strings.TrimSpace(trade.StockCode)
		if code != "" {
			holdingSet[code] = true
		}
	}
	now := time.Now()
	holdingRisk, watchRisk := 0, 0
	activeRisk := make([]MacroEvent, 0)
	for _, event := range events {
		stateName := workbench.MacroEventAlertState(event, settings, now)
		if stateName != "window_active" && stateName != "alert_due" {
			continue
		}
		if len(holdingSet) > 0 {
			holdingRisk++
		}
		if len(watchPool.Symbols) > 0 {
			watchRisk++
		}
		activeRisk = append(activeRisk, event)
	}
	successResponse(w, map[string]interface{}{
		"holding_symbols":       len(holdingSet),
		"watchlist_symbols":     len(watchPool.Symbols),
		"holding_risk_events":   holdingRisk,
		"watchlist_risk_events": watchRisk,
		"active_risk_events":    activeRisk,
		"settings":              settings,
	})
}

func handleMacroEvents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := appStore.ListMacroEvents(
			r.URL.Query().Get("from"),
			r.URL.Query().Get("to"),
			r.URL.Query().Get("category"),
			r.URL.Query().Get("impact"),
		)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, items)
	case http.MethodPost:
		var event MacroEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		item, err := appStore.UpsertMacroEvent(event)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleMacroEventSync(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		states, err := appStore.ListMacroEventSyncStates()
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, map[string]interface{}{"states": states})
	case http.MethodPost:
		result, err := syncMacroCalendars(r.Context())
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		dispatchMacroAlerts(r.Context())
		successResponse(w, result)
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleMacroAlertSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := appStore.GetMacroAlertSettings()
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, settings)
	case http.MethodPut:
		var settings MacroAlertSettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		item, err := appStore.UpsertMacroAlertSettings(settings)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleMacroEventOperations(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/macro-events/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 2 && parts[1] == "acknowledge" {
		if r.Method != http.MethodPost {
			errorResponse(w, "只支持POST请求")
			return
		}
		var req struct {
			Acknowledged bool `json:"acknowledged"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		event, err := appStore.SetMacroEventAcknowledged(id, req.Acknowledged)
		if err != nil {
			errorResponse(w, notFoundMessage(err, "事件不存在"))
			return
		}
		successResponse(w, event)
		return
	}

	switch r.Method {
	case http.MethodGet:
		event, err := appStore.GetMacroEvent(id)
		if err != nil {
			errorResponse(w, notFoundMessage(err, "事件不存在"))
			return
		}
		successResponse(w, event)
	case http.MethodPut:
		if _, err := appStore.GetMacroEvent(id); err != nil {
			errorResponse(w, notFoundMessage(err, "事件不存在"))
			return
		}
		var event MacroEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		event.ID = id
		item, err := appStore.UpsertMacroEvent(event)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
	case http.MethodDelete:
		if err := appStore.DeleteMacroEvent(id); err != nil {
			errorResponse(w, notFoundMessage(err, "事件不存在"))
			return
		}
		successResponse(w, map[string]string{"id": id})
	default:
		errorResponse(w, "不支持的请求方法")
	}
}
