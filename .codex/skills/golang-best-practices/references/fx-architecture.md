# Uber Fx Dependency Injection Architecture

## Overview

Uber Fx is a dependency injection framework for Go that enables building modular, testable applications with explicit dependency management. This guide covers patterns for using Fx with handlers, services, and GORM.

## Project Structure with Fx

```
project/
├── cmd/
│   └── api/
│       └── main.go           # Application entry point
├── internal/
│   ├── config/               # Configuration management
│   │   ├── config.go
│   │   └── module.go         # Fx module
│   ├── handler/              # HTTP handlers (controllers)
│   │   ├── user_handler.go
│   │   ├── response.go       # Unified response helpers
│   │   └── module.go
│   ├── service/              # Business logic & data access layer
│   │   ├── user_service.go
│   │   └── module.go
│   ├── model/                # Domain models
│   │   ├── user.go
│   │   └── response.go       # Unified response structures
│   ├── middleware/           # HTTP middleware
│   │   ├── auth.go
│   │   └── module.go
│   └── infrastructure/       # Infrastructure concerns
│       ├── database/
│       │   ├── db.go
│       │   └── module.go
│       ├── router/
│       │   ├── router.go
│       │   └── module.go
│       └── server/
│           ├── server.go
│           └── module.go
├── config.yaml               # Configuration file
├── go.mod
└── go.sum
```

## Configuration Management

### Using Viper for Configuration

**config/config.go:**
```go
package config

import (
	"fmt"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// NewConfig loads configuration from file
func NewConfig(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.AutomaticEnv() // Read environment variables
	
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	
	return &cfg, nil
}
```

**config.yaml:**
```yaml
server:
  host: "0.0.0.0"
  port: 8080

database:
  driver: "mysql"
  host: "localhost"
  port: 3306
  username: "root"
  password: "password"
  database: "myapp"

redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0
```

**config/module.go:**
```go
package config

import "go.uber.org/fx"

// Module exports config dependency
var Module = fx.Module("config",
	fx.Provide(NewConfig),
)

// Alternative: with config file path parameter
func ModuleWithPath(configPath string) fx.Option {
	return fx.Module("config",
		fx.Provide(func() (*Config, error) {
			return NewConfig(configPath)
		}),
	)
}
```

## Database Layer with GORM

### Database Connection

**infrastructure/database/db.go:**
```go
package database

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/fx"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	
	"yourapp/internal/config"
)

// Database interface for dependency injection
type Database interface {
	GetDB() *gorm.DB
	Close() error
}

type database struct {
	db *gorm.DB
}

// NewDatabase creates a new database connection
func NewDatabase(lc fx.Lifecycle, cfg *config.Config) (Database, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
	)
	
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}
	
	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	
	d := &database{db: db}
	
	// Register lifecycle hooks
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return sqlDB.PingContext(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return d.Close()
		},
	})
	
	return d, nil
}

func (d *database) GetDB() *gorm.DB {
	return d.db
}

func (d *database) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
```

**infrastructure/database/module.go:**
```go
package database

import "go.uber.org/fx"

var Module = fx.Module("database",
	fx.Provide(
		fx.Annotate(
			NewDatabase,
			fx.As(new(Database)),
		),
	),
)
```

## Domain Models

**model/user.go:**
```go
package model

import (
	"time"
	"gorm.io/gorm"
)

type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Email     string         `gorm:"uniqueIndex;size:100;not null" json:"email"`
	Password  string         `gorm:"size:255;not null" json:"-"`
	Status    int            `gorm:"default:1;index" json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type UserCreateRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type UserUpdateRequest struct {
	Username string `json:"username" binding:"omitempty,min=3,max=50"`
	Email    string `json:"email" binding:"omitempty,email"`
	Status   *int   `json:"status" binding:"omitempty,oneof=0 1"`
}

type UserResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

**model/response.go:**
```go
package model

// Response is the unified API response structure
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// ErrorInfo contains error details
type ErrorInfo struct {
	Code    string                 `json:"code,omitempty"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// Meta contains pagination metadata
type Meta struct {
	Total     int64 `json:"total,omitempty"`
	Page      int   `json:"page,omitempty"`
	PageSize  int   `json:"page_size,omitempty"`
	TotalPage int   `json:"total_page,omitempty"`
}

