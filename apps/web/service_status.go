package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type serviceStatus struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Healthy   bool   `json:"healthy"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
}

type serviceStatusResult struct {
	Services  []serviceStatus `json:"services"`
	Ready     bool            `json:"ready"`
	CheckedAt string          `json:"checked_at"`
}

type serviceProbe struct {
	id       string
	name     string
	url      string
	local    bool
}

func serviceURL(envName, fallback string) string {
	if value := strings.TrimRight(strings.TrimSpace(os.Getenv(envName)), "/"); value != "" {
		return value
	}
	return fallback
}

func serviceProbes() []serviceProbe {
	return []serviceProbe{
		{id: "stock-web", name: "页面服务", local: true},
		{id: "market-service", name: "行情服务", url: serviceURL("MARKET_SERVICE_URL", "http://market-service:8081") + "/api/health"},
		{id: "formula-worker", name: "公式服务", url: serviceURL("FORMULA_WORKER_URL", "http://formula-worker:8712") + "/health"},
		{id: "hikyuu-data-service", name: "数据服务", url: serviceURL("HIKYUU_DATA_SERVICE_URL", "http://hikyuu-data-service:8091") + "/api/hikyuu/health"},
		{id: "selection-worker", name: "选股服务", url: serviceURL("SELECTION_WORKER_URL", "http://selection-worker:8082") + "/api/health"},
		{id: "ai-service", name: "AI 服务", url: serviceURL("AI_SERVICE_URL", "http://ai-service:8083") + "/api/health"},
	}
}

func probeService(ctx context.Context, client *http.Client, probe serviceProbe) serviceStatus {
	if probe.local {
		healthy := len(startupWarnings) == 0
		status := "running"
		if !healthy {
			status = "degraded"
		}
		return serviceStatus{
			ID:      probe.id,
			Name:    probe.name,
			Status:  status,
			Healthy: healthy,
		}
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probe.url, nil)
	if err == nil {
		resp, requestErr := client.Do(req)
		if requestErr == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return serviceStatus{
					ID:        probe.id,
					Name:      probe.name,
					Status:    "running",
					Healthy:   true,
					LatencyMS: time.Since(start).Milliseconds(),
				}
			}
		}
	}

	return serviceStatus{
		ID:        probe.id,
		Name:      probe.name,
		Status:    "offline",
		Healthy:   false,
		LatencyMS: time.Since(start).Milliseconds(),
	}
}

func handleServiceStatuses(w http.ResponseWriter, r *http.Request) {
	probes := serviceProbes()
	client := &http.Client{Timeout: 2 * time.Second}
	ctx, cancel := context.WithTimeout(r.Context(), 2500*time.Millisecond)
	defer cancel()

	statuses := make([]serviceStatus, len(probes))
	var waitGroup sync.WaitGroup
	for index, probe := range probes {
		waitGroup.Add(1)
		go func(index int, probe serviceProbe) {
			defer waitGroup.Done()
			statuses[index] = probeService(ctx, client, probe)
		}(index, probe)
	}
	waitGroup.Wait()

	ready := true
	for _, status := range statuses {
		if !status.Healthy {
			ready = false
			break
		}
	}

	successResponse(w, serviceStatusResult{
		Services:  statuses,
		Ready:     ready,
		CheckedAt: time.Now().Format(time.RFC3339),
	})
}
