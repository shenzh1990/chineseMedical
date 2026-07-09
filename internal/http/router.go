package http

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"chinese-medical/internal/ai"
	"chinese-medical/internal/config"
	"chinese-medical/internal/repository"
	"chinese-medical/internal/web"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Dependencies struct {
	Config config.Config
	DB     *pgxpool.Pool
	Redis  *redis.Client
	Logger *slog.Logger
}

func NewRouter(deps Dependencies) (*gin.Engine, error) {
	if deps.Config.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger(deps.Logger))

	templates, err := template.New("").Funcs(template.FuncMap{
		"urlquery": func(value string) string {
			return template.URLQueryEscaper(value)
		},
	}).ParseFS(web.Templates, "templates/*.html")
	if err != nil {
		return nil, err
	}
	router.SetHTMLTemplate(templates)

	staticFiles, err := fs.Sub(web.Static, "static")
	if err != nil {
		return nil, err
	}
	router.StaticFS("/static", http.FS(staticFiles))
	if err := os.MkdirAll(deps.Config.AI.OutputDir, 0755); err != nil {
		return nil, err
	}
	router.Static("/generated", deps.Config.AI.OutputDir)

	handler := Handler{
		deps:       deps,
		foods:      repository.NewMedicatedFoodRepository(deps.DB),
		renshu:     repository.NewRenShuDataRepository(deps.DB),
		users:      repository.NewUserRepository(deps.DB),
		aiSettings: ai.NewSettingsStore(deps.Config.AI),
	}
	router.GET("/login", handler.Login)
	router.POST("/login", handler.LoginPost)
	router.POST("/logout", handler.Logout)
	router.GET("/healthz", handler.Healthz)

	protected := router.Group("/")
	protected.Use(handler.requireAuth())
	protected.GET("/", handler.Index)
	protected.GET("/renshu/data", handler.RenShuData)
	protected.GET("/account/password", handler.ChangePassword)
	protected.POST("/account/password", handler.ChangePasswordPost)
	protected.GET("/tools/image-splitter", handler.ImageSplitter)
	protected.GET("/tcm/questions", handler.TCMQuestions)
	protected.POST("/tcm/questions/ask", handler.AskTCMQuestion)
	protected.GET("/settings/ai", handler.AISettings)
	protected.POST("/settings/ai", handler.SaveAISettings)
	protected.POST("/settings/ai/test", handler.TestAISettings)
	protected.GET("/foods/new", handler.NewFood)
	protected.POST("/foods/research", handler.ResearchFood)
	protected.POST("/foods", handler.CreateFood)
	protected.GET("/foods/:id/images", handler.FoodImages)
	protected.POST("/foods/:id/images/generate", handler.GenerateFoodImages)
	protected.POST("/foods/:id/images/upload", handler.UploadFoodImage)
	protected.POST("/foods/:id/images/delete", handler.DeleteFoodImage)

	api := router.Group("/api")
	api.GET("/health", handler.Healthz)

	return router, nil
}
