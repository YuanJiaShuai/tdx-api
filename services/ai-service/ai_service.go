package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	AIProviderOpenAICompatible = "openai-compatible"
	AIProviderOpenAI           = "openai"
	AIProviderDeepSeek         = "deepseek"
	AIProviderQwen             = "qwen"
	AIProviderZhipu            = "zhipu"
)

type AIProviderInfo struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Adapter        string    `json:"adapter"`
	DefaultBaseURL string    `json:"default_base_url"`
	DefaultModels  []AIModel `json:"default_models"`
}

type AIModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIRequestOptions struct {
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
	JSONMode    bool     `json:"json_mode,omitempty"`
}

type AIChatRequest struct {
	Provider     string           `json:"provider"`
	Model        string           `json:"model"`
	CredentialID string           `json:"credential_id"`
	Messages     []AIMessage      `json:"messages"`
	Options      AIRequestOptions `json:"options"`
}

type AIChatResponse struct {
	Provider    string                 `json:"provider"`
	Model       string                 `json:"model"`
	Content     string                 `json:"content"`
	Usage       map[string]interface{} `json:"usage"`
	LatencyMS   int64                  `json:"latency_ms"`
	Credential  string                 `json:"credential"`
	GeneratedAt string                 `json:"generated_at"`
}

type AIAnalyzeRequest struct {
	Provider     string                 `json:"provider"`
	Model        string                 `json:"model"`
	CredentialID string                 `json:"credential_id"`
	Symbol       string                 `json:"symbol"`
	PoolID       string                 `json:"pool_id"`
	Symbols      []string               `json:"symbols"`
	Input        map[string]interface{} `json:"input"`
	Options      AIRequestOptions       `json:"options"`
	Research     bool                   `json:"research,omitempty"`
}

type AIAnalyzeResponse struct {
	RunID         string                 `json:"run_id"`
	TaskType      string                 `json:"task_type"`
	Provider      string                 `json:"provider"`
	Model         string                 `json:"model"`
	Content       string                 `json:"content"`
	Result        map[string]interface{} `json:"result"`
	Usage         map[string]interface{} `json:"usage"`
	LatencyMS     int64                  `json:"latency_ms"`
	Input         map[string]interface{} `json:"input"`
	GeneratedAt   string                 `json:"generated_at"`
	PromptVersion string                 `json:"prompt_version"`
	DataRevision  string                 `json:"data_revision"`
	ToolsUsed     []string               `json:"tools_used"`
}

type AIProviderClient interface {
	Chat(ctx context.Context, req AIChatRequest, credential AICredential) (AIChatResponse, error)
	Test(ctx context.Context, req AIChatRequest, credential AICredential) error
	Stream(ctx context.Context, req AIChatRequest, credential AICredential, emit func(map[string]interface{}) error) error
}

type OpenAICompatibleClient struct {
	httpClient *http.Client
}

