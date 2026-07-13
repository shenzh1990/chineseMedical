package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"chinese-medical/internal/ai"
	"chinese-medical/internal/config"
	"chinese-medical/internal/model"
	"chinese-medical/internal/repository"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	deps       Dependencies
	foods      repository.MedicatedFoodRepository
	renshu     repository.RenShuDataRepository
	skills     repository.SkillRepository
	users      repository.UserRepository
	aiSettings *ai.SettingsStore
}

type medicatedFoodForm struct {
	ID       string
	Category string
	Name     string
	Source   string
	Food     string
	Method   string
	Effect   string
}

type foodResearchRequest struct {
	Name      string `json:"name"`
	Category  string `json:"category"`
	SourceURL string `json:"source_url"`
}

type tcmQuestionRequest struct {
	Question string              `json:"question"`
	Mode     string              `json:"mode"`
	SkillID  int64               `json:"skill_id"`
	MCPIDs   []int64             `json:"mcp_ids"`
	History  []ai.TCMChatMessage `json:"history"`
}

type foodRecommendation struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	URL      string `json:"url"`
	Reason   string `json:"reason"`
	Effect   string `json:"effect,omitempty"`
}

type aiSettingsForm struct {
	BaseURL              string
	EndpointPath         string
	APIKey               string
	APIKeySet            bool
	APIKeyEnv            string
	Model                string
	ImageCount           string
	Size                 string
	Quality              string
	OutputFormat         string
	Timeout              string
	ResearchBaseURL      string
	ResearchEndpointPath string
	ResearchAPIKey       string
	ResearchAPIKeySet    bool
	ResearchAPIKeyEnv    string
	ResearchModel        string
	ResearchToolType     string
	ResearchContextSize  string
	ResearchTimeout      string
}

func (h Handler) Login(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"AppName": h.deps.Config.AppName,
		"Error":   c.Query("error"),
	})
}

func (h Handler) LoginPost(c *gin.Context) {
	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	if username == "" || password == "" {
		c.Redirect(http.StatusSeeOther, "/login?error="+url.QueryEscape("请输入用户名和密码"))
		return
	}

	user, err := h.users.GetByUsername(c.Request.Context(), username)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		c.Redirect(http.StatusSeeOther, "/login?error="+url.QueryEscape("用户名或密码不正确"))
		return
	}

	setSessionCookie(c, user.Username)
	c.Redirect(http.StatusSeeOther, "/")
}

func (h Handler) Logout(c *gin.Context) {
	clearSessionCookie(c)
	c.Redirect(http.StatusSeeOther, "/login")
}

func (h Handler) ChangePassword(c *gin.Context) {
	c.HTML(http.StatusOK, "change_password.html", gin.H{
		"AppName": h.deps.Config.AppName,
		"Active":  "account",
		"Error":   c.Query("error"),
		"Updated": c.Query("updated") == "1",
	})
}

func (h Handler) ChangePasswordPost(c *gin.Context) {
	username, _ := c.Get("username")
	currentPassword := c.PostForm("current_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	if currentPassword == "" || newPassword == "" || confirmPassword == "" {
		c.Redirect(http.StatusSeeOther, "/account/password?error="+url.QueryEscape("请完整填写密码信息"))
		return
	}
	if newPassword != confirmPassword {
		c.Redirect(http.StatusSeeOther, "/account/password?error="+url.QueryEscape("两次输入的新密码不一致"))
		return
	}
	if len(newPassword) < 6 {
		c.Redirect(http.StatusSeeOther, "/account/password?error="+url.QueryEscape("新密码至少需要 6 位"))
		return
	}

	user, err := h.users.GetByUsername(c.Request.Context(), username.(string))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
		c.Redirect(http.StatusSeeOther, "/account/password?error="+url.QueryEscape("当前密码不正确"))
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		h.deps.Logger.Warn("hash changed password", "username", username, "error", err)
		c.Redirect(http.StatusSeeOther, "/account/password?error="+url.QueryEscape("修改密码失败"))
		return
	}
	if err := h.users.UpdatePasswordHash(c.Request.Context(), user.Username, string(hash)); err != nil {
		h.deps.Logger.Warn("update password", "username", username, "error", err)
		c.Redirect(http.StatusSeeOther, "/account/password?error="+url.QueryEscape("修改密码失败"))
		return
	}

	c.Redirect(http.StatusSeeOther, "/account/password?updated=1")
}

