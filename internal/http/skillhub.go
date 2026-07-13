package http

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"chinese-medical/internal/model"
	"chinese-medical/internal/repository"

	"github.com/gin-gonic/gin"
)

type skillForm struct {
	ID          string
	Name        string
	Slug        string
	Category    string
	Description string
	Instruction string
	Tags        string
	Enabled     bool
}

func (h Handler) SkillHub(c *gin.Context) {
	form := skillForm{
		Category: "中医问答",
		Enabled:  true,
	}
	editing := false
	editID := positiveQueryInt(c, "edit", 0)
	if editID > 0 {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		skill, err := h.skills.Get(ctx, int64(editID))
		if err != nil {
			h.deps.Logger.Warn("get skill for edit", "id", editID, "error", err)
			h.renderSkillHub(c, http.StatusNotFound, "未找到这个 Skill。", "", form, false)
			return
		}
		form = skillFormFromModel(skill)
		editing = true
	}

	h.renderSkillHub(c, http.StatusOK, "", skillHubSuccessMessage(c), form, editing)
}

func (h Handler) CreateSkill(c *gin.Context) {
	form := skillFormFromPost(c)
	if message := validateSkillForm(form); message != "" {
		h.renderSkillHub(c, http.StatusBadRequest, message, "", form, false)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if _, err := h.skills.Create(ctx, form.toModel(0)); err != nil {
		h.deps.Logger.Warn("create skill", "slug", form.Slug, "error", err)
		h.renderSkillHub(c, http.StatusInternalServerError, userFacingSkillError(err), "", form, false)
		return
	}

	c.Redirect(http.StatusSeeOther, "/skillhub?created=1")
}

func (h Handler) UpdateSkill(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	form := skillFormFromPost(c)
	form.ID = strconv.FormatInt(id, 10)
	if message := validateSkillForm(form); message != "" {
		h.renderSkillHub(c, http.StatusBadRequest, message, "", form, true)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if _, err := h.skills.Update(ctx, form.toModel(id)); err != nil {
		h.deps.Logger.Warn("update skill", "id", id, "slug", form.Slug, "error", err)
		h.renderSkillHub(c, http.StatusInternalServerError, userFacingSkillError(err), "", form, true)
		return
	}

	c.Redirect(http.StatusSeeOther, "/skillhub?updated=1")
}

func (h Handler) ToggleSkill(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	enabled := c.PostForm("enabled") == "1"

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if err := h.skills.SetEnabled(ctx, id, enabled); err != nil {
		h.deps.Logger.Warn("toggle skill", "id", id, "enabled", enabled, "error", err)
		c.Redirect(http.StatusSeeOther, "/skillhub?error="+url.QueryEscape("Skill 状态更新失败"))
		return
	}

	c.Redirect(http.StatusSeeOther, "/skillhub?toggled=1")
}

func (h Handler) DeleteSkill(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if err := h.skills.Delete(ctx, id); err != nil {
		h.deps.Logger.Warn("delete skill", "id", id, "error", err)
		c.Redirect(http.StatusSeeOther, "/skillhub?error="+url.QueryEscape("Skill 删除失败"))
		return
	}

	c.Redirect(http.StatusSeeOther, "/skillhub?deleted=1")
}

func (h Handler) renderSkillHub(c *gin.Context, status int, errorMessage, successMessage string, form skillForm, editing bool) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	page := positiveQueryInt(c, "page", 1)
	limit := positiveQueryInt(c, "limit", 12)
	if limit > 48 {
		limit = 48
	}
	query := strings.TrimSpace(c.Query("q"))
	category := strings.TrimSpace(c.Query("category"))
	enabledFilter := normalizeSkillEnabledFilter(c.Query("enabled"))
	offset := (page - 1) * limit

	filter := repository.SkillFilter{
		Query:    query,
		Category: category,
		Enabled:  enabledFilter,
	}

	categories, err := h.skills.Categories(ctx)
	if err != nil {
		h.deps.Logger.Error("list skill categories", "error", err)
		errorMessage = "SkillHub 分类读取失败，请检查数据库连接。"
		categories = nil
	}

	skills, summary, err := h.skills.List(ctx, filter, limit, offset)
	if err != nil {
		h.deps.Logger.Error("list skills", "error", err)
		errorMessage = "SkillHub 数据读取失败，请检查数据库连接。"
		skills = nil
		summary = model.SkillHubSummary{}
	}

	totalPages := 0
	if summary.Total > 0 {
		totalPages = int((summary.Total + int64(limit) - 1) / int64(limit))
	}

	c.HTML(status, "skillhub.html", gin.H{
		"AppName":       h.deps.Config.AppName,
		"Active":        "skillhub",
		"Error":         firstNonEmpty(errorMessage, c.Query("error")),
		"Success":       successMessage,
		"Skills":        skills,
		"Summary":       summary,
		"Categories":    categories,
		"Form":          form,
		"Editing":       editing,
		"Query":         query,
		"Category":      category,
		"EnabledFilter": enabledFilter,
		"HasFilters":    query != "" || category != "" || enabledFilter != "",
		"Page":          page,
		"Limit":         limit,
		"TotalPages":    totalPages,
		"HasPrev":       page > 1,
		"HasNext":       int64(offset+len(skills)) < summary.Total,
		"PrevPage":      page - 1,
		"NextPage":      page + 1,
	})
}

func skillFormFromPost(c *gin.Context) skillForm {
	return skillForm{
		Name:        strings.TrimSpace(c.PostForm("name")),
		Slug:        normalizeSkillSlug(c.PostForm("slug")),
		Category:    fallbackSetting(c.PostForm("category"), "通用"),
		Description: strings.TrimSpace(c.PostForm("description")),
		Instruction: strings.TrimSpace(c.PostForm("instruction")),
		Tags:        strings.TrimSpace(c.PostForm("tags")),
		Enabled:     c.PostForm("enabled") != "",
	}
}

func skillFormFromModel(skill model.Skill) skillForm {
	return skillForm{
		ID:          strconv.FormatInt(skill.ID, 10),
		Name:        skill.Name,
		Slug:        skill.Slug,
		Category:    skill.Category,
		Description: skill.Description,
		Instruction: skill.Instruction,
		Tags:        skill.Tags,
		Enabled:     skill.Enabled,
	}
}

func (f skillForm) toModel(id int64) model.Skill {
	return model.Skill{
		ID:          id,
		Name:        f.Name,
		Slug:        f.Slug,
		Category:    fallbackSetting(f.Category, "通用"),
		Description: f.Description,
		Instruction: f.Instruction,
		Tags:        f.Tags,
		Enabled:     f.Enabled,
	}
}

func validateSkillForm(form skillForm) string {
	if form.Name == "" {
		return "请填写 Skill 名称。"
	}
	if len([]rune(form.Name)) > 80 {
		return "Skill 名称请控制在 80 个字以内。"
	}
	if form.Slug == "" {
		return "请填写调用标识。"
	}
	if !isValidSkillSlug(form.Slug) {
		return "调用标识只能使用小写字母、数字、下划线或中划线，并且长度为 2-64 位。"
	}
	if len([]rune(form.Category)) > 40 {
		return "分类请控制在 40 个字以内。"
	}
	if len([]rune(form.Description)) > 500 {
		return "简介请控制在 500 个字以内。"
	}
	if form.Instruction == "" {
		return "请填写 Skill 指令。"
	}
	return ""
}

func normalizeSkillSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func isValidSkillSlug(value string) bool {
	if len(value) < 2 || len(value) > 64 {
		return false
	}
	for _, item := range value {
		if (item >= 'a' && item <= 'z') || (item >= '0' && item <= '9') || item == '_' || item == '-' {
			continue
		}
		return false
	}
	return true
}

func normalizeSkillEnabledFilter(value string) string {
	switch strings.TrimSpace(value) {
	case "enabled", "disabled":
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

func skillHubSuccessMessage(c *gin.Context) string {
	switch {
	case c.Query("created") == "1":
		return "Skill 已创建。"
	case c.Query("updated") == "1":
		return "Skill 已更新。"
	case c.Query("toggled") == "1":
		return "Skill 状态已更新。"
	case c.Query("deleted") == "1":
		return "Skill 已删除。"
	default:
		return ""
	}
}

func userFacingSkillError(err error) string {
	if err == nil {
		return ""
	}
	if strings.Contains(err.Error(), "duplicate key") {
		return "调用标识已存在，请换一个。"
	}
	return "保存 Skill 失败：" + err.Error()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