type openAICompatibleResponse struct {
	Choices []struct {
		Message AIMessage `json:"message"`
	} `json:"choices"`
	Usage map[string]interface{} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func NewOpenAICompatibleClient() *OpenAICompatibleClient {
	return &OpenAICompatibleClient{
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *OpenAICompatibleClient) Chat(ctx context.Context, req AIChatRequest, credential AICredential) (AIChatResponse, error) {
	provider := normalizeAIProvider(req.Provider)
	model := chooseAIModel(provider, chooseNonEmpty(req.Model, credential.Model))
	baseURL := chooseAIBaseURL(provider, credential.BaseURL)
	if err := requireNonEmpty(credential.APIKey, "AI API key未配置"); err != nil {
		return AIChatResponse{}, err
	}
	if len(req.Messages) == 0 {
		return AIChatResponse{}, errors.New("messages不能为空")
	}
	if baseURL == "" {
		return AIChatResponse{}, errors.New("AI base URL未配置")
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": req.Messages,
	}
	if req.Options.Temperature != nil {
		body["temperature"] = *req.Options.Temperature
	}
	if req.Options.MaxTokens > 0 {
		body["max_tokens"] = req.Options.MaxTokens
	}
	if req.Options.JSONMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	rawBody, err := json.Marshal(body)
	if err != nil {
		return AIChatResponse{}, err
	}

	start := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(rawBody))
	if err != nil {
		return AIChatResponse{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+credential.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return AIChatResponse{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return AIChatResponse{}, err
	}

	var parsed openAICompatibleResponse
	_ = json.Unmarshal(respBody, &parsed)
	if resp.StatusCode >= 300 {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return AIChatResponse{}, fmt.Errorf("AI请求失败: %s", parsed.Error.Message)
		}
		return AIChatResponse{}, fmt.Errorf("AI请求失败: %s %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return AIChatResponse{}, errors.New(parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return AIChatResponse{}, errors.New("AI响应为空")
	}
	return AIChatResponse{
		Provider:    provider,
		Model:       model,
		Content:     parsed.Choices[0].Message.Content,
		Usage:       parsed.Usage,
		LatencyMS:   time.Since(start).Milliseconds(),
		Credential:  credential.ID,
		GeneratedAt: nowText(),
	}, nil
}

func (c *OpenAICompatibleClient) Stream(ctx context.Context, req AIChatRequest, credential AICredential, emit func(map[string]interface{}) error) error {
	provider := normalizeAIProvider(req.Provider)
	model := chooseAIModel(provider, chooseNonEmpty(req.Model, credential.Model))
	baseURL := chooseAIBaseURL(provider, credential.BaseURL)
	if err := requireNonEmpty(credential.APIKey, "AI API key未配置"); err != nil {
		return err
	}
	if len(req.Messages) == 0 {
		return errors.New("messages不能为空")
	}
	if baseURL == "" {
		return errors.New("AI base URL未配置")
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": req.Messages,
		"stream":   true,
	}
	if req.Options.Temperature != nil {
		body["temperature"] = *req.Options.Temperature
	}
	if req.Options.MaxTokens > 0 {
		body["max_tokens"] = req.Options.MaxTokens
	}
	rawBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(rawBody))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+credential.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		rawResponse, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("AI请求失败: %s", resp.Status)
		}
		var parsed openAICompatibleResponse
		_ = json.Unmarshal(rawResponse, &parsed)
		if parsed.Error != nil && parsed.Error.Message != "" {
			return fmt.Errorf("AI请求失败: %s", parsed.Error.Message)
		}
		return fmt.Errorf("AI请求失败: %s %s", resp.Status, strings.TrimSpace(string(rawResponse)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			return nil
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string                   `json:"content"`
					ReasoningContent string                   `json:"reasoning_content"`
					Reasoning        string                   `json:"reasoning"`
					ToolCalls        []map[string]interface{} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return errors.New(chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		event := map[string]interface{}{}
		if delta.Content != "" {
			event["content"] = delta.Content
		}
		if delta.ReasoningContent != "" {
			event["reasoning_content"] = delta.ReasoningContent
		} else if delta.Reasoning != "" {
			event["reasoning_content"] = delta.Reasoning
		}
		if len(delta.ToolCalls) > 0 {
			event["tool_calls"] = delta.ToolCalls
		}
		if len(event) > 0 {
			if err := emit(event); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func (c *OpenAICompatibleClient) Test(ctx context.Context, req AIChatRequest, credential AICredential) error {
	req.Messages = []AIMessage{
		{Role: "system", Content: "You are a connection test endpoint. Reply with JSON only."},
		{Role: "user", Content: `Return {"ok":true}.`},
	}
	req.Options.JSONMode = true
	req.Options.MaxTokens = 32
	_, err := c.Chat(ctx, req, credential)
	return err
}

func aiProviderInfos() []AIProviderInfo {
	return []AIProviderInfo{
		{
			ID:             AIProviderDeepSeek,
			Name:           "DeepSeek",
			Adapter:        AIProviderOpenAICompatible,
			DefaultBaseURL: "https://api.deepseek.com",
			DefaultModels: []AIModel{
				{ID: "deepseek-chat", Name: "DeepSeek Chat", Description: "通用分析模型"},
				{ID: "deepseek-reasoner", Name: "DeepSeek Reasoner", Description: "推理增强模型"},
			},
		},
		{
			ID:             AIProviderOpenAI,
			Name:           "OpenAI",
			Adapter:        AIProviderOpenAICompatible,
			DefaultBaseURL: "https://api.openai.com/v1",
			DefaultModels: []AIModel{
				{ID: "gpt-4.1-mini", Name: "GPT-4.1 mini", Description: "通用低延迟模型"},
				{ID: "gpt-4.1", Name: "GPT-4.1", Description: "通用高质量模型"},
			},
		},
		{
			ID:             AIProviderQwen,
			Name:           "通义千问",
			Adapter:        AIProviderOpenAICompatible,
			DefaultBaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
			DefaultModels: []AIModel{
				{ID: "qwen-plus", Name: "Qwen Plus", Description: "兼容模式通用模型"},
				{ID: "qwen-turbo", Name: "Qwen Turbo", Description: "兼容模式低延迟模型"},
			},
		},
		{
			ID:             AIProviderZhipu,
			Name:           "智谱 GLM",
			Adapter:        AIProviderOpenAICompatible,
			DefaultBaseURL: "https://open.bigmodel.cn/api/paas/v4",
			DefaultModels: []AIModel{
				{ID: "glm-4-flash", Name: "GLM-4 Flash", Description: "兼容模式通用模型"},
				{ID: "glm-4-plus", Name: "GLM-4 Plus", Description: "兼容模式高质量模型"},
			},
		},
		{
			ID:            AIProviderOpenAICompatible,
			Name:          "自定义 OpenAI-compatible",
			Adapter:       AIProviderOpenAICompatible,
			DefaultModels: []AIModel{},
		},
	}
}

func handleAIProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	successResponse(w, aiProviderInfos())
}

func handleAIModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, "只支持GET请求")
		return
	}
	provider := normalizeAIProvider(r.URL.Query().Get("provider"))
	for _, item := range aiProviderInfos() {
		if item.ID == provider {
			successResponse(w, item.DefaultModels)
			return
		}
	}
	errorResponse(w, "provider不存在")
}

func handleAICredentials(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := appStore.ListAICredentials()
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, append(envAICredentials(), items...))
	case http.MethodPost:
		var req AICredential
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		item, err := appStore.UpsertAICredential(req)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleAICredentialOperations(w http.ResponseWriter, r *http.Request) {
	parts := pathParts(r.URL.Path, "/api/ai/credentials/")
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 2 && parts[1] == "test" {
		handleAITestConnectionWithCredential(w, r, id)
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := resolveAICredential(id, "")
		if err != nil {
			errorResponse(w, notFoundMessage(err, "AI凭据不存在"))
			return
		}
		item.APIKey = ""
		item.APISecret = ""
		successResponse(w, item)
	case http.MethodPut:
		var req AICredential
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, "请求参数错误: "+err.Error())
			return
		}
		req.ID = id
		item, err := appStore.UpsertAICredential(req)
		if err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, item)
	case http.MethodDelete:
		if strings.HasPrefix(id, "env:") {
			errorResponse(w, "环境变量凭据不能删除")
			return
		}
		if err := appStore.DeleteAICredential(id); err != nil {
			errorResponse(w, err.Error())
			return
		}
		successResponse(w, map[string]string{"id": id})
	default:
		errorResponse(w, "不支持的请求方法")
	}
}

func handleAITestConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, "只支持POST请求")
		return
	}
	var req AIChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, "请求参数错误: "+err.Error())
		return
	}
	credential, err := resolveAICredential(req.CredentialID, req.Provider)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	if req.Provider == "" {
		req.Provider = credential.Provider
	}
	if err := aiClientFor(req.Provider).Test(r.Context(), req, credential); err != nil {
		errorResponse(w, err.Error())
		return
	}
	successResponse(w, map[string]interface{}{
		"ok":         true,
		"provider":   normalizeAIProvider(req.Provider),
		"model":      chooseAIModel(req.Provider, chooseNonEmpty(req.Model, credential.Model)),
		"credential": credential.ID,
	})
}

func handleAITestConnectionWithCredential(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		errorResponse(w, "只支持POST请求")
		return
	}
	var req AIChatRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	req.CredentialID = id
	credential, err := resolveAICredential(id, req.Provider)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	if req.Provider == "" {
		req.Provider = credential.Provider
	}
	if err := aiClientFor(req.Provider).Test(r.Context(), req, credential); err != nil {
		errorResponse(w, err.Error())
		return
	}
	successResponse(w, map[string]interface{}{"ok": true, "provider": req.Provider, "model": chooseAIModel(req.Provider, chooseNonEmpty(req.Model, credential.Model)), "credential": id})
}

func handleAIChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, "只支持POST请求")
		return
	}
	var req AIChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, "请求参数错误: "+err.Error())
		return
	}
	credential, err := resolveAICredential(req.CredentialID, req.Provider)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	if req.Provider == "" {
		req.Provider = credential.Provider
	}
	resp, err := aiClientFor(req.Provider).Chat(r.Context(), req, credential)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	successResponse(w, resp)
}

func handleAIChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, "只支持POST请求")
		return
	}
	var req AIChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, "请求参数错误: "+err.Error())
		return
	}
	credential, err := resolveAICredential(req.CredentialID, req.Provider)
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	if req.Provider == "" {
		req.Provider = credential.Provider
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		errorResponse(w, "流式响应不受支持")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	emit := func(event map[string]interface{}) error {
		raw, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			return marshalErr
		}
		if _, writeErr := fmt.Fprintf(w, "data: %s\n\n", raw); writeErr != nil {
			return writeErr
		}
		flusher.Flush()
		return nil
	}
	if err := aiClientFor(req.Provider).Stream(r.Context(), req, credential, emit); err != nil {
		raw, _ := json.Marshal(map[string]string{"message": err.Error()})
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", raw)
		flusher.Flush()
		return
	}
	_, _ = w.Write([]byte("event: done\ndata: [DONE]\n\n"))
	flusher.Flush()
}

func handleAIAnalyzeStock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, "只支持POST请求")
		return
	}
	var req AIAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, "请求参数错误: "+err.Error())
		return
	}
	symbols := normalizeSymbols([]string{req.Symbol})
	if len(symbols) == 0 {
		errorResponse(w, "symbol不能为空")
		return
	}
	req.Symbol = symbols[0]
	input := cloneAIInput(req.Input)
	input["symbol"] = req.Symbol
	if marketData, err := fetchAnalysisContext(r.Context(), []string{req.Symbol}); err == nil {
		input["market_data"] = firstAnalysisItem(marketData)
	} else {
		input["market_data_error"] = err.Error()
	}
	resp, err := runAIAnalysis(r.Context(), "stock_analysis", req, input, stockAnalysisMessages(req.Symbol, input))
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	successResponse(w, resp)
}