func (h Handler) Index(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	page := positiveQueryInt(c, "page", 1)
	limit := positiveQueryInt(c, "limit", 12)
	if limit > 48 {
		limit = 48
	}
	query := c.Query("q")
	category := strings.TrimSpace(c.Query("category"))
	offset := (page - 1) * limit

	categories, err := h.foods.Categories(ctx)
	if err != nil {
		h.deps.Logger.Error("list medicated food categories", "error", err)
		c.HTML(http.StatusInternalServerError, "index.html", gin.H{
			"AppName": h.deps.Config.AppName,
			"Active":  "knowledge",
			"Env":     h.deps.Config.Env,
			"Error":   "类别读取失败，请先执行 SQL 同步或检查数据库连接。",
		})
		return
	}
	var categoryTotal int64
	for _, item := range categories {
		categoryTotal += item.Count
	}

	items, total, err := h.foods.List(ctx, query, category, limit, offset)
	if err != nil {
		h.deps.Logger.Error("list medicated foods", "error", err)
		c.HTML(http.StatusInternalServerError, "index.html", gin.H{
			"AppName": h.deps.Config.AppName,
			"Active":  "knowledge",
			"Env":     h.deps.Config.Env,
			"Error":   "数据读取失败，请先执行 SQL 同步或检查数据库连接。",
		})
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	if totalPages > 0 && page > totalPages {
		page = totalPages
		offset = (page - 1) * limit
		items, total, err = h.foods.List(ctx, query, category, limit, offset)
		if err != nil {
			h.deps.Logger.Error("list medicated foods", "error", err)
			c.HTML(http.StatusInternalServerError, "index.html", gin.H{
				"AppName": h.deps.Config.AppName,
				"Active":  "knowledge",
				"Env":     h.deps.Config.Env,
				"Error":   "数据读取失败，请先执行 SQL 同步或检查数据库连接。",
			})
			return
		}
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"AppName":       h.deps.Config.AppName,
		"Active":        "knowledge",
		"Env":           h.deps.Config.Env,
		"Foods":         items,
		"Categories":    categories,
		"Category":      category,
		"CategoryTotal": categoryTotal,
		"Total":         total,
		"Query":         query,
		"Page":          page,
		"Limit":         limit,
		"TotalPages":    totalPages,
		"HasPrev":       page > 1,
		"HasNext":       int64(offset+len(items)) < total,
		"PrevPage":      page - 1,
		"NextPage":      page + 1,
	})
}

func (h Handler) RenShuData(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	datasetKey := strings.TrimSpace(c.Query("dataset"))
	if datasetKey == "" {
		datasetKey = "system_configs"
	}
	dataset, ok := repository.RenShuDatasetByKey(datasetKey)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/renshu/data?dataset=system_configs")
		return
	}

	page := positiveQueryInt(c, "page", 1)
	limit := positiveQueryInt(c, "limit", 20)
	if limit > 80 {
		limit = 80
	}
	query := strings.TrimSpace(c.Query("q"))
	offset := (page - 1) * limit

	summaries, err := h.renshu.Summaries(ctx)
	if err != nil {
		h.deps.Logger.Error("summarize renshu data", "error", err)
		h.renderRenShuDataError(c, "RenShu 基础表读取失败，请确认已执行导入命令。")
		return
	}

	data, err := h.renshu.List(ctx, dataset, query, limit, offset)
	if err != nil {
		h.deps.Logger.Error("list renshu data", "dataset", dataset.Key, "error", err)
		h.renderRenShuDataError(c, "当前数据集读取失败，请检查导入表结构是否完整。")
		return
	}

	totalPages := 0
	if data.Total > 0 {
		totalPages = int((data.Total + int64(limit) - 1) / int64(limit))
	}
	if totalPages > 0 && page > totalPages {
		page = totalPages
		offset = (page - 1) * limit
		data, err = h.renshu.List(ctx, dataset, query, limit, offset)
		if err != nil {
			h.deps.Logger.Error("list renshu data after page correction", "dataset", dataset.Key, "error", err)
			h.renderRenShuDataError(c, "当前数据集读取失败，请检查导入表结构是否完整。")
			return
		}
	}

	totalRows := int64(0)
	for _, summary := range summaries {
		totalRows += summary.Count
	}

	c.HTML(http.StatusOK, "renshu_data.html", gin.H{
		"AppName":         h.deps.Config.AppName,
		"Active":          "renshu-data",
		"Datasets":        repository.RenShuDatasets(),
		"Summaries":       summaries,
		"SelectedDataset": dataset,
		"Rows":            data.Rows,
		"Total":           data.Total,
		"TotalRows":       totalRows,
		"Query":           query,
		"Page":            page,
		"Limit":           limit,
		"TotalPages":      totalPages,
		"HasPrev":         page > 1,
		"HasNext":         int64(offset+len(data.Rows)) < data.Total,
		"PrevPage":        page - 1,
		"NextPage":        page + 1,
	})
}

func (h Handler) renderRenShuDataError(c *gin.Context, message string) {
	c.HTML(http.StatusInternalServerError, "renshu_data.html", gin.H{
		"AppName":         h.deps.Config.AppName,
		"Active":          "renshu-data",
		"Datasets":        repository.RenShuDatasets(),
		"Summaries":       []repository.RenShuDatasetSummary{},
		"SelectedDataset": repository.RenShuDataset{},
		"Rows":            []map[string]string{},
		"Total":           int64(0),
		"TotalRows":       int64(0),
		"Query":           "",
		"Page":            1,
		"Limit":           20,
		"TotalPages":      0,
		"Error":           message,
	})
}

