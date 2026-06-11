package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chinese-medical/internal/config"
	"chinese-medical/internal/model"
)

type ImageGenerator struct {
	cfg    config.AIConfig
	client *http.Client
}

type GeneratedImage struct {
	Path string
	URL  string
}

type generationRequest struct {
	Model        string `json:"model"`
	Prompt       string `json:"prompt"`
	N            int    `json:"n"`
	Size         string `json:"size"`
	Quality      string `json:"quality,omitempty"`
	OutputFormat string `json:"output_format,omitempty"`
}

type generationResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func NewImageGenerator(cfg config.AIConfig) ImageGenerator {
	return ImageGenerator{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (g ImageGenerator) Existing(foodID int64) ([]GeneratedImage, error) {
	dir := g.foodDir(foodID)
	matches, err := filepath.Glob(filepath.Join(dir, "*."+g.extension()))
	if err != nil {
		return nil, fmt.Errorf("list generated images: %w", err)
	}

	images := make([]GeneratedImage, 0, len(matches))
	for _, path := range matches {
		images = append(images, GeneratedImage{
			Path: path,
			URL:  "/" + filepath.ToSlash(path),
		})
	}
	return images, nil
}

func (g ImageGenerator) Generate(ctx context.Context, item model.MedicatedFood) ([]GeneratedImage, error) {
	if err := os.MkdirAll(g.foodDir(item.ID), 0755); err != nil {
		return nil, fmt.Errorf("create image output dir: %w", err)
	}

	payload := generationRequest{
		Model:        g.cfg.Model,
		Prompt:       buildPrompt(g.cfg.ImageCount, item),
		N:            g.cfg.ImageCount,
		Size:         g.cfg.Size,
		Quality:      g.cfg.Quality,
		OutputFormat: g.cfg.OutputFormat,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal image request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpointURL(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create image request: %w", err)
	}
	if apiKey := g.apiKey(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call image model: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read image response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var result generationResponse
		if err := json.Unmarshal(respBody, &result); err == nil && result.Error != nil {
			return nil, fmt.Errorf("image model returned %s: %s", resp.Status, result.Error.Message)
		}
		return nil, fmt.Errorf("image model returned %s (%s): %s", resp.Status, resp.Header.Get("Content-Type"), responseSnippet(respBody))
	}

	var result generationResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse image response from %s (%s): %w; body: %s", resp.Status, resp.Header.Get("Content-Type"), err, responseSnippet(respBody))
	}
	if result.Error != nil {
		return nil, fmt.Errorf("image model returned error: %s", result.Error.Message)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("image model returned empty data")
	}

	images := make([]GeneratedImage, 0, len(result.Data))
	stamp := time.Now().Format("20060102-150405")
	for index, image := range result.Data {
		path := filepath.Join(g.foodDir(item.ID), fmt.Sprintf("%s-%02d.%s", stamp, index+1, g.extension()))
		if image.B64JSON != "" {
			if err := writeBase64Image(path, image.B64JSON); err != nil {
				return nil, err
			}
		} else if image.URL != "" {
			if err := g.downloadImage(ctx, path, image.URL); err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("image response item %d has no image payload", index+1)
		}
		images = append(images, GeneratedImage{
			Path: path,
			URL:  "/" + filepath.ToSlash(path),
		})
	}

	return images, nil
}

func (g ImageGenerator) apiKey() string {
	if key := strings.TrimSpace(g.cfg.APIKey); key != "" {
		return key
	}
	if env := strings.TrimSpace(g.cfg.APIKeyEnv); env != "" {
		return strings.TrimSpace(os.Getenv(env))
	}
	return ""
}

func (g ImageGenerator) endpointURL() string {
	return strings.TrimRight(g.cfg.BaseURL, "/") + g.cfg.EndpointPath
}

func (g ImageGenerator) downloadImage(ctx context.Context, path, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create image download request: %w", err)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("download generated image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("download generated image returned %s", resp.Status)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create generated image file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("write generated image file: %w", err)
	}
	return nil
}

func (g ImageGenerator) foodDir(foodID int64) string {
	return filepath.Join(g.cfg.OutputDir, fmt.Sprintf("food-%d", foodID))
}

func (g ImageGenerator) extension() string {
	format := strings.ToLower(strings.TrimSpace(g.cfg.OutputFormat))
	switch format {
	case "jpeg", "jpg":
		return "jpg"
	case "webp":
		return "webp"
	default:
		return "png"
	}
}

func writeBase64Image(path, payload string) error {
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return fmt.Errorf("decode generated image: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write generated image file: %w", err)
	}
	return nil
}

func responseSnippet(body []byte) string {
	const limit = 500
	text := strings.TrimSpace(string(body))
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > limit {
		return text[:limit] + "..."
	}
	if text == "" {
		return "<empty body>"
	}
	return text
}

func buildPrompt(count int, item model.MedicatedFood) string {
	if count <= 0 {
		count = 4
	}
	return fmt.Sprintf(`为中医药食同源调理方“%s”生成 %d 张连贯的中文方剂介绍图。

整体要求：
- 这 %d 张图要像同一套科普海报系列：统一配色、统一字体层级、统一版式语言。
- 风格清雅、专业、适合网页展示；不要出现夸张疗效承诺、医生肖像、医院背书、处方笺或药品广告语。
- 每张图都需要有清晰中文标题“%s”，并带序号，例如 1/%d、2/%d。
- 文案要简洁，不要堆满小字；信息准确来自下面字段。

请把内容自然分配到多张图：
1. 来源：%s
2. 组成：%s
3. 制法：%s
4. 功效：%s

如果某个字段为空，请用“未注明”自然表达。`, item.Name, count, count, item.Name, count, count, textOrUnknown(item.Source), textOrUnknown(item.Food), textOrUnknown(item.Method), textOrUnknown(item.Effect))
}

func textOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "未注明"
	}
	return value
}
