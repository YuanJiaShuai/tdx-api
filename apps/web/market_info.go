package main

import "net/http"

// Market information is served by market-service in the deployed topology.
// Keep explicit fallbacks here so the web binary also builds in standalone mode.
func handleMarketResearch(w http.ResponseWriter, r *http.Request) {
	errorResponse(w, "该接口需要配置MARKET_SERVICE_URL")
}

func handleMarketNotice(w http.ResponseWriter, r *http.Request) {
	errorResponse(w, "该接口需要配置MARKET_SERVICE_URL")
}

func handleMarketHotMoney(w http.ResponseWriter, r *http.Request) {
	errorResponse(w, "该接口需要配置MARKET_SERVICE_URL")
}