func (h Handler) ImageSplitter(c *gin.Context) {
	c.HTML(http.StatusOK, "image_splitter.html", gin.H{
		"AppName": h.deps.Config.AppName,
		"Active":  "image-splitter",
	})
}

func (h Handler) TCMQuestions(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	skills, mcps, err := h.tcmCapabilities(ctx)
	if err != nil {
		h.deps.Logger.Warn("list tcm capabilities", "error", err)
	}

	c.HTML(http.StatusOK, "tcm_questions.html", gin.H{
		"AppName": h.deps.Config.AppName,
		"Active":  "tcm-questions",
		"Model":   h.aiSettings.Config().ResearchModel,
		"Skills":  skills,
		"MCPs":    mcps,
	})
}

func (h Handler) AskTCMQuestion(c *gin.Context) {
	var req tcmQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式不正确。"})
		return
	}

	question := strings.TrimSpace(req.Question)
	if question == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先输入问题。"})
		return
	}
	if len([]rune(question)) > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "问题过长，请控制在 1000 个字以内。"})
		return
	}

	cfg := h.aiSettings.Config()
	ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.ResearchTimeout)
	defer cancel()

	foods, _, err := h.foods.List(ctx, question, "", 5, 0)
	if err != nil {
		h.deps.Logger.Warn("search knowledge for tcm question", "error", err)
		foods = nil
	}
	recommendedFoods, err := h.relatedFoods(ctx, question, foods)
	if err != nil {
		h.deps.Logger.Warn("search related foods for tcm question", "error", err)
		recommendedFoods = foods
	}

	selectedSkill, selectedMCPs, err := h.selectedTCMCapabilities(ctx, req.SkillID, req.MCPIDs)
	if err != nil {
		h.deps.Logger.Warn("select tcm capabilities", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	intent := ai.AnalyzeTCMIntent(question, req.Mode)
	recommendations := foodRecommendations(question, recommendedFoods)
	thoughts := tcmThoughts(
		intent,
		req.Mode,
		req.History,
		foods,
		selectedSkill,
		selectedMCPs,
	)

	startedAt := time.Now()
	answer, err := ai.NewTCMAdvisor(cfg).Answer(ctx, question, req.Mode, req.History, foods, selectedSkill, selectedMCPs)
	thinkingDuration := time.Since(startedAt)
	if err != nil {
		h.deps.Logger.Warn("answer tcm question", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": userFacingResearchError(err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"answer":           answer.Answer,
		"mode":             answer.Mode,
		"intent":           answer.Intent,
		"sources":          foods,
		"recommendations":  recommendations,
		"skill":            selectedSkill,
		"mcps":             selectedMCPs,
		"thoughts":         thoughts,
		"thinking_time":    formatThinkingTime(thinkingDuration),
		"thinking_time_ms": thinkingDuration.Milliseconds(),
	})
}

func formatThinkingTime(duration time.Duration) string {
	if duration < time.Second {
		return fmt.Sprintf("%d 毫秒", duration.Milliseconds())
	}
	return fmt.Sprintf("%.1f 秒", duration.Seconds())
}

func writeTCMStreamEvent(c *gin.Context, event gin.H) bool {
	select {
	case <-c.Request.Context().Done():
		return false
	default:
	}

	data, err := json.Marshal(event)
	if err != nil {
		return false
	}
	if _, err := c.Writer.Write(data); err != nil {
		return false
	}
	if _, err := c.Writer.Write([]byte("\n")); err != nil {
		return false
	}
	c.Writer.Flush()
	return true
}

func (h Handler) relatedFoods(ctx context.Context, question string, fallback []model.MedicatedFood) ([]model.MedicatedFood, error) {
	keywords := recommendationKeywords(question)
	foods, err := h.foods.Related(ctx, keywords, model.DefaultFoodCategory, 3)
	if err != nil {
		return nil, err
	}
	if len(foods) > 0 {
		return foods, nil
	}

	filtered := []model.MedicatedFood{}
	for _, item := range fallback {
		if item.Category == model.DefaultFoodCategory {
			filtered = append(filtered, item)
		}
		if len(filtered) >= 3 {
			break
		}
	}
	return filtered, nil
}

func foodRecommendations(question string, foods []model.MedicatedFood) []foodRecommendation {
	recommendations := make([]foodRecommendation, 0, len(foods))
	seen := map[int64]bool{}
	for _, item := range foods {
		if item.ID <= 0 || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		recommendations = append(recommendations, foodRecommendation{
			ID:       item.ID,
			Name:     item.Name,
			Category: item.Category,
			URL:      fmt.Sprintf("/foods/%d/images", item.ID),
			Reason:   recommendationReason(question, item),
			Effect:   firstSentence(item.Effect, 90),
		})
		if len(recommendations) >= 3 {
			break
		}
	}
	return recommendations
}

func recommendationReason(question string, item model.MedicatedFood) string {
	question = strings.TrimSpace(question)
	if question != "" {
		for _, keyword := range recommendationKeywords(question) {
			if containsAnyFold(item.Name, keyword) || containsAnyFold(item.Food, keyword) || containsAnyFold(item.Effect, keyword) || containsAnyFold(item.Source, keyword) {
				return fmt.Sprintf("问题中提到「%s」，该条目的名称、组成或功效描述与之相关。", keyword)
			}
		}
	}
	if strings.TrimSpace(item.Effect) != "" {
		return "本地知识库检索命中，功效描述与当前问题语义相关。"
	}
	if strings.TrimSpace(item.Food) != "" {
		return "本地知识库检索命中，组成信息与当前问题语义相关。"
	}
	return "本地知识库检索命中，建议打开详情后结合自身情况谨慎参考。"
}

func recommendationKeywords(text string) []string {
	replacer := strings.NewReplacer(
		"，", " ", "。", " ", "、", " ", "；", " ", "？", " ", "！", " ",
		",", " ", ".", " ", ";", " ", "?", " ", "!", " ", "\n", " ", "\t", " ",
	)
	text = replacer.Replace(strings.TrimSpace(text))
	parts := strings.Fields(text)
	keywords := []string{}
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len([]rune(part)) < 2 || seen[part] {
			continue
		}
		seen[part] = true
		keywords = append(keywords, part)
		if len(keywords) >= 8 {
			break
		}
	}
	return keywords
}

