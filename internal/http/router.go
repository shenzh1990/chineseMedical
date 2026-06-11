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
		deps:      deps,
		foods:     repository.NewMedicatedFoodRepository(deps.DB),
		generator: ai.NewImageGenerator(deps.Config.AI),
	}
	router.GET("/", handler.Index)
	router.GET("/foods/:id/images", handler.FoodImages)
	router.POST("/foods/:id/images/generate", handler.GenerateFoodImages)
	router.GET("/healthz", handler.Healthz)

	api := router.Group("/api")
	api.GET("/health", handler.Healthz)

	return router, nil
}