func handleAIResearchStock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, "只支持POST请求")
		return
	}
	var req AIAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, "请求参数错误: "+err.Error())
		return
	}
	symbols := normalizeSymbols([]string{req.Symbol})
	if len(symbols) == 0 {
		errorResponse(w, "symbol不能为空")
		return
	}
	req.Symbol = symbols[0]
	req.Research = true
	input := cloneAIInput(req.Input)
	input["symbol"] = req.Symbol
	input["report_type"] = "single_stock_research"
	input["prompt_version"] = "research-v1"
	if marketData, err := fetchAnalysisContext(r.Context(), []string{req.Symbol}); err == nil {
		input["market_data"] = firstAnalysisItem(marketData)
	} else {
		input["market_data_error"] = err.Error()
	}
	enrichResearchInput(r.Context(), req.Symbol, input)
	resp, err := runAIAnalysis(r.Context(), "stock_research_report", req, input, stockResearchMessages(req.Symbol, input))
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	successResponse(w, resp)
}

func handleAIAnalyzeWatchlist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, "只支持POST请求")
		return
	}
	var req AIAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, "请求参数错误: "+err.Error())
		return
	}
	symbols := normalizeSymbols(req.Symbols)
	if len(symbols) == 0 {
		errorResponse(w, "symbols不能为空，股票池由调用方解析后传入")
		return
	}
	if len(symbols) > 20 {
		symbols = symbols[:20]
	}
	input := cloneAIInput(req.Input)
	input["pool_id"] = req.PoolID
	input["symbols"] = symbols
	stocks := make([]map[string]interface{}, 0, len(symbols))
	if marketData, err := fetchAnalysisContext(r.Context(), symbols); err == nil {
		for _, item := range analysisItemsBySymbol(marketData) {
			stocks = append(stocks, item)
		}
	} else {
		for _, symbol := range symbols {
			stocks = append(stocks, map[string]interface{}{"symbol": symbol, "market_data_error": err.Error()})
		}
	}
	input["stocks"] = stocks
	resp, err := runAIAnalysis(r.Context(), "watchlist_analysis", req, input, watchlistAnalysisMessages(input))
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	successResponse(w, resp)
}

func handleAISelectRank(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, "只支持POST请求")
		return
	}
	var req AIAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, "请求参数错误: "+err.Error())
		return
	}
	symbols := normalizeSymbols(req.Symbols)
	if len(symbols) == 0 {
		errorResponse(w, "symbols不能为空")
		return
	}
	if len(symbols) > 20 {
		symbols = symbols[:20]
	}
	input := cloneAIInput(req.Input)
	input["symbols"] = symbols
	input["prompt_version"] = "selection-v1"
	input["methodology"] = "hikyuu_ma_cross_reference"
	tools := []string{}
	if marketData, err := fetchAnalysisContext(r.Context(), symbols); err == nil {
		input["market_data"] = analysisItemsBySymbol(marketData)
		tools = append(tools, "market_context")
	} else {
		input["market_data_error"] = err.Error()
	}
	baseURL := hikyuuServiceURL()
	if baseURL == "" {
		input["hikyuu_error"] = "HIKYUU_DATA_SERVICE_URL未配置"
	} else {
		metadataRaw := map[string]interface{}{}
		if err := fetchJSON(r.Context(), http.MethodGet, baseURL+"/api/hikyuu/metadata", nil, &metadataRaw); err == nil {
			metadata := unwrapToolData(metadataRaw)
			input["hikyuu"] = metadata
			if item, ok := metadata.(map[string]interface{}); ok {
				input["data_revision"] = stringValue(item, "data_revision", "")
			}
			tools = append(tools, "hikyuu_metadata")
		} else {
			input["hikyuu_metadata_error"] = err.Error()
		}
		fast := intValue(input, "fast_period", 5)
		slow := intValue(input, "slow_period", 20)
		historyCount := intValue(input, "history_count", 520)
		backtestRaw := map[string]interface{}{}
		payload := map[string]interface{}{"symbols": symbols, "fast": fast, "slow": slow, "history_count": historyCount, "type": "day"}
		if err := fetchJSON(r.Context(), http.MethodPost, baseURL+"/api/hikyuu/backtest", payload, &backtestRaw); err == nil {
			input["historical_validation"] = compactBacktest(unwrapToolData(backtestRaw))
			tools = append(tools, "hikyuu_ma_cross_backtest")
		} else {
			input["historical_validation_error"] = err.Error()
		}
	}
	input["tools_used"] = tools
	resp, err := runAIAnalysis(r.Context(), "stock_selection_ranking", req, input, selectionRankingMessages(input))
	if err != nil {
		errorResponse(w, err.Error())
		return
	}
	successResponse(w, resp)
}

