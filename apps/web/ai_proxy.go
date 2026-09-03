package main

import "net/http"

func registerAIProxyRoutes() {
	handler := proxyToService("AI_SERVICE_URL", func(w http.ResponseWriter, r *http.Request) {
		errorResponse(w, "AI_SERVICE_URL未配置")
	})
	http.HandleFunc("/api/ai/providers", handler)
	http.HandleFunc("/api/ai/models", handler)
	http.HandleFunc("/api/ai/credentials", handler)
	http.HandleFunc("/api/ai/credentials/", handler)
	http.HandleFunc("/api/ai/test-connection", handler)
	http.HandleFunc("/api/ai/chat", handler)
	http.HandleFunc("/api/ai/chat/stream", handler)
	http.HandleFunc("/api/ai/analyze/stock", handler)
	http.HandleFunc("/api/ai/research/stock", handler)
	http.HandleFunc("/api/ai/select/rank", handler)
	http.HandleFunc("/api/ai/analyze/watchlist", handler)
}
