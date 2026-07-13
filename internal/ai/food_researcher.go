package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"chinese-medical/internal/config"
	"chinese-medical/internal/model"

	"golang.org/x/net/html"
)

const maxResearchPageBytes = 1 << 20

type FoodResearcher struct {
	cfg    config.AIConfig
	client *http.Client
}

type FoodResearchDraft struct {
	Category      string   `json:"category"`
	Name          string   `json:"name"`
	Source        string   `json:"source"`
	Food          string   `json:"food"`
	Method        string   `json:"method"`
	Effect        string   `json:"effect"`
	ReferenceURLs []string `json:"reference_urls,omitempty"`
}

type researchRequest struct {
	Model        string         `json:"model"`
	Instructions string         `json:"instructions,omitempty"`
	Input        string         `json:"input"`
	Tools        []researchTool `json:"tools,omitempty"`
	Stream       bool           `json:"stream,omitempty"`
}

type researchTool struct {
	Type              string `json:"type"`
	SearchContextSize string `json:"search_context_size,omitempty"`
}

type researchResponse struct {
	OutputText string `json:"output_text"`
	Output     []struct {
		Type    string `json:"type"`
		Content []struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			Annotations []struct {
				Title string `json:"title"`
				URL   string `json:"url"`
			} `json:"annotations"`
		} `json:"content"`
	} `json:"output"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type researchStreamEvent struct {
	Type     string            `json:"type"`
	Delta    string            `json:"delta"`
	Text     string            `json:"text"`
	Response *researchResponse `json:"response,omitempty"`
	Error    *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices,omitempty"`
}

type chatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model    string                  `json:"model"`
	Messages []chatCompletionMessage `json:"messages"`
	Stream   bool                    `json:"stream,omitempty"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type chatCompletionStreamEvent struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func NewFoodResearcher(cfg config.AIConfig) FoodResearcher {
	return FoodResearcher{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.ResearchTimeout,
		},
	}
}

func (r FoodResearcher) Research(ctx context.Context, name, category, sourceURL string) (FoodResearchDraft, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return FoodResearchDraft{}, fmt.Errorf("name is required")
	}
	sourceURL = strings.TrimSpace(sourceURL)

	payload := researchRequest{
		Model:        r.cfg.ResearchModel,
		Instructions: foodResearchInstructions(),
		Input:        foodResearchPrompt(name, model.NormalizeFoodCategory(category)),
	}
	if sourceURL != "" {
		pageText, err := r.fetchPageText(ctx, sourceURL)
		if err != nil {
			return FoodResearchDraft{}, err
		}
		payload.Input = foodResearchPromptFromPage(name, model.NormalizeFoodCategory(category), sourceURL, pageText)
	} else if toolType := strings.TrimSpace(r.cfg.ResearchToolType); toolType != "" {
		payload.Tools = []researchTool{{
			Type:              toolType,
			SearchContextSize: strings.TrimSpace(r.cfg.ResearchContextSize),
		}}
	}

	result, err := r.callModel(ctx, payload)
	if err != nil {
		return FoodResearchDraft{}, err
	}

	text, urls := researchOutputText(result)
	draft, err := parseFoodResearchDraft(text)
	if err != nil {
		return FoodResearchDraft{}, err
	}

	if draft.Name == "" {
		draft.Name = name
	}
	draft.Category = model.NormalizeFoodCategory(firstNonEmpty(draft.Category, category))
	draft.ReferenceURLs = mergeStrings(draft.ReferenceURLs, urls)
	if sourceURL != "" {
		draft.ReferenceURLs = mergeStrings([]string{sourceURL}, draft.ReferenceURLs)
	}
	draft.trim()
	if len(draft.ReferenceURLs) > 0 {
		draft.Source = appendReferenceURLs(draft.Source, draft.ReferenceURLs)
	}

	return draft, nil
}

func (r FoodResearcher) SimpleTest(ctx context.Context) (string, error) {
	payload := researchRequest{
		Model:        r.cfg.ResearchModel,
		Instructions: "你是一个连接测试助手。请严格使用中文简短回复。",
		Input:        "请只回复：连接测试成功",
	}

	result, err := r.callModel(ctx, payload)
	if err != nil {
		return "", err
	}

	text, _ := researchOutputText(result)
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("food research model returned empty test response")
	}
	if len([]rune(text)) > 120 {
		runes := []rune(text)
		text = string(runes[:120]) + "..."
	}
	return text, nil
}

func (r FoodResearcher) callModel(ctx context.Context, payload researchRequest) (researchResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return researchResponse{}, fmt.Errorf("marshal food research request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpointURL(), bytes.NewReader(body))
	if err != nil {
		return researchResponse{}, fmt.Errorf("create food research request: %w", err)
	}
	if apiKey := r.apiKey(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return researchResponse{}, fmt.Errorf("call food research model: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return researchResponse{}, fmt.Errorf("read food research response: %w", err)
	}

	var result researchResponse
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if err := json.Unmarshal(respBody, &result); err == nil && result.Error != nil {
			return researchResponse{}, fmt.Errorf("food research model returned %s: %s", resp.Status, result.Error.Message)
		}
		return researchResponse{}, fmt.Errorf("food research model returned %s (%s): %s", resp.Status, resp.Header.Get("Content-Type"), responseSnippet(respBody))
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return researchResponse{}, fmt.Errorf("parse food research response from %s (%s): %w; body: %s", resp.Status, resp.Header.Get("Content-Type"), err, responseSnippet(respBody))
	}
	if result.Error != nil {
		return researchResponse{}, fmt.Errorf("food research model returned error: %s", result.Error.Message)
	}
	return result, nil
}

func (r FoodResearcher) chatCompletionText(ctx context.Context, messages []chatCompletionMessage) (string, error) {
	payload := chatCompletionRequest{
		Model:    r.cfg.ResearchModel,
		Messages: messages,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal chat completion request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.chatCompletionEndpointURL(), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create chat completion request: %w", err)
	}
	if apiKey := r.apiKey(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call chat completion model: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read chat completion response: %w", err)
	}

	var result chatCompletionResponse
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if err := json.Unmarshal(respBody, &result); err == nil && result.Error != nil {
			return "", fmt.Errorf("chat completion model returned %s: %s", resp.Status, result.Error.Message)
		}
		return "", fmt.Errorf("chat completion model returned %s (%s): %s", resp.Status, resp.Header.Get("Content-Type"), responseSnippet(respBody))
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse chat completion response from %s (%s): %w; body: %s", resp.Status, resp.Header.Get("Content-Type"), err, responseSnippet(respBody))
	}
	if result.Error != nil {
		return "", fmt.Errorf("chat completion model returned error: %s", result.Error.Message)
	}
	for _, choice := range result.Choices {
		if text := strings.TrimSpace(choice.Message.Content); text != "" {
			return text, nil
		}
	}
	return "", errors.New("chat completion model returned empty response")
}

func (r FoodResearcher) streamModelText(ctx context.Context, payload researchRequest, onDelta func(string) error) (string, error) {
	payload.Stream = true
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal food research stream request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpointURL(), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create food research stream request: %w", err)
	}
	if apiKey := r.apiKey(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call food research stream model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("read food research stream error response: %w", err)
		}
		var result researchResponse
		if err := json.Unmarshal(respBody, &result); err == nil && result.Error != nil {
			return "", fmt.Errorf("food research model returned %s: %s", resp.Status, result.Error.Message)
		}
		return "", fmt.Errorf("food research model returned %s (%s): %s", resp.Status, resp.Header.Get("Content-Type"), responseSnippet(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024), maxResearchPageBytes)

	var answer strings.Builder
	var finalText string
	dataLines := []string{}
	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		if strings.TrimSpace(data) == "[DONE]" {
			return nil
		}

		var event researchStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("parse food research stream event: %w", err)
		}
		if event.Error != nil {
			return fmt.Errorf("food research model returned stream error: %s", event.Error.Message)
		}

		if text := researchStreamDelta(event); text != "" {
			answer.WriteString(text)
			if onDelta != nil {
				if err := onDelta(text); err != nil {
					return err
				}
			}
			return nil
		}
		if text := researchStreamFinalText(event); text != "" {
			finalText = text
		}
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				return answer.String(), err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return answer.String(), fmt.Errorf("read food research stream response: %w", err)
	}
	if err := flush(); err != nil {
		return answer.String(), err
	}

	text := answer.String()
	if text == "" && finalText != "" {
		text = finalText
		if onDelta != nil {
			if err := onDelta(finalText); err != nil {
				return text, err
			}
		}
	}
	if strings.TrimSpace(text) == "" {
		return "", errors.New("food research stream model returned empty response")
	}
	return text, nil
}

func (r FoodResearcher) streamChatCompletionText(ctx context.Context, messages []chatCompletionMessage, onDelta func(string) error) (string, error) {
	payload := chatCompletionRequest{
		Model:    r.cfg.ResearchModel,
		Messages: messages,
		Stream:   true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal chat completion stream request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.chatCompletionEndpointURL(), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create chat completion stream request: %w", err)
	}
	if apiKey := r.apiKey(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call chat completion stream model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("read chat completion stream error response: %w", err)
		}
		var result chatCompletionResponse
		if err := json.Unmarshal(respBody, &result); err == nil && result.Error != nil {
			return "", fmt.Errorf("chat completion model returned %s: %s", resp.Status, result.Error.Message)
		}
		return "", fmt.Errorf("chat completion model returned %s (%s): %s", resp.Status, resp.Header.Get("Content-Type"), responseSnippet(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024), maxResearchPageBytes)

	var answer strings.Builder
	dataLines := []string{}
	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		if strings.TrimSpace(data) == "[DONE]" {
			return nil
		}

		var event chatCompletionStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("parse chat completion stream event: %w", err)
		}
		if event.Error != nil {
			return fmt.Errorf("chat completion model returned stream error: %s", event.Error.Message)
		}
		for _, choice := range event.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			answer.WriteString(choice.Delta.Content)
			if onDelta != nil {
				if err := onDelta(choice.Delta.Content); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				return answer.String(), err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return answer.String(), fmt.Errorf("read chat completion stream response: %w", err)
	}
	if err := flush(); err != nil {
		return answer.String(), err
	}

	text := answer.String()
	if strings.TrimSpace(text) == "" {
		return "", errors.New("chat completion stream model returned empty response")
	}
	return text, nil
}

func (r FoodResearcher) fetchPageText(ctx context.Context, rawURL string) (string, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("invalid source url")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("create source page request: %w", err)
	}
	req.Header.Set("User-Agent", "chinese-medical-ai-research/1.0")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch source page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("fetch source page returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResearchPageBytes))
	if err != nil {
		return "", fmt.Errorf("read source page: %w", err)
	}

	text := strings.TrimSpace(extractHTMLText(strings.NewReader(string(body))))
	if text == "" {
		text = strings.TrimSpace(string(body))
	}
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return "", fmt.Errorf("source page has no readable text")
	}
	if len([]rune(text)) > 12000 {
		runes := []rune(text)
		text = string(runes[:12000])
	}
	return text, nil
}

func (r FoodResearcher) apiKey() string {
	if key := strings.TrimSpace(r.cfg.ResearchAPIKey); key != "" {
		return key
	}
	if env := strings.TrimSpace(r.cfg.ResearchAPIKeyEnv); env != "" {
		return strings.TrimSpace(os.Getenv(env))
	}
	if key := strings.TrimSpace(r.cfg.APIKey); key != "" {
		return key
	}
	if env := strings.TrimSpace(r.cfg.APIKeyEnv); env != "" {
		return strings.TrimSpace(os.Getenv(env))
	}
	return ""
}

func (r FoodResearcher) endpointURL() string {
	return strings.TrimRight(r.cfg.ResearchBaseURL, "/") + r.cfg.ResearchEndpointPath
}

func (r FoodResearcher) chatCompletionEndpointURL() string {
	baseURL := strings.TrimRight(r.cfg.ResearchBaseURL, "/")
	path := strings.TrimSpace(r.cfg.ResearchEndpointPath)
	if strings.Contains(path, "chat/completions") {
		if strings.HasSuffix(baseURL, "/v1") && strings.HasPrefix(path, "/v1/") {
			return strings.TrimSuffix(baseURL, "/v1") + path
		}
		if strings.HasPrefix(path, "/") {
			return baseURL + path
		}
		return baseURL + "/" + path
	}
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/chat/completions"
	}
	return baseURL + "/v1/chat/completions"
}

func foodResearchInstructions() string {
	return `你是一名严谨的中医药膳资料整理助手。你需要只基于可核验资料整理结果。不要编造典籍、出处、食材、制法或功效；缺少可靠资料时使用空字符串。输出必须是一个 JSON 对象，不要包含 Markdown 或解释文字。`
}

func foodResearchPrompt(name, category string) string {
	return fmt.Sprintf(`请检索网页并整理“%s”的新增数据，目标类别为“%s”。

