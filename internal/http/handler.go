package http

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"chinese-medical/internal/ai"
	"chinese-medical/internal/model"
	"chinese-medical/internal/repository"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	deps      Dependencies
	foods     repository.MedicatedFoodRepository
	users     repository.UserRepository
	generator ai.ImageGenerator
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
				"Env":     h.deps.Config.Env,
				"Error":   "数据读取失败，请先执行 SQL 同步或检查数据库连接。",
			})
			return
		}
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"AppName":       h.deps.Config.AppName,
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

func (h Handler) ImageSplitter(c *gin.Context) {
	c.HTML(http.StatusOK, "image_splitter.html", gin.H{
		"AppName": h.deps.Config.AppName,
	})
}

func (h Handler) NewFood(c *gin.Context) {
	h.renderFoodForm(c, http.StatusOK, "", medicatedFoodForm{
		Category: model.DefaultFoodCategory,
	})
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
			"Error":   "未找到这个调理方。",
		})
		return
	}

	images, err := h.generator.Existing(id)
	if err != nil {
		h.deps.Logger.Warn("list generated images", "id", id, "error", err)
	}

	c.HTML(http.StatusOK, "food_images.html", gin.H{
		"AppName":    h.deps.Config.AppName,
		"Food":       item,
		"Images":     images,
		"ImageCount": h.deps.Config.AI.ImageCount,
		"Model":      h.deps.Config.AI.Model,
		"BaseURL":    h.deps.Config.AI.BaseURL,
		"Prompt":     h.generator.Prompt(item),
		"Error":      c.Query("error"),
		"Created":    c.Query("created") == "1",
		"Generated":  c.Query("generated") == "1",
		"Uploaded":   c.Query("uploaded") == "1",
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

	generationCtx, generationCancel := context.WithTimeout(c.Request.Context(), h.deps.Config.AI.Timeout)
	defer generationCancel()

	if _, err := h.generator.Generate(generationCtx, item); err != nil {
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

	if _, err := h.generator.SaveUploaded(id, fileHeader.Filename, file); err != nil {
		h.deps.Logger.Warn("save uploaded food image", "id", id, "error", err)
		c.Redirect(http.StatusSeeOther, "/foods/"+strconv.FormatInt(id, 10)+"/images?error="+url.QueryEscape(userFacingUploadError(err)))
		return
	}

	c.Redirect(http.StatusSeeOther, "/foods/"+strconv.FormatInt(id, 10)+"/images?uploaded=1")
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

func userFacingGenerationError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "生成超时，请稍后重试。"
	}
	return err.Error()
}

func userFacingUploadError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
