package main

import (
	"log"
	"net/http"
	"os"
	"strings"
)

var (
	appStore        *AppStore
	startupWarnings []string
)

func main() {
	store, err := OpenAppStore()
	if err != nil {
		log.Fatal(err)
	}
	appStore = store
	defer appStore.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", handleHealthCheck)
	mux.HandleFunc("/api/server-status", handleServerStatus)
	mux.HandleFunc("/api/ai/providers", withServiceAuth(handleAIProviders))
	mux.HandleFunc("/api/ai/models", withServiceAuth(handleAIModels))
	mux.HandleFunc("/api/ai/credentials", withServiceAuth(handleAICredentials))
	mux.HandleFunc("/api/ai/credentials/", withServiceAuth(handleAICredentialOperations))
	mux.HandleFunc("/api/ai/test-connection", withServiceAuth(handleAITestConnection))
	mux.HandleFunc("/api/ai/chat", withServiceAuth(handleAIChat))
	mux.HandleFunc("/api/ai/chat/stream", withServiceAuth(handleAIChatStream))
	mux.HandleFunc("/api/ai/analyze/stock", withServiceAuth(handleAIAnalyzeStock))
	mux.HandleFunc("/api/ai/research/stock", withServiceAuth(handleAIResearchStock))
	mux.HandleFunc("/api/ai/select/rank", withServiceAuth(handleAISelectRank))
	mux.HandleFunc("/api/ai/analyze/watchlist", withServiceAuth(handleAIAnalyzeWatchlist))

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8083"
	}
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}
	log.Printf("AI服务启动成功，访问 http://localhost%s", port)
	log.Fatal(http.ListenAndServe(port, mux))
}