请返回且仅返回如下 JSON 对象：
{
  "category": "类别，优先使用输入类别",
  "name": "名称",
  "source": "来源，写明资料来源、出处或网页标题；如果多个来源，用中文分号分隔",
  "food": "组成，列出食材/药食同源材料和用量；没有可靠用量时只列材料",
  "method": "制法，整理为可执行的简明步骤",
  "effect": "功效，使用资料中的谨慎表述，不添加治疗承诺",
  "reference_urls": ["用于核验的网页 URL，最多 5 个"]
}

要求：
- 优先引用政府、医院、高校、权威媒体、公开标准或可信书籍数据库网页。
- “功效”只写传统功效或膳食调理描述，不写保证疗效、治愈率或替代医疗建议。
- 所有字段都必须是字符串，reference_urls 必须是字符串数组。`, name, category)
}

func foodResearchPromptFromPage(name, category, sourceURL, pageText string) string {
	return fmt.Sprintf(`请只根据下面提供的网页内容整理“%s”的新增数据，目标类别为“%s”。不要进行额外网页搜索，不要使用网页内容之外的信息。

网页地址：
%s

网页内容：
%s

请返回且仅返回如下 JSON 对象：
{
  "category": "类别，优先使用输入类别",
  "name": "名称",
  "source": "来源，写明网页标题、资料来源或出处；如果网页没有标题则写网页地址",
  "food": "组成，列出食材/药食同源材料和用量；没有可靠用量时只列材料",
  "method": "制法，整理为可执行的简明步骤",
  "effect": "功效，使用网页内容中的谨慎表述，不添加治疗承诺",
  "reference_urls": ["%s"]
}