// SuccessResponse creates a success response
func SuccessResponse(data interface{}) *Response {
	return &Response{
		Success: true,
		Data:    data,
	}
}

// SuccessResponseWithMessage creates a success response with a message
func SuccessResponseWithMessage(message string, data interface{}) *Response {
	return &Response{
		Success: true,
		Message: message,
		Data:    data,
	}
}

// SuccessResponseWithMeta creates a success response with pagination metadata
func SuccessResponseWithMeta(data interface{}, meta Meta) *Response {
	return &Response{
		Success: true,
		Data:    data,
		Meta:    &meta,
	}
}

// ErrorResponse creates an error response
func ErrorResponse(code, message string) *Response {
	return &Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	}
}

// ErrorResponseWithDetails creates an error response with details
func ErrorResponseWithDetails(code, message string, details map[string]interface{}) *Response {
	return &Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

// Common error codes
const (
	ErrCodeValidationFailed = "VALIDATION_FAILED"
	ErrCodeNotFound         = "NOT_FOUND"
	ErrCodeConflict         = "CONFLICT"
	ErrCodeUnauthorized     = "UNAUTHORIZED"
	ErrCodeForbidden        = "FORBIDDEN"
	ErrCodeInternalError    = "INTERNAL_ERROR"
	ErrCodeBadRequest       = "BAD_REQUEST"
)
```

## Service Layer (Business Logic & Data Access)

The service layer now includes both business logic and data access operations, eliminating the need for a separate repository layer.

**service/user_service.go:**
```go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"yourapp/internal/infrastructure/database"
	"yourapp/internal/model"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidPassword   = errors.New("invalid password")
)

// UserService defines the interface for user business logic
type UserService interface {
	CreateUser(ctx context.Context, req *model.UserCreateRequest) (*model.UserResponse, error)
	GetUser(ctx context.Context, id uint) (*model.UserResponse, error)
	UpdateUser(ctx context.Context, id uint, req *model.UserUpdateRequest) (*model.UserResponse, error)
	DeleteUser(ctx context.Context, id uint) error
	ListUsers(ctx context.Context, page, pageSize int) ([]*model.UserResponse, *model.Meta, error)
	Login(ctx context.Context, username, password string) (*model.UserResponse, error)
}

type userService struct {
	db database.Database
}

// NewUserService creates a new user service with database access
func NewUserService(db database.Database) UserService {
	return &userService{db: db}
}

func (s *userService) CreateUser(ctx context.Context, req *model.UserCreateRequest) (*model.UserResponse, error) {
	// Check if username exists
	var existingUser model.User
	err := s.db.GetDB().WithContext(ctx).
		Where("username = ?", req.Username).
		First(&existingUser).Error
	if err == nil {
		return nil, ErrUserAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("check username: %w", err)
	}

	// Check if email exists
	var existingEmail model.User
	err = s.db.GetDB().WithContext(ctx).
		Where("email = ?", req.Email).
		First(&existingEmail).Error
	if err == nil {
		return nil, ErrUserAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("check email: %w", err)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	// Create user
	user := &model.User{
		Username:  req.Username,
		Email:     req.Email,
		Password:  string(hashedPassword),
		Status:    1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.db.GetDB().WithContext(ctx).Create(user).Error; err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return s.toResponse(user), nil
}

func (s *userService) GetUser(ctx context.Context, id uint) (*model.UserResponse, error) {
	var user model.User
	err := s.db.GetDB().WithContext(ctx).First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return s.toResponse(&user), nil
}

func (s *userService) UpdateUser(ctx context.Context, id uint, req *model.UserUpdateRequest) (*model.UserResponse, error) {
	var user model.User
	err := s.db.GetDB().WithContext(ctx).First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Update fields
	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}
	if req.Username != "" {
		// Check if username is taken by another user
		var existing model.User
		if err := s.db.GetDB().WithContext(ctx).
			Where("username = ? AND id != ?", req.Username, id).
			First(&existing).Error; err == nil {
			return nil, ErrUserAlreadyExists
		}
		updates["username"] = req.Username
	}
	if req.Email != "" {
		// Check if email is taken by another user
		var existing model.User
		if err := s.db.GetDB().WithContext(ctx).
			Where("email = ? AND id != ?", req.Email, id).
			First(&existing).Error; err == nil {
			return nil, ErrUserAlreadyExists
		}
		updates["email"] = req.Email
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if err := s.db.GetDB().WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	// Refresh from database
	if err := s.db.GetDB().WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("refresh user: %w", err)
	}

	return s.toResponse(&user), nil
}

func (s *userService) DeleteUser(ctx context.Context, id uint) error {
	var user model.User
	err := s.db.GetDB().WithContext(ctx).First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("get user: %w", err)
	}

	if err := s.db.GetDB().WithContext(ctx).Delete(&user).Error; err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}

