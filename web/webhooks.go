package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type WebhookEvent struct {
	Event          string      `json:"event"`
	TaskID         string      `json:"task_id,omitempty"`
	TaskName       string      `json:"task_name,omitempty"`
	TaskType       string      `json:"task_type,omitempty"`
	RunID          string      `json:"run_id,omitempty"`
	Status         string      `json:"status,omitempty"`
	Message        string      `json:"message,omitempty"`
	MatchedCount   int         `json:"matched_count,omitempty"`
	MatchedSymbols []string    `json:"matched_symbols,omitempty"`
	Result         interface{} `json:"result,omitempty"`
	RunAt          string      `json:"run_at"`
}

func sendWebhooks(ctx context.Context, hooks []Webhook, event WebhookEvent) []string {
	if len(hooks) == 0 {
		return nil
	}
	event.RunAt = nowText()
	raw, _ := json.Marshal(event)
	client := &http.Client{Timeout: 20 * time.Second}
	logs := make([]string, 0, len(hooks))

	for _, hook := range hooks {
		if !webhookAllowsEvent(hook, event.Event) {
			continue
		}
		method := strings.ToUpper(hook.Method)
		if method == "" {
			method = http.MethodPost
		}
		req, err := http.NewRequestWithContext(ctx, method, hook.URL, bytes.NewReader(raw))
		if err != nil {
			logs = append(logs, fmt.Sprintf("%s 创建请求失败: %v", hook.Name, err))
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		var headers map[string]string
		if err := json.Unmarshal([]byte(hook.HeadersJSON), &headers); err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			logs = append(logs, fmt.Sprintf("%s 发送失败: %v", hook.Name, err))
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			logs = append(logs, fmt.Sprintf("%s 返回状态: %s", hook.Name, resp.Status))
			continue
		}
		logs = append(logs, fmt.Sprintf("%s 已发送", hook.Name))
	}
	return logs
}

func webhookAllowsEvent(hook Webhook, event string) bool {
	var events []string
	if err := json.Unmarshal([]byte(hook.Events), &events); err != nil {
		return true
	}
	if len(events) == 0 {
		return true
	}
	for _, item := range events {
		if item == "*" || item == event {
			return true
		}
	}
	return false
}
