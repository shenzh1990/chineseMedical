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
	matches := make([]string, 0)
	for _, extension := range []string{"png", "jpg", "jpeg", "webp", "gif"} {
		found, err := filepath.Glob(filepath.Join(dir, "*."+extension))
		if err != nil {
			return nil, fmt.Errorf("list generated images: %w", err)
		}
		matches = append(matches, found...)
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

func (g ImageGenerator) Prompt(item model.MedicatedFood) string {
	return buildPrompt(g.cfg.ImageCount, item)
}

func (g ImageGenerator) SaveUploaded(foodID int64, filename string, r io.Reader) (GeneratedImage, error) {
	if err := os.MkdirAll(g.foodDir(foodID), 0755); err != nil {
		return GeneratedImage{}, fmt.Errorf("create image output dir: %w", err)
	}

	head := make([]byte, 512)
	n, err := r.Read(head)
	if err != nil && err != io.EOF {
		return GeneratedImage{}, fmt.Errorf("read uploaded image: %w", err)
	}
	head = head[:n]

	extension, err := uploadExtension(filename, http.DetectContentType(head))
	if err != nil {
		return GeneratedImage{}, err
	}

	stamp := time.Now().Format("20060102-150405")
	path := filepath.Join(g.foodDir(foodID), fmt.Sprintf("%s-upload.%s", stamp, extension))
	file, err := os.Create(path)
	if err != nil {
		return GeneratedImage{}, fmt.Errorf("create uploaded image file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, io.MultiReader(bytes.NewReader(head), r)); err != nil {
		return GeneratedImage{}, fmt.Errorf("write uploaded image file: %w", err)
	}

	return GeneratedImage{
		Path: path,
		URL:  "/" + filepath.ToSlash(path),
	}, nil
}

func (g ImageGenerator) Generate(ctx context.Context, item model.MedicatedFood) ([]GeneratedImage, error) {
	if err := os.MkdirAll(g.foodDir(item.ID), 0755); err != nil {
		return nil, fmt.Errorf("create image output dir: %w", err)
	}

	payload := generationRequest{
		Model:        g.cfg.Model,
		Prompt:       g.Prompt(item),
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

func uploadExtension(filename string, contentType string) (string, error) {
	switch strings.ToLower(contentType) {
	case "image/png":
		return "png", nil
	case "image/jpeg":
		return "jpg", nil
	case "image/webp":
		return "webp", nil
	case "image/gif":
		return "gif", nil
	}

	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), ".")) {
	case "png":
		return "png", nil
	case "jpg", "jpeg":
		return "jpg", nil
	case "webp":
		return "webp", nil
	case "gif":
		return "gif", nil
	default:
		return "", fmt.Errorf("unsupported image type %q", contentType)
	}
}

func buildPrompt(count int, item model.MedicatedFood) string {
	if count <= 0 {
		count = 4
	}
	category := model.NormalizeFoodCategory(item.Category)
	return fmt.Sprintf(`为中医“%s”类别下的调理方“%s”生成 %d 张连贯的中文方剂介绍图。

主要应用场景：
- 图片主要用于手机竖屏浏览，请按竖版海报设计。
- 每张图的画面比例为 9:16，目标分辨率为 1080x1920 px。
- 重要标题、序号和正文需要在手机屏幕上清晰可读，避免过小文字。
- 内容需要适合在移动端网页中连续上下滑动查看，留出安全边距，不要把文字贴近边缘。

整体要求：
- 请生成 %d 张独立图片，每张图都是单独的竖屏海报，不要生成一张包含四个版面的合集图。
- 禁止四宫格、拼贴图、长图分栏、单张图片内同时排布 1/4、2/4、3/4、4/4 四个页面。
- 这 %d 张独立图片要像同一套科普海报系列：统一配色、统一字体层级、统一版式语言。
- 风格清雅、专业、适合网页展示；不要出现夸大疗效承诺、医生肖像、医院背书、处方笺或药品广告语。
- 每张图都需要有清晰中文标题“%s”，并带序号，例如 1/%d、2/%d。
- 文案要简洁，不要堆满小字；信息准确来自下面字段，可以在不改变原意、不添加未经证实疗效的前提下适当扩展表达，让画面内容更自然完整。

请把内容自然分配到多张图：
1. 类别：%s
2. 来源：%s
3. 组成：%s
4. 制法：%s
5. 功效：%s

如果某个字段为空，请用“未注明”自然表达。`, category, item.Name, count, count, count, item.Name, count, count, category, promptTextOrUnknown(item.Source), promptTextOrUnknown(item.Food), promptTextOrUnknown(item.Method), promptTextOrUnknown(item.Effect))
}

func promptTextOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "未注明"
	}
	return value
}