func containsAnyFold(text, keyword string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	return text != "" && keyword != "" && strings.Contains(text, keyword)
}

func firstSentence(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	for _, sep := range []string{"。", "；", "\n"} {
		if idx := strings.Index(text, sep); idx > 0 {
			text = text[:idx+len(sep)]
			break
		}
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}

func tcmThoughts(intent ai.TCMIntentResult, requestedMode string, history []ai.TCMChatMessage, foods []model.MedicatedFood, skill *model.Skill, mcps []model.Skill) []gin.H {
	mode := strings.TrimSpace(requestedMode)
	if mode == "" || mode == "auto" {
		mode = "自动识别"
	}

	thoughts := []gin.H{
		{
			"title":  "接收问题",
			"detail": fmt.Sprintf("读取用户问题，并带入最近 %d 条对话上下文。", len(history)),
		},
		{
			"title":  "模式与场景",
			"detail": fmt.Sprintf("问答模式为 %s；识别为%s，置信度约 %.0f%%。", mode, intent.Label, intent.Confidence*100),
		},
	}

	if len(intent.MatchedSignals) > 0 {
		thoughts = append(thoughts, gin.H{
			"title":  "命中信号",
			"detail": strings.Join(intent.MatchedSignals, "；"),
		})
	}

	if skill != nil {
		thoughts = append(thoughts, gin.H{
			"title":  "选用 Skill",
			"detail": fmt.Sprintf("按「%s」的流程和边界组织回答。", skill.Name),
		})
	} else {
		thoughts = append(thoughts, gin.H{
			"title":  "选用 Skill",
			"detail": "未指定 Skill，使用默认中医识别网络和安全边界。",
		})
	}

	if len(mcps) > 0 {
		names := make([]string, 0, len(mcps))
		for _, item := range mcps {
			names = append(names, item.Name)
		}
		thoughts = append(thoughts, gin.H{
			"title":  "MCP 能力",
			"detail": "已选择 " + strings.Join(names, "、") + "；当前作为待接入能力上下文，不声称真实调用外部系统。",
		})
	} else {
		thoughts = append(thoughts, gin.H{
			"title":  "MCP 能力",
			"detail": "未选择 MCP，回答只使用本地知识库检索和模型能力。",
		})
	}

	sourceDetail := "本地知识库未命中相关调理方条目。"
	if len(foods) > 0 {
		names := make([]string, 0, len(foods))
		for _, item := range foods {
			names = append(names, item.Name)
		}
		sourceDetail = fmt.Sprintf("本地知识库命中 %d 条：%s。", len(foods), strings.Join(names, "、"))
	}
	thoughts = append(thoughts, gin.H{
		"title":  "参考知识",
		"detail": sourceDetail,
	})

	if len(intent.MissingInfo) > 0 {
		thoughts = append(thoughts, gin.H{
			"title":  "追问信息",
			"detail": "仍缺少：" + strings.Join(intent.MissingInfo, "、") + "。",
		})
	}

	thoughts = append(thoughts, gin.H{
		"title":  "安全边界",
		"detail": "回答按健康科普处理，不替代诊断、处方或线下医生判断；高风险情况提示及时就医。",
	})

	return thoughts
}

func (h Handler) tcmCapabilities(ctx context.Context) ([]model.Skill, []model.Skill, error) {
	items, err := h.skills.Enabled(ctx)
	if err != nil {
		return nil, nil, err
	}

	skills := []model.Skill{}
	mcps := []model.Skill{}
	for _, item := range items {
		if isMCPCapability(item) {
			mcps = append(mcps, item)
			continue
		}
		skills = append(skills, item)
	}
	return skills, mcps, nil
}

func (h Handler) selectedTCMCapabilities(ctx context.Context, skillID int64, mcpIDs []int64) (*model.Skill, []model.Skill, error) {
	var selectedSkill *model.Skill
	if skillID > 0 {
		item, err := h.skills.Get(ctx, skillID)
		if err != nil {
			return nil, nil, fmt.Errorf("选择的 Skill 不存在")
		}
		if !item.Enabled || isMCPCapability(item) {
			return nil, nil, fmt.Errorf("选择的 Skill 不可用")
		}
		selectedSkill = &item
	}

	selectedMCPs := []model.Skill{}
	seen := map[int64]bool{}
	for _, id := range mcpIDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		item, err := h.skills.Get(ctx, id)
		if err != nil {
			return nil, nil, fmt.Errorf("选择的 MCP 不存在")
		}
		if !item.Enabled || !isMCPCapability(item) {
			return nil, nil, fmt.Errorf("选择的 MCP 不可用")
		}
		selectedMCPs = append(selectedMCPs, item)
		if len(selectedMCPs) >= 5 {
			break
		}
	}

	return selectedSkill, selectedMCPs, nil
}