func compactBacktest(value interface{}) interface{} {
	item, ok := value.(map[string]interface{})
	if !ok {
		return value
	}
	result := map[string]interface{}{}
	for _, key := range []string{"engine", "calculation_engine", "symbols", "signals", "per_symbol", "metrics", "warnings", "meta"} {
		if field, exists := item[key]; exists {
			result[key] = field
		}
	}
	return result
}

func runAIAnalysis(ctx context.Context, taskType string, req AIAnalyzeRequest, input map[string]interface{}, messages []AIMessage) (AIAnalyzeResponse, error) {
	credential, err := resolveAICredential(req.CredentialID, req.Provider)
	if err != nil {
		return AIAnalyzeResponse{}, err
	}
	provider := normalizeAIProvider(req.Provider)
	if provider == "" {
		provider = credential.Provider
	}
	model := chooseAIModel(provider, chooseNonEmpty(req.Model, credential.Model))
	options := req.Options
	options.JSONMode = true
	if options.MaxTokens == 0 {
		options.MaxTokens = 1600
	}
	chatResp, chatErr := aiClientFor(provider).Chat(ctx, AIChatRequest{
		Provider:     provider,
		Model:        model,
		CredentialID: credential.ID,
		Messages:     messages,
		Options:      options,
	}, credential)

	status := "success"
	errorText := ""
	content := ""
	latencyMS := int64(0)
	usage := map[string]interface{}{}
	result := map[string]interface{}{}
	if chatErr != nil {
		status = "failed"
		errorText = chatErr.Error()
	} else {
		content = chatResp.Content
		latencyMS = chatResp.LatencyMS
		usage = chatResp.Usage
		_ = json.Unmarshal([]byte(content), &result)
	}
	toolsUsed := input["tools_used"]
	if toolsUsed == nil {
		toolsUsed = []string{}
	}
	input["tools_used"] = toolsUsed
	run, saveErr := appStore.SaveAIAnalysisRun(AIAnalysisRun{
		TaskType:       taskType,
		Provider:       provider,
		Model:          model,
		InputJSON:      mustJSON(input),
		ResultJSON:     mustJSON(result),
		Status:         status,
		Error:          errorText,
		LatencyMS:      latencyMS,
		TokenUsageJSON: mustJSON(usage),
		PromptVersion:  stringValue(input, "prompt_version", "v1"),
		DataRevision:   stringValue(input, "data_revision", ""),
		ToolsJSON:      mustJSON(toolsUsed),
	})
	if chatErr != nil {
		return AIAnalyzeResponse{}, chatErr
	}
	if saveErr != nil {
		return AIAnalyzeResponse{}, saveErr
	}
	return AIAnalyzeResponse{
		RunID:         run.ID,
		TaskType:      taskType,
		Provider:      provider,
		Model:         model,
		Content:       content,
		Result:        result,
		Usage:         usage,
		LatencyMS:     latencyMS,
		Input:         input,
		GeneratedAt:   run.CreatedAt,
		PromptVersion: run.PromptVersion,
		DataRevision:  run.DataRevision,
		ToolsUsed:     decodeStringSlice(input["tools_used"]),
	}, nil
}

func decodeStringSlice(value interface{}) []string {
	items, ok := value.([]string)
	if ok {
		return items
	}
	values, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, item := range values {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			result = append(result, text)
		}
	}
	return result
}

func stringValue(input map[string]interface{}, key, fallback string) string {
	if input == nil {
		return fallback
	}
	if value, ok := input[key].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func intValue(input map[string]interface{}, key string, fallback int) int {
	if input == nil {
		return fallback
	}
	switch value := input[key].(type) {
	case float64:
		if value > 0 {
			return int(value)
		}
	case int:
		if value > 0 {
			return value
		}
	}
	return fallback
}

func fetchJSON(ctx context.Context, method, endpoint string, payload interface{}, output interface{}) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("研究工具返回 %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return err
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	if code, ok := envelope["code"].(float64); ok && code != 0 {
		message := strings.TrimSpace(fmt.Sprint(envelope["message"]))
		if message == "" {
			message = "工具返回失败"
		}
		return errors.New(message)
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return err
	}
	return nil
}

func hikyuuServiceURL() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("HIKYUU_DATA_SERVICE_URL")), "/")
}

func unwrapToolData(raw map[string]interface{}) interface{} {
	if data, ok := raw["data"]; ok {
		return data
	}
	return raw
}

