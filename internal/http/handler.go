package http

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"chinese-medical/internal/ai"
	"chinese-medical/internal/repository"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	deps      Dependencies
	foods     repository.MedicatedFoodRepository
	generator ai.ImageGenerator
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
	offset := (page - 1) * limit

	items, total, err := h.foods.List(ctx, query, limit, offset)
	if err != nil {
		h.deps.Logger.Error("list medicated foods", "error", err)
		c.HTML(http.StatusInternalServerError, "index.html", gin.H{
			"AppName": h.deps.Config.AppName,
			"Env":     h.deps.Config.Env,
			"Error":   "数据读取失败，请先执行 SQL 同步或检查数据库连接。",
		})
		return
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"AppName":  h.deps.Config.AppName,
		"Env":      h.deps.Config.Env,
		"Foods":    items,
		"Total":    total,
		"Query":    query,
		"Page":     page,
		"Limit":    limit,
		"HasPrev":  page > 1,
		"HasNext":  int64(offset+len(items)) < total,
		"PrevPage": page - 1,
		"NextPage": page + 1,
	})
}

func (h Handler) ImageSplitter(c *gin.Context) {
	c.HTML(http.StatusOK, "image_splitter.html", gin.H{
		"AppName": h.deps.Config.AppName,
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

func positiveQueryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