func isMCPCapability(item model.Skill) bool {
	category := strings.ToUpper(item.Category)
	name := strings.ToUpper(item.Name)
	tags := strings.ToUpper(item.Tags)
	return strings.Contains(category, "MCP") || strings.Contains(name, "MCP") || strings.Contains(tags, "MCP")
}

func (h Handler) AISettings(c *gin.Context) {
	h.renderAISettings(c, http.StatusOK, "", "", aiSettingsFormFromConfig(h.aiSettings.Config()))
}

func (h Handler) SaveAISettings(c *gin.Context) {
	form := aiSettingsFormFromPost(c, h.aiSettings.Config())
	updated, err := form.toConfig(h.aiSettings.Config(), c.PostForm("clear_api_key") != "", c.PostForm("clear_research_api_key") != "")
	if err != nil {
		h.renderAISettings(c, http.StatusBadRequest, userFacingAISettingsError(err), "", form)
		return
	}

	if err := config.WriteAIConfig(h.deps.Config.Path, updated); err != nil {
		h.deps.Logger.Warn("write ai settings", "error", err)
		h.renderAISettings(c, http.StatusInternalServerError, userFacingAISettingsError(err), "", form)
		return
	}

	h.aiSettings.Update(updated)
	h.renderAISettings(c, http.StatusOK, "", "大模型配置已保存。", aiSettingsFormFromConfig(updated))
}

func (h Handler) TestAISettings(c *gin.Context) {
	form := aiSettingsFormFromPost(c, h.aiSettings.Config())
	cfg, err := form.toConfig(h.aiSettings.Config(), c.PostForm("clear_api_key") != "", c.PostForm("clear_research_api_key") != "")
	if err != nil {
		h.renderAISettings(c, http.StatusBadRequest, userFacingAISettingsError(err), "", form)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.ResearchTimeout)
	defer cancel()

	result, err := ai.NewFoodResearcher(cfg).SimpleTest(ctx)
	if err != nil {
		h.deps.Logger.Warn("test ai settings", "error", err)
		h.renderAISettings(c, http.StatusBadGateway, userFacingResearchError(err), "", form)
		return
	}

	h.renderAISettings(c, http.StatusOK, "", "测试成功："+result, form)
}

func (h Handler) renderAISettings(c *gin.Context, status int, errorMessage, successMessage string, form aiSettingsForm) {
	c.HTML(status, "ai_settings.html", gin.H{
		"AppName": h.deps.Config.AppName,
		"Active":  "ai-settings",
		"Error":   errorMessage,
		"Success": successMessage,
		"Form":    form,
	})
}

func (h Handler) NewFood(c *gin.Context) {
	h.renderFoodForm(c, http.StatusOK, "", medicatedFoodForm{
		Category: model.DefaultFoodCategory,
	})
}

func (h Handler) ResearchFood(c *gin.Context) {
	var req foodResearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式不正确。"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先填写名称。"})
		return
	}
	if len([]rune(name)) > 120 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称过长，请控制在 120 个字以内。"})
		return
	}
	sourceURL := strings.TrimSpace(req.SourceURL)
	if sourceURL != "" && !isHTTPURL(sourceURL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "网页地址必须以 http:// 或 https:// 开头。"})
		return
	}

	cfg := h.aiSettings.Config()
	ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.ResearchTimeout)
	defer cancel()

	draft, err := ai.NewFoodResearcher(cfg).Research(ctx, name, req.Category, sourceURL)
	if err != nil {
		h.deps.Logger.Warn("research medicated food", "name", name, "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": userFacingResearchError(err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"food": draft})
}