func enrichResearchInput(ctx context.Context, symbol string, input map[string]interface{}) {
	baseURL := hikyuuServiceURL()
	if baseURL == "" {
		input["hikyuu_error"] = "HIKYUU_DATA_SERVICE_URL未配置"
		return
	}
	tools := []string{"market_context"}
	metadataRaw := map[string]interface{}{}
	if err := fetchJSON(ctx, http.MethodGet, baseURL+"/api/hikyuu/metadata", nil, &metadataRaw); err != nil {
		input["hikyuu_error"] = err.Error()
	} else {
		metadata := unwrapToolData(metadataRaw)
		input["hikyuu"] = metadata
		if item, ok := metadata.(map[string]interface{}); ok {
			input["data_revision"] = stringValue(item, "data_revision", "")
		}
		tools = append(tools, "hikyuu_metadata")
	}

	qualityRaw := map[string]interface{}{}
	qualityURL := baseURL + "/api/hikyuu/quality?code=" + url.QueryEscape(symbol) + "&period=day"
	if err := fetchJSON(ctx, http.MethodGet, qualityURL, nil, &qualityRaw); err != nil {
		input["hikyuu_quality_error"] = err.Error()
	} else {
		input["hikyuu_quality"] = unwrapToolData(qualityRaw)
		tools = append(tools, "hikyuu_quality")
	}

	indicators := map[string]interface{}{}
	indicatorSuccess := map[string]bool{}
	indicatorNames := []string{"ma", "ema", "macd", "boll", "atr"}
	type indicatorResult struct {
		name string
		data interface{}
		err  error
	}
	results := make(chan indicatorResult, len(indicatorNames))
	var group sync.WaitGroup
	for _, name := range indicatorNames {
		group.Add(1)
		go func(indicatorName string) {
			defer group.Done()
			payload := map[string]interface{}{"code": symbol, "type": "day", "indicator": indicatorName, "limit": 240, "recover": "none"}
			resultRaw := map[string]interface{}{}
			err := fetchJSON(ctx, http.MethodPost, baseURL+"/api/hikyuu/indicators", payload, &resultRaw)
			results <- indicatorResult{name: indicatorName, data: unwrapToolData(resultRaw), err: err}
		}(name)
	}
	group.Wait()
	close(results)
	for result := range results {
		if result.err != nil {
			input["hikyuu_"+result.name+"_error"] = result.err.Error()
			continue
		}
		indicators[result.name] = result.data
		indicatorSuccess[result.name] = true
	}
	for _, name := range indicatorNames {
		if indicatorSuccess[name] {
			tools = append(tools, "hikyuu_"+name)
		}
	}
	if len(indicators) > 0 {
		input["hikyuu_indicators"] = indicators
	}
	input["tools_used"] = tools
}

func stockResearchMessages(symbol string, input map[string]interface{}) []AIMessage {
	system := `你是本地A股研究工作台的审慎研究员。请基于输入数据生成单股研究报告，不提供确定性买卖建议，不承诺收益。

必须只输出一个JSON对象，严格使用以下结构：
{"summary":"","confidence":"low|medium|high","facts":[{"label":"","value":"","source":""}],"technical":{"trend":"up|down|sideways|unknown","signals":[{"name":"","value":"","evidence":[]}]},"fundamental":{"summary":"","evidence":[]},"macro_risk":{"level":"low|medium|high|unknown","events":[]},"strategy_fit":{"status":"match|mismatch|unknown","reason":""},"evidence":[{"claim":"","evidence":[],"source":""}],"data_quality":{"status":"pass|warn|unknown","notes":[]},"next_checks":[],"discipline_notes":[],"disclaimer":"仅供学习研究和复盘，不构成投资建议"}

严格区分事实、指标计算结果和AI判断：facts只写输入中的事实；technical只解释输入中的指标；evidence必须引用具体输入字段或工具名称。禁止编造行情、财务、宏观事件、新闻或策略回测结果。缺少数据时写unknown，并在data_quality.notes说明。macro_risk只能使用输入中的宏观事件，未提供时返回unknown。`
	user := fmt.Sprintf("请为标的 %s 生成研究报告。输入数据如下：\n%s\n\n只输出JSON。", symbol, mustJSON(input))
	return []AIMessage{{Role: "system", Content: system}, {Role: "user", Content: user}}
}

func selectionRankingMessages(input map[string]interface{}) []AIMessage {
	system := `你是A股候选筛选助手。候选股票已经由确定性公式产生，Hikyuu提供统一的MA交叉参考回测。你的职责仅是根据输入证据排序和解释，不得创造数据或把历史胜率描述成未来成功概率。

必须只输出一个JSON对象，结构如下：
{"summary":"","methodology":"hikyuu_ma_cross_reference","ranking":[{"rank":1,"symbol":"","name":"","status":"candidate|watch|exclude","score":0,"historical_validation":{"sample_count":0,"win_rate":0,"average_return":0,"max_drawdown":0},"reasons":[],"risks":[],"next_checks":[]}],"data_quality":{"status":"pass|warn|unknown","notes":[]},"discipline_notes":[],"disclaimer":"仅供学习研究和复盘，不构成投资建议"}

规则：ranking只能包含输入symbols；历史统计必须逐字使用historical_validation.per_symbol中的数值；没有样本或验证失败时不得填写推测值，status应为watch或exclude并在risks说明；score只是候选集内的相对排序分，不是概率；优先考虑样本量、历史胜率、平均收益、最大回撤和当前行情风险；相同证据下保持保守。`
	user := fmt.Sprintf("请对候选股票进行排序。输入数据如下：\n%s\n\n只输出JSON。", mustJSON(input))
	return []AIMessage{{Role: "system", Content: system}, {Role: "user", Content: user}}
}