func (s *userService) ListUsers(ctx context.Context, page, pageSize int) ([]*model.UserResponse, *model.Meta, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	// Count total
	var total int64
	if err := s.db.GetDB().WithContext(ctx).Model(&model.User{}).Count(&total).Error; err != nil {
		return nil, nil, fmt.Errorf("count users: %w", err)
	}

	// Get users
	offset := (page - 1) * pageSize
	var users []*model.User
	err := s.db.GetDB().WithContext(ctx).
		Limit(pageSize).
		Offset(offset).
		Order("created_at DESC").
		Find(&users).Error
	if err != nil {
		return nil, nil, fmt.Errorf("list users: %w", err)
	}

	responses := make([]*model.UserResponse, len(users))
	for i, user := range users {
		responses[i] = s.toResponse(user)
	}

	meta := &model.Meta{
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
		TotalPage: int((total + int64(pageSize) - 1) / int64(pageSize)),
	}

	return responses, meta, nil
}

func (s *userService) Login(ctx context.Context, username, password string) (*model.UserResponse, error) {
	var user model.User
	err := s.db.GetDB().WithContext(ctx).
		Where("username = ?", username).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidPassword
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}

	return s.toResponse(&user), nil
}

func (s *userService) toResponse(user *model.User) *model.UserResponse {
	return &model.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
```

**service/module.go:**
```go
package service

import "go.uber.org/fx"

var Module = fx.Module("service",
	fx.Provide(
		fx.Annotate(
			NewUserService,
			fx.As(new(UserService)),
		),
	),
)
```

## Handler Layer (HTTP Controllers)

**handler/response.go - Unified Response Helpers:**
```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"yourapp/internal/model"
)

// JSON sends a unified JSON response
func JSON(c *gin.Context, statusCode int, response *model.Response) {
	c.JSON(statusCode, response)
}

// Success sends a success response with data
func Success(c *gin.Context, data interface{}) {
	JSON(c, http.StatusOK, model.SuccessResponse(data))
}

// SuccessCreated sends a success response with 201 status
func SuccessCreated(c *gin.Context, data interface{}) {
	JSON(c, http.StatusCreated, model.SuccessResponse(data))
}

// SuccessWithMessage sends a success response with message
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	JSON(c, http.StatusOK, model.SuccessResponseWithMessage(message, data))
}

// SuccessWithMeta sends a success response with pagination metadata
func SuccessWithMeta(c *gin.Context, data interface{}, meta model.Meta) {
	JSON(c, http.StatusOK, model.SuccessResponseWithMeta(data, meta))
}

// Error sends an error response
func Error(c *gin.Context, statusCode int, code, message string) {
	JSON(c, statusCode, model.ErrorResponse(code, message))
}

// ErrorWithDetails sends an error response with details
func ErrorWithDetails(c *gin.Context, statusCode int, code, message string, details map[string]interface{}) {
	JSON(c, statusCode, model.ErrorResponseWithDetails(code, message, details))
}

// BadRequest sends a 400 error response
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, model.ErrCodeBadRequest, message)
}

// Unauthorized sends a 401 error response
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, model.ErrCodeUnauthorized, message)
}

// Forbidden sends a 403 error response
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, model.ErrCodeForbidden, message)
}

// NotFound sends a 404 error response
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, model.ErrCodeNotFound, message)
}

// Conflict sends a 409 error response
func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, model.ErrCodeConflict, message)
}

// ValidationError sends a validation error response with details
func ValidationError(c *gin.Context, message string, details map[string]interface{}) {
	ErrorWithDetails(c, http.StatusBadRequest, model.ErrCodeValidationFailed, message, details)
}

// InternalError sends a 500 error response
func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, model.ErrCodeInternalError, message)
}
```

**handler/user_handler.go:**
```go
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"yourapp/internal/model"
	"yourapp/internal/service"
)