func (h Handler) CreateFood(c *gin.Context) {
	form := medicatedFoodForm{
		ID:       strings.TrimSpace(c.PostForm("id")),
		Category: model.NormalizeFoodCategory(c.PostForm("category")),
		Name:     strings.TrimSpace(c.PostForm("name")),
		Source:   strings.TrimSpace(c.PostForm("source")),
		Food:     strings.TrimSpace(c.PostForm("food")),
		Method:   strings.TrimSpace(c.PostForm("method")),
		Effect:   strings.TrimSpace(c.PostForm("effect")),
	}

	if form.Name == "" {
		h.renderFoodForm(c, http.StatusBadRequest, "请填写名称。", form)
		return
	}

	var id int64
	if form.ID != "" {
		parsedID, err := strconv.ParseInt(form.ID, 10, 64)
		if err != nil || parsedID <= 0 {
			h.renderFoodForm(c, http.StatusBadRequest, "ID 必须是大于 0 的数字。", form)
			return
		}
		id = parsedID
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	item, err := h.foods.Create(ctx, model.MedicatedFood{
		ID:       id,
		Category: form.Category,
		Name:     form.Name,
		Source:   form.Source,
		Food:     form.Food,
		Method:   form.Method,
		Effect:   form.Effect,
	})
	if err != nil {
		h.deps.Logger.Warn("create medicated food", "error", err)
		h.renderFoodForm(c, http.StatusInternalServerError, userFacingCreateFoodError(err), form)
		return
	}

	c.Redirect(http.StatusSeeOther, "/foods/"+strconv.FormatInt(item.ID, 10)+"/images?created=1")
}

func (h Handler) renderFoodForm(c *gin.Context, status int, message string, form medicatedFoodForm) {
	c.HTML(status, "food_new.html", gin.H{
		"AppName":    h.deps.Config.AppName,
		"Active":     "knowledge-new",
		"Error":      message,
		"Form":       form,
		"Categories": model.FoodCategoryOptions(),
	})
}

func (h Handler) FoodImages(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	item, err := h.foods.Get(ctx, id)
	if err != nil {
		h.deps.Logger.Warn("get medicated food", "id", id, "error", err)
		c.HTML(http.StatusNotFound, "food_images.html", gin.H{
			"AppName": h.deps.Config.AppName,
			"Active":  "knowledge",
			"Error":   "未找到这个调理方。",
		})
		return
	}

	cfg := h.aiSettings.Config()
	generator := ai.NewImageGenerator(cfg)
	images, err := generator.Existing(id)
	if err != nil {
		h.deps.Logger.Warn("list generated images", "id", id, "error", err)
	}

	c.HTML(http.StatusOK, "food_images.html", gin.H{
		"AppName":    h.deps.Config.AppName,
		"Active":     "knowledge",
		"Food":       item,
		"Images":     images,
		"ImageCount": cfg.ImageCount,
		"Model":      cfg.Model,
		"BaseURL":    cfg.BaseURL,
		"Prompt":     generator.Prompt(item),
		"Error":      c.Query("error"),
		"Created":    c.Query("created") == "1",
		"Generated":  c.Query("generated") == "1",
		"Uploaded":   c.Query("uploaded") == "1",
		"Deleted":    c.Query("deleted") == "1",
	})
}

func (h Handler) GenerateFoodImages(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	item, err := h.foods.Get(ctx, id)
	if err != nil {
		h.deps.Logger.Warn("get medicated food before image generation", "id", id, "error", err)
		c.Redirect(http.StatusSeeOther, "/foods/"+strconv.FormatInt(id, 10)+"/images?error=未找到这个调理方")
		return
	}

	cfg := h.aiSettings.Config()
	generationCtx, generationCancel := context.WithTimeout(c.Request.Context(), cfg.Timeout)
	defer generationCancel()

	if _, err := ai.NewImageGenerator(cfg).Generate(generationCtx, item); err != nil {
		h.deps.Logger.Warn("generate food images", "id", id, "error", err)
		c.Redirect(http.StatusSeeOther, "/foods/"+strconv.FormatInt(id, 10)+"/images?error="+url.QueryEscape(userFacingGenerationError(err)))
		return
	}

	c.Redirect(http.StatusSeeOther, "/foods/"+strconv.FormatInt(id, 10)+"/images?generated=1")
}

func (h Handler) UploadFoodImage(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 25<<20)
	fileHeader, err := c.FormFile("image")
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/foods/"+strconv.FormatInt(id, 10)+"/images?error="+url.QueryEscape("请选择要上传的图片"))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		h.deps.Logger.Warn("open uploaded food image", "id", id, "error", err)
		c.Redirect(http.StatusSeeOther, "/foods/"+strconv.FormatInt(id, 10)+"/images?error="+url.QueryEscape("读取上传图片失败"))
		return
	}
	defer file.Close()

	if _, err := h.aiSettings.ImageGenerator().SaveUploaded(id, fileHeader.Filename, file); err != nil {
		h.deps.Logger.Warn("save uploaded food image", "id", id, "error", err)
		c.Redirect(http.StatusSeeOther, "/foods/"+strconv.FormatInt(id, 10)+"/images?error="+url.QueryEscape(userFacingUploadError(err)))
		return
	}

	c.Redirect(http.StatusSeeOther, "/foods/"+strconv.FormatInt(id, 10)+"/images?uploaded=1")
}