func stockAnalysisMessages(symbol string, input map[string]interface{}) []AIMessage {
	if task, _ := input["task"].(string); task == "trading_levels" {
		return []AIMessage{
			{Role: "system", Content: aiAnalysisSystemPrompt() + `

当前任务是 trading_levels。你必须在JSON中额外返回：
invalid_price: 技术无效点，必须是明确的数值；
first_observation_level: 第一观察/压力位，必须是明确的价格或价格区间，例如"42.60-43.30"；
strong_pressure_level: 强压力/止盈区，必须是明确的价格或价格区间，例如"45.00-46.00"；
level_basis: 用一句话说明区间依据；
invalid_point_basis: 用一句话说明技术无效点依据。
只填写基于输入行情数据推导出的价格，不要返回百分比、模糊形容词或确定性买卖结论。`},
			{Role: "user", Content: fmt.Sprintf("请为标的 %s 生成交易计划中的两个价格区间。输入数据如下：\n%s\n\n只输出JSON。", symbol, mustJSON(input))},
		}
	}
	return []AIMessage{
		{Role: "system", Content: aiAnalysisSystemPrompt()},
		{Role: "user", Content: fmt.Sprintf("请分析A股标的 %s。输入数据如下：\n%s\n\n只输出JSON。", symbol, mustJSON(input))},
	}
}

func watchlistAnalysisMessages(input map[string]interface{}) []AIMessage {
	return []AIMessage{
		{Role: "system", Content: aiAnalysisSystemPrompt()},
		{Role: "user", Content: fmt.Sprintf("请分析这个自选/观察列表，找出优先观察对象、风险对象和下一交易日检查项。除系统字段外，请额外返回 stock_analyses 数组，每项包含 symbol、name、summary、strength、risk、next_check。输入数据如下：\n%s\n\n只输出JSON。", mustJSON(input))},
	}
}

func aiAnalysisSystemPrompt() string {
	return `你是一个本地A股交易研究工作台的分析助手。你的任务是帮助用户复盘数据、解释信号、列出风险和观察条件，不提供确定性买卖建议，不承诺收益。

输出必须是JSON对象，字段包括：
summary: 一句话摘要；
bullish_points: 看多或改善因素数组；
risk_points: 风险因素数组；
watch_levels: 需要观察的价格、均线、成交量或时间条件数组；
next_checks: 下一交易日需要检查的事项数组；
discipline_notes: 交易纪律提醒数组；
confidence: low/medium/high；
disclaimer: 固定写"仅供学习研究和复盘，不构成投资建议"。`
}