type UserHandler struct {
	userService service.UserService
}

// NewUserHandler creates a new user handler
func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// RegisterRoutes registers user routes
func (h *UserHandler) RegisterRoutes(router *gin.RouterGroup) {
	users := router.Group("/users")
	{
		users.POST("", h.CreateUser)
		users.GET("/:id", h.GetUser)
		users.PUT("/:id", h.UpdateUser)
		users.DELETE("/:id", h.DeleteUser)
		users.GET("", h.ListUsers)
	}

	router.POST("/login", h.Login)
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	var req model.UserCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	user, err := h.userService.CreateUser(c.Request.Context(), &req)
	if err != nil {
		if err == service.ErrUserAlreadyExists {
			Conflict(c, err.Error())
			return
		}
		InternalError(c, err.Error())
		return
	}

	SuccessCreated(c, user)
}

func (h *UserHandler) GetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}

	user, err := h.userService.GetUser(c.Request.Context(), uint(id))
	if err != nil {
		if err == service.ErrUserNotFound {
			NotFound(c, err.Error())
			return
		}
		InternalError(c, err.Error())
		return
	}

	Success(c, user)
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}

	var req model.UserUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	user, err := h.userService.UpdateUser(c.Request.Context(), uint(id), &req)
	if err != nil {
		if err == service.ErrUserNotFound {
			NotFound(c, err.Error())
			return
		}
		if err == service.ErrUserAlreadyExists {
			Conflict(c, err.Error())
			return
		}
		InternalError(c, err.Error())
		return
	}

	Success(c, user)
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		BadRequest(c, "invalid id")
		return
	}

	if err := h.userService.DeleteUser(c.Request.Context(), uint(id)); err != nil {
		if err == service.ErrUserNotFound {
			NotFound(c, err.Error())
			return
		}
		InternalError(c, err.Error())
		return
	}

	SuccessWithMessage(c, "user deleted successfully", nil)
}

func (h *UserHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	users, meta, err := h.userService.ListUsers(c.Request.Context(), page, pageSize)
	if err != nil {
		InternalError(c, err.Error())
		return
	}

	SuccessWithMeta(c, users, *meta)
}

func (h *UserHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	user, err := h.userService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if err == service.ErrInvalidPassword {
			Unauthorized(c, err.Error())
			return
		}
		InternalError(c, err.Error())
		return
	}

	Success(c, user)
}
```

**handler/module.go:**
```go
package handler

import "go.uber.org/fx"

var Module = fx.Module("handler",
	fx.Provide(NewUserHandler),
)
```

## Router Setup

**infrastructure/router/router.go:**
```go
package router

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

// Router wraps the Gin engine
type Router struct {
	*gin.Engine
}

// NewRouter creates a new Gin router with base middleware
func NewRouter() *Router {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	// Middleware
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())

	// Health check
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return &Router{Engine: engine}
}
```

**infrastructure/router/module.go:**
```go
package router

import (
	"go.uber.org/fx"

	"yourapp/internal/handler"
)

var Module = fx.Module("router",
	fx.Provide(NewRouter),

	// Register routes using fx.Invoke for cleaner DI
	fx.Invoke(func(r *Router, h *handler.UserHandler) {
		api := r.Group("/api/v1")
		h.RegisterRoutes(api)
	}),
)
```

## HTTP Server

**infrastructure/server/server.go:**
```go
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/fx"

	"yourapp/internal/config"
	"yourapp/internal/infrastructure/router"
)

// NewHTTPServer creates and starts the HTTP server
func NewHTTPServer(lc fx.Lifecycle, cfg *config.Config, router *router.Router) *http.Server {
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router.Engine,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				fmt.Printf("Server starting on %s\n", srv.Addr)
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					fmt.Printf("Server error: %v\n", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			fmt.Println("Server shutting down...")
			return srv.Shutdown(ctx)
		},
	})

	return srv
}
```

**infrastructure/server/module.go:**
```go
package server

import (
	"go.uber.org/fx"

	"yourapp/internal/infrastructure/router"
)

var Module = fx.Module("server",
	fx.Provide(NewHTTPServer),

	// Start server using fx.Invoke
	fx.Invoke(func(srv *http.Server, r *router.Router) {
		// Server is already started by fx.Lifecycle hooks in NewHTTPServer
		fmt.Printf("✓ Server configured with router at %s\n", r.Addr)
	}),
)
```
```