func (h Handler) DeleteFoodImage(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	name := strings.TrimSpace(c.PostForm("image"))
	if name == "" {
		c.Redirect(http.StatusSeeOther, "/foods/"+strconv.FormatInt(id, 10)+"/images?error="+url.QueryEscape("请选择要删除的图片"))
		return
	}

	if err := h.aiSettings.ImageGenerator().Delete(id, name); err != nil {
		h.deps.Logger.Warn("delete food image", "id", id, "image", name, "error", err)
		c.Redirect(http.StatusSeeOther, "/foods/"+strconv.FormatInt(id, 10)+"/images?error="+url.QueryEscape(userFacingDeleteImageError(err)))
		return
	}

	c.Redirect(http.StatusSeeOther, "/foods/"+strconv.FormatInt(id, 10)+"/images?deleted=1")
}

func (h Handler) Healthz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	checks := gin.H{
		"postgres": "ok",
		"redis":    "ok",
	}
	status := http.StatusOK

	if err := h.deps.DB.Ping(ctx); err != nil {
		checks["postgres"] = err.Error()
		status = http.StatusServiceUnavailable
	}

	if h.deps.Redis == nil {
		checks["redis"] = "unavailable"
		status = http.StatusServiceUnavailable
	} else if err := h.deps.Redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = err.Error()
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, gin.H{
		"status": statusText(status),
		"checks": checks,
	})
}

func statusText(status int) string {
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		return "ok"
	}
	return "unavailable"
}

func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}

func aiSettingsFormFromConfig(cfg config.AIConfig) aiSettingsForm {
	return aiSettingsForm{
		BaseURL:              cfg.BaseURL,
		EndpointPath:         cfg.EndpointPath,
		APIKeySet:            strings.TrimSpace(cfg.APIKey) != "",
		APIKeyEnv:            cfg.APIKeyEnv,
		Model:                cfg.Model,
		ImageCount:           strconv.Itoa(cfg.ImageCount),
		Size:                 cfg.Size,
		Quality:              cfg.Quality,
		OutputFormat:         cfg.OutputFormat,
		Timeout:              cfg.Timeout.String(),
		ResearchBaseURL:      cfg.ResearchBaseURL,
		ResearchEndpointPath: cfg.ResearchEndpointPath,
		ResearchAPIKeySet:    strings.TrimSpace(cfg.ResearchAPIKey) != "",
		ResearchAPIKeyEnv:    cfg.ResearchAPIKeyEnv,
		ResearchModel:        cfg.ResearchModel,
		ResearchToolType:     cfg.ResearchToolType,
		ResearchContextSize:  cfg.ResearchContextSize,
		ResearchTimeout:      cfg.ResearchTimeout.String(),
	}
}

func aiSettingsFormFromPost(c *gin.Context, current config.AIConfig) aiSettingsForm {
	form := aiSettingsForm{
		BaseURL:              strings.TrimSpace(c.PostForm("base_url")),
		EndpointPath:         strings.TrimSpace(c.PostForm("endpoint_path")),
		APIKey:               strings.TrimSpace(c.PostForm("api_key")),
		APIKeySet:            strings.TrimSpace(current.APIKey) != "",
		APIKeyEnv:            strings.TrimSpace(c.PostForm("api_key_env")),
		Model:                strings.TrimSpace(c.PostForm("model")),
		ImageCount:           strings.TrimSpace(c.PostForm("image_count")),
		Size:                 strings.TrimSpace(c.PostForm("size")),
		Quality:              strings.TrimSpace(c.PostForm("quality")),
		OutputFormat:         strings.TrimSpace(c.PostForm("output_format")),
		Timeout:              strings.TrimSpace(c.PostForm("timeout")),
		ResearchBaseURL:      strings.TrimSpace(c.PostForm("research_base_url")),
		ResearchEndpointPath: strings.TrimSpace(c.PostForm("research_endpoint_path")),
		ResearchAPIKey:       strings.TrimSpace(c.PostForm("research_api_key")),
		ResearchAPIKeySet:    strings.TrimSpace(current.ResearchAPIKey) != "",
		ResearchAPIKeyEnv:    strings.TrimSpace(c.PostForm("research_api_key_env")),
		ResearchModel:        strings.TrimSpace(c.PostForm("research_model")),
		ResearchToolType:     strings.TrimSpace(c.PostForm("research_tool_type")),
		ResearchContextSize:  strings.TrimSpace(c.PostForm("research_context_size")),
		ResearchTimeout:      strings.TrimSpace(c.PostForm("research_timeout")),
	}
	if form.APIKey != "" {
		form.APIKeySet = true
	}
	if c.PostForm("clear_api_key") != "" {
		form.APIKeySet = false
	}
	if form.ResearchAPIKey != "" {
		form.ResearchAPIKeySet = true
	}
	if c.PostForm("clear_research_api_key") != "" {
		form.ResearchAPIKeySet = false
	}
	return form
}