func fetchAnalysisContext(ctx context.Context, symbols []string) (map[string]interface{}, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("MARKET_SERVICE_URL")), "/")
	if baseURL == "" {
		return nil, errors.New("MARKET_SERVICE_URL未配置")
	}
	query := url.Values{}
	query.Set("codes", strings.Join(symbols, ","))
	query.Set("kline_limit", "120")
	query.Set("news_limit", "10")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/analysis/context?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("行情服务返回: %s", resp.Status)
	}
	var wrapped Response
	if err := json.NewDecoder(resp.Body).Decode(&wrapped); err != nil {
		return nil, err
	}
	if wrapped.Code != 0 {
		if wrapped.Message == "" {
			wrapped.Message = "行情服务返回失败"
		}
		return nil, errors.New(wrapped.Message)
	}
	raw, _ := json.Marshal(wrapped.Data)
	data := map[string]interface{}{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func firstAnalysisItem(context map[string]interface{}) map[string]interface{} {
	items, _ := context["items"].([]interface{})
	if len(items) == 0 {
		return context
	}
	item, _ := items[0].(map[string]interface{})
	if item == nil {
		return context
	}
	return item
}

func analysisItemsBySymbol(context map[string]interface{}) []map[string]interface{} {
	items, _ := context["items"].([]interface{})
	result := make([]map[string]interface{}, 0, len(items))
	for _, raw := range items {
		item, _ := raw.(map[string]interface{})
		if item != nil {
			result = append(result, item)
		}
	}
	return result
}

func cloneAIInput(input map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for key, value := range input {
		result[key] = value
	}
	return result
}

func resolveAICredential(id, provider string) (AICredential, error) {
	provider = normalizeAIProvider(provider)
	if strings.HasPrefix(id, "env:") {
		for _, credential := range envAICredentials() {
			if credential.ID == id {
				credential.APIKey = envAIKey(credential.Provider)
				credential.APISecret = envAISecret(credential.Provider)
				return credential, nil
			}
		}
		return AICredential{}, errors.New("环境变量凭据不存在")
	}
	if id != "" {
		return appStore.GetAICredential(id)
	}
	if provider != "" {
		if credential, err := appStore.FindEnabledAICredential(provider); err == nil {
			return credential, nil
		}
	}
	for _, credential := range envAICredentials() {
		if provider == "" || credential.Provider == provider {
			credential.APIKey = envAIKey(credential.Provider)
			credential.APISecret = envAISecret(credential.Provider)
			return credential, nil
		}
	}
	if provider == "" {
		provider = AIProviderDeepSeek
	}
	return AICredential{}, fmt.Errorf("%s 凭据未配置", provider)
}

func envAICredentials() []AICredential {
	providers := []string{AIProviderDeepSeek, AIProviderOpenAI, AIProviderQwen, AIProviderZhipu}
	items := []AICredential{}
	for _, provider := range providers {
		key := envAIKey(provider)
		if key == "" {
			continue
		}
		baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv(envPrefix(provider)+"_BASE_URL")), "/")
		if baseURL == "" {
			baseURL = chooseAIBaseURL(provider, "")
		}
		items = append(items, AICredential{
			ID:              "env:" + provider,
			Name:            provider + " 环境变量",
			Provider:        provider,
			BaseURL:         baseURL,
			Model:           chooseAIModel(provider, ""),
			APIKeyMasked:    maskAISecret(key),
			APISecretMasked: maskAISecret(envAISecret(provider)),
			HasAPIKey:       true,
			HasAPISecret:    envAISecret(provider) != "",
			ExtraJSON:       "{}",
			Enabled:         true,
			Source:          "env",
		})
	}
	return items
}

func envPrefix(provider string) string {
	switch normalizeAIProvider(provider) {
	case AIProviderOpenAI:
		return "OPENAI"
	case AIProviderQwen:
		return "QWEN"
	case AIProviderZhipu:
		return "ZHIPU"
	default:
		return "DEEPSEEK"
	}
}

func envAIKey(provider string) string {
	prefix := envPrefix(provider)
	for _, name := range []string{prefix + "_API_KEY", prefix + "_KEY"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func envAISecret(provider string) string {
	return strings.TrimSpace(os.Getenv(envPrefix(provider) + "_API_SECRET"))
}

func aiClientFor(provider string) AIProviderClient {
	return NewOpenAICompatibleClient()
}

func normalizeAIProvider(provider string) string {
	value := strings.ToLower(strings.TrimSpace(provider))
	switch value {
	case "", "ds":
		return AIProviderDeepSeek
	case "chatgpt":
		return AIProviderOpenAI
	case "tongyi", "dashscope", "aliyun":
		return AIProviderQwen
	case "glm", "bigmodel":
		return AIProviderZhipu
	default:
		return value
	}
}

func chooseAIBaseURL(provider, override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimRight(strings.TrimSpace(override), "/")
	}
	for _, item := range aiProviderInfos() {
		if item.ID == normalizeAIProvider(provider) {
			return strings.TrimRight(item.DefaultBaseURL, "/")
		}
	}
	return ""
}

func chooseAIModel(provider, model string) string {
	if strings.TrimSpace(model) != "" {
		return strings.TrimSpace(model)
	}
	switch normalizeAIProvider(provider) {
	case AIProviderOpenAI:
		return "gpt-4.1-mini"
	case AIProviderQwen:
		return "qwen-plus"
	case AIProviderZhipu:
		return "glm-4-flash"
	default:
		return "deepseek-chat"
	}
}

func normalizeSymbols(symbols []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		value := strings.TrimSpace(strings.ToUpper(symbol))
		value = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(value, ".SH"), ".SZ"), ".BJ")
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func mustJSON(value interface{}) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(raw)
}

func encryptAISecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	block, err := aes.NewCipher(aiEncryptionKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	payload := gcm.Seal(nil, nonce, []byte(value), nil)
	return "v1:" + base64.StdEncoding.EncodeToString(append(nonce, payload...)), nil
}

func decryptAISecret(value string) (string, error) {
	if value == "" || !strings.HasPrefix(value, "v1:") {
		return value, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "v1:"))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(aiEncryptionKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("AI密钥密文格式错误")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func aiEncryptionKey() []byte {
	secret := strings.TrimSpace(os.Getenv("AI_CREDENTIAL_SECRET"))
	if secret == "" {
		secret = "tdx-workbench-local-ai-credential-secret"
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func maskAISecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return strings.Repeat("*", len(value))
	}
	return value[:4] + strings.Repeat("*", 4) + value[len(value)-4:]
}