## Main Application

**cmd/api/main.go:**
```go
package main

import (
	"go.uber.org/fx"

	"yourapp/internal/config"
	"yourapp/internal/handler"
	"yourapp/internal/infrastructure/database"
	"yourapp/internal/infrastructure/router"
	"yourapp/internal/infrastructure/server"
	"yourapp/internal/service"
)

func main() {
	app := fx.New(
		// Provide config with file path
		fx.Provide(func() (*config.Config, error) {
			return config.NewConfig("config.yaml")
		}),

		// Infrastructure modules
		database.Module,

		// Service module (includes data access logic)
		service.Module,

		// Handler module
		handler.Module,

		// Router and server (routes are registered via fx.Invoke in each module)
		router.Module,
		server.Module,
	)

	app.Run()
}
```

## Testing with Fx

**service/user_service_test.go:**
```go
package service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"yourapp/internal/model"
	"yourapp/internal/infrastructure/database"
	"yourapp/internal/service"
)

// UserServiceTestSuite test suite for user service
type UserServiceTestSuite struct {
	suite.Suite
	db       *gorm.DB
	userSvc  service.UserService
}

// SetupSuite runs before all tests
func (s *UserServiceTestSuite) SetupSuite() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	s.Require().NoError(err)

	// Auto migrate tables
	err = db.AutoMigrate(&model.User{})
	s.Require().NoError(err)

	s.db = db

	// Create test database wrapper
	testDB := &testDatabase{db: db}

	// Create service
	s.userSvc = service.NewUserService(testDB)
}

// TearDownSuite runs after all tests
func (s *UserServiceTestSuite) TearDownSuite() {
	sqlDB, _ := s.db.DB()
	sqlDB.Close()
}

// SetupTest runs before each test
func (s *UserServiceTestSuite) SetupTest() {
	s.db.Exec("DELETE FROM users")
}

// testDatabase implements database.Database for testing
type testDatabase struct {
	db *gorm.DB
}

func (d *testDatabase) GetDB() *gorm.DB {
	return d.db
}

func (d *testDatabase) Close() error {
	sqlDB, _ := d.db.DB()
	return sqlDB.Close()
}

func (s *UserServiceTestSuite) TestCreateUser_Success() {
	req := &model.UserCreateRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}

	user, err := s.userSvc.CreateUser(context.Background(), req)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), user)
	assert.Equal(s.T(), "testuser", user.Username)
	assert.Equal(s.T(), "test@example.com", user.Email)
	assert.NotEqual(s.T(), "password123", "", "Password should be hashed")
}

func (s *UserServiceTestSuite) TestCreateUser_DuplicateUsername() {
	// Create first user
	req := &model.UserCreateRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}
	_, err := s.userSvc.CreateUser(context.Background(), req)
	assert.NoError(s.T(), err)

	// Try to create duplicate
	duplicateReq := &model.UserCreateRequest{
		Username: "testuser",
		Email:    "different@example.com",
		Password: "password456",
	}
	_, err = s.userSvc.CreateUser(context.Background(), duplicateReq)

	assert.Error(s.T(), err)
	assert.Equal(s.T(), service.ErrUserAlreadyExists, err)
}

func (s *UserServiceTestSuite) TestGetUser_Success() {
	// Create user
	req := &model.UserCreateRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
	}
	created, _ := s.userSvc.CreateUser(context.Background(), req)

	// Get user
	user, err := s.userSvc.GetUser(context.Background(), created.ID)

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), user)
	assert.Equal(s.T(), created.ID, user.ID)
	assert.Equal(s.T(), "testuser", user.Username)
}

func (s *UserServiceTestSuite) TestGetUser_NotFound() {
	_, err := s.userSvc.GetUser(context.Background(), 999)

	assert.Error(s.T(), err)
	assert.Equal(s.T(), service.ErrUserNotFound, err)
}

func (s *UserServiceTestSuite) TestListUsers_Pagination() {
	// Create multiple users
	for i := 1; i <= 5; i++ {
		req := &model.UserCreateRequest{
			Username: fmt.Sprintf("user%d", i),
			Email:    fmt.Sprintf("user%d@example.com", i),
			Password: "password123",
		}
		s.userSvc.CreateUser(context.Background(), req)
	}

	// Get first page
	users, meta, err := s.userSvc.ListUsers(context.Background(), 1, 2)

	assert.NoError(s.T(), err)
	assert.Len(s.T(), users, 2)
	assert.Equal(s.T(), int64(5), meta.Total)
	assert.Equal(s.T(), 1, meta.Page)
	assert.Equal(s.T(), 2, meta.PageSize)
	assert.Equal(s.T(), 3, meta.TotalPage)
}

// Run the test suite
func TestUserServiceSuite(t *testing.T) {
	suite.Run(t, new(UserServiceTestSuite))
}
```

