package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// dispatchMacroAlerts sends only the first occurrence of each alert kind for
// an event and webhook. The delivery claim is persisted before the request so
// concurrent pollers cannot duplicate a notification.
func dispatchMacroAlerts(ctx context.Context) {
	settings, err := appStore.GetMacroAlertSettings()
	if err != nil || !settings.Enabled || !settings.NotifyWebhooks {
		return
	}
	hooks, err := appStore.ResolveWebhooks(settings.WebhookIDs)
	if err != nil || len(hooks) == 0 {
		return
	}
	events, err := appStore.ListMacroEvents("", "", "", "")
	if err != nil {
		return
	}
	now := time.Now()
	for _, event := range events {
		state := workbenchMacroEventAlertState(event, settings, now)
		kind := ""
		switch state {
		case "alert_due":
			kind = "alert_due"
		case "window_active":
			kind = "window_started"
		}
		if kind == "" {
			continue
		}
		pending := make([]Webhook, 0, len(hooks))
		for _, hook := range hooks {
			if !webhookAllowsEvent(hook, "macro_event."+kind) {
				continue
			}
			claimed, claimErr := appStore.ClaimMacroAlertDelivery(event.ID, hook.ID, kind)
			if claimErr == nil && claimed {
				pending = append(pending, hook)
			}
		}
		if len(pending) == 0 {
			continue
		}
		sendWebhooks(ctx, pending, WebhookEvent{
			Event:  "macro_event." + kind,
			Status: state,
			Message: fmt.Sprintf("%s：%s（%s）\n发布时间：%s\n对应 A 股交易日：%s\n来源：%s\n风险提示：事件窗口内请复核交易计划，不会自动阻止交易。",
				map[string]string{"alert_due": "提前提醒", "window_started": "风险窗口开始"}[kind], event.Name, event.Code,
				event.StartsAt, valueOr(event.AShareDate, "待交易日历确认"), valueOr(event.Source, "自定义来源")),
			Result: map[string]interface{}{
				"event_id": event.ID, "code": event.Code, "impact": event.Impact,
				"source_url": event.SourceURL, "a_share_date": event.AShareDate,
			},
		})
	}
}

func workbenchMacroEventAlertState(event MacroEvent, settings MacroAlertSettings, now time.Time) string {
	// Kept in the web package to avoid leaking scheduler concerns into handlers.
	if !settings.Enabled {
		return "disabled"
	}
	if settings.CriticalOnly && event.Impact != "high" && event.Impact != "critical" {
		return "muted"
	}
	startsAt, err := time.Parse(time.RFC3339, event.StartsAt)
	if err != nil {
		return "unknown"
	}
	windowStart := startsAt.Add(-time.Duration(settings.WindowBeforeMinutes) * time.Minute)
	windowEnd := startsAt.Add(time.Duration(settings.WindowAfterMinutes) * time.Minute)
	if !now.Before(windowStart) && now.Before(windowEnd) {
		return "window_active"
	}
	if now.Before(startsAt.Add(-time.Duration(settings.LeadMinutes) * time.Minute)) {
		return "scheduled"
	}
	if now.Before(startsAt) {
		return "alert_due"
	}
	return "released"
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