func (f aiSettingsForm) toConfig(current config.AIConfig, clearAPIKey, clearResearchAPIKey bool) (config.AIConfig, error) {
	updated := current

	baseURL := strings.TrimRight(strings.TrimSpace(f.BaseURL), "/")
	if baseURL == "" {
		return config.AIConfig{}, fmt.Errorf("图片生成 Base URL 不能为空")
	}
	model := strings.TrimSpace(f.Model)
	if model == "" {
		return config.AIConfig{}, fmt.Errorf("图片生成模型不能为空")
	}
	imageCount, err := strconv.Atoi(strings.TrimSpace(f.ImageCount))
	if err != nil || imageCount <= 0 {
		return config.AIConfig{}, fmt.Errorf("图片数量必须是大于 0 的数字")
	}
	timeout, err := time.ParseDuration(strings.TrimSpace(f.Timeout))
	if err != nil {
		return config.AIConfig{}, fmt.Errorf("图片生成超时时间格式不正确：%w", err)
	}

	researchBaseURL := strings.TrimRight(strings.TrimSpace(f.ResearchBaseURL), "/")
	if researchBaseURL == "" {
		return config.AIConfig{}, fmt.Errorf("资料检索 Base URL 不能为空")
	}
	researchModel := strings.TrimSpace(f.ResearchModel)
	if researchModel == "" {
		return config.AIConfig{}, fmt.Errorf("资料检索模型不能为空")
	}
	researchTimeout, err := time.ParseDuration(strings.TrimSpace(f.ResearchTimeout))
	if err != nil {
		return config.AIConfig{}, fmt.Errorf("资料检索超时时间格式不正确：%w", err)
	}

	updated.BaseURL = baseURL
	updated.EndpointPath = settingEndpointPath(f.EndpointPath, "/images/generations")
	updated.Model = model
	updated.ImageCount = imageCount
	updated.Size = fallbackSetting(f.Size, "720x1280")
	updated.Quality = fallbackSetting(f.Quality, "medium")
	updated.OutputFormat = fallbackSetting(f.OutputFormat, "png")
	updated.Timeout = timeout
	updated.APIKeyEnv = strings.TrimSpace(f.APIKeyEnv)
	if clearAPIKey {
		updated.APIKey = ""
	} else if strings.TrimSpace(f.APIKey) != "" {
		updated.APIKey = strings.TrimSpace(f.APIKey)
	}

	updated.ResearchBaseURL = researchBaseURL
	updated.ResearchEndpointPath = settingEndpointPath(f.ResearchEndpointPath, "/responses")
	updated.ResearchModel = researchModel
	updated.ResearchToolType = strings.TrimSpace(f.ResearchToolType)
	updated.ResearchContextSize = strings.TrimSpace(f.ResearchContextSize)
	updated.ResearchTimeout = researchTimeout
	updated.ResearchAPIKeyEnv = strings.TrimSpace(f.ResearchAPIKeyEnv)
	if clearResearchAPIKey {
		updated.ResearchAPIKey = ""
	} else if strings.TrimSpace(f.ResearchAPIKey) != "" {
		updated.ResearchAPIKey = strings.TrimSpace(f.ResearchAPIKey)
	}

	return updated, nil
}

func settingEndpointPath(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if !strings.HasPrefix(value, "/") {
		return "/" + value
	}
	return value
}

func fallbackSetting(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func isHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func userFacingGenerationError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "生成超时，请稍后重试。"
	}
	if errors.Is(err, ai.ErrTemporaryImageGeneration) {
		return "图片生成服务暂时繁忙或网关超时，系统已自动重试仍失败。请稍后再试。"
	}
	return err.Error()
}

func userFacingUploadError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func userFacingDeleteImageError(err error) string {
	if err == nil {
		return ""
	}
	return "删除图片失败：" + err.Error()
}

func userFacingResearchError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "检索超时，请稍后重试。"
	}
	if errors.Is(err, context.Canceled) {
		return "检索已取消，请重试。"
	}
	return "资料检索失败：" + err.Error()
}

func userFacingAISettingsError(err error) string {
	if err == nil {
		return ""
	}
	return "配置保存失败：" + err.Error()
}

func userFacingCreateFoodError(err error) string {
	if err == nil {
		return ""
	}
	if strings.Contains(err.Error(), "duplicate key") {
		return "这个 ID 已存在，请更换 ID 或留空自动生成。"
	}
	return "保存失败：" + err.Error()
}

func positiveQueryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