**handler/user_handler_test.go:**
```go
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"yourapp/internal/handler"
	"yourapp/internal/model"
	"yourapp/internal/service"
)

// Setup test app with in-memory database
func setupTestApp(t *testing.T) (*gin.Engine, func()) {
	gin.SetMode(gin.TestMode)

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	assert.NoError(t, err)

	// Auto migrate
	err = db.AutoMigrate(&model.User{})
	assert.NoError(t, err)

	// Create test database wrapper
	testDB := &testDatabase{db: db}

	// Create service and handler
	userSvc := service.NewUserService(testDB)
	userHandler := handler.NewUserHandler(userSvc)

	// Setup router
	router := gin.New()
	userHandler.RegisterRoutes(router.Group("/api/v1"))

	// Cleanup function
	cleanup := func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}

	return router, cleanup
}

type testDatabase struct {
	db *gorm.DB
}

func (d *testDatabase) GetDB() *gorm.DB {
	return d.db
}

func (d *testDatabase) Close() error {
	sqlDB, _ := d.db.DB()
	return sqlDB.Close()
}

func TestUserHandler_CreateUser(t *testing.T) {
	router, cleanup := setupTestApp(t)
	defer cleanup()

	reqBody := map[string]interface{}{
		"username": "testuser",
		"email":    "test@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Data)
}

func TestUserHandler_CreateUser_ValidationError(t *testing.T) {
	router, cleanup := setupTestApp(t)
	defer cleanup()

	reqBody := map[string]interface{}{
		"username": "ab", // Too short
		"email":    "test@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp model.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.False(t, resp.Success)
	assert.NotNil(t, resp.Error)
}
```

## Best Practices Summary

1. **Single Responsibility**: Each layer has one clear purpose
   - Handlers: HTTP request/response handling using unified response helpers
   - Services: Business logic combined with data access
   - Models: Domain models and unified response structures

2. **Unified API Response**: Consistent response format across all endpoints
   - Use `model.Response` structure with `success`, `data`, `error`, and `meta` fields
   - Leverage helper functions in `handler/response.go` (Success, Error, BadRequest, NotFound, etc.)
   - Standardized error codes (VALIDATION_FAILED, NOT_FOUND, CONFLICT, UNAUTHORIZED, etc.)
   - Pagination metadata included in responses for list endpoints

3. **Simplified Architecture**: Optional repository layer
   - Data access logic can be directly implemented in the service layer
   - Reduces boilerplate and simplifies the codebase
   - Repository layer can be added when needed for complex data access patterns
   - Services receive `database.Database` interface for direct GORM access

4. **Clean Dependency Injection with Fx**: Use `fx.Invoke` for initialization
   - Define interfaces for services
   - Use `fx.Annotate` with `fx.As` for interface binding
   - Group related providers in modules
   - Use `fx.Invoke` within modules to register routes and perform side effects
   - No need for explicit invoke in `main()` - modules handle their own initialization
   - Example: Route registration via `fx.Invoke(func(r *Router, h *handler.UserHandler) { ... })`

5. **Configuration**: Use Viper for flexible configuration
   - Support YAML/TOML files
   - Allow environment variable overrides
   - Type-safe configuration structs

6. **Database**: GORM with clean abstractions
   - Optional repository pattern for data access
   - Direct GORM usage in service layer is acceptable
   - Context propagation for cancellation
   - Connection pooling configuration

7. **Lifecycle Management**: Use Fx hooks
   - OnStart for initialization
   - OnStop for cleanup
   - Graceful shutdown handling

8. **Testing**: Use in-memory databases for integration tests
   - Use SQLite in-memory database for faster tests
   - Test suites with setup/teardown for proper isolation
   - Handler tests use httptest for HTTP level testing
   - Response structure validation in tests