要求：
- 如果网页内容不足以判断某个字段，请将该字段设为空字符串。
- “功效”只写传统功效或膳食调理描述，不写保证疗效、治愈率或替代医疗建议。
- 所有字段都必须是字符串，reference_urls 必须是字符串数组。`, name, category, sourceURL, pageText, sourceURL)
}

func extractHTMLText(r io.Reader) string {
	doc, err := html.Parse(r)
	if err != nil {
		return ""
	}

	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript", "svg":
				return
			}
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				if builder.Len() > 0 {
					builder.WriteString(" ")
				}
				builder.WriteString(text)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return builder.String()
}

func researchOutputText(result researchResponse) (string, []string) {
	if strings.TrimSpace(result.OutputText) != "" {
		return result.OutputText, nil
	}

	var parts []string
	var urls []string
	for _, item := range result.Output {
		for _, content := range item.Content {
			if strings.TrimSpace(content.Text) != "" {
				parts = append(parts, content.Text)
			}
			for _, annotation := range content.Annotations {
				if strings.TrimSpace(annotation.URL) != "" {
					urls = append(urls, annotation.URL)
				}
			}
		}
	}
	return strings.Join(parts, "\n"), urls
}

func researchStreamDelta(event researchStreamEvent) string {
	if event.Delta != "" {
		return event.Delta
	}
	for _, choice := range event.Choices {
		if choice.Delta.Content != "" {
			return choice.Delta.Content
		}
		if choice.Message.Content != "" {
			return choice.Message.Content
		}
	}
	return ""
}

func researchStreamFinalText(event researchStreamEvent) string {
	if event.Text != "" && strings.HasSuffix(event.Type, ".done") {
		return event.Text
	}
	if event.Response != nil {
		text, _ := researchOutputText(*event.Response)
		return text
	}
	return ""
}

func parseFoodResearchDraft(text string) (FoodResearchDraft, error) {
	text = extractJSONObject(text)
	if text == "" {
		return FoodResearchDraft{}, errors.New("food research response did not contain JSON")
	}

	var draft FoodResearchDraft
	if err := json.Unmarshal([]byte(text), &draft); err != nil {
		return FoodResearchDraft{}, fmt.Errorf("parse food research JSON: %w", err)
	}
	return draft, nil
}

func extractJSONObject(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return ""
	}
	return text[start : end+1]
}

func (d *FoodResearchDraft) trim() {
	d.Category = strings.TrimSpace(d.Category)
	d.Name = strings.TrimSpace(d.Name)
	d.Source = strings.TrimSpace(d.Source)
	d.Food = strings.TrimSpace(d.Food)
	d.Method = strings.TrimSpace(d.Method)
	d.Effect = strings.TrimSpace(d.Effect)
	d.ReferenceURLs = mergeStrings(d.ReferenceURLs, nil)
}

func appendReferenceURLs(source string, urls []string) string {
	source = strings.TrimSpace(source)
	if len(urls) == 0 {
		return source
	}
	references := "参考链接：" + strings.Join(urls, "；")
	if source == "" {
		return references
	}
	if strings.Contains(source, "参考链接：") {
		return source
	}
	return source + "\n" + references
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mergeStrings(left, right []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(left)+len(right))
	for _, value := range append(left, right...) {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
