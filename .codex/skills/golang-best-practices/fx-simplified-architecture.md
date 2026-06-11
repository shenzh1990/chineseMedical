---
skill_name: "Golang Uber FX 简化架构实战技能"
version: "3.0"
author: "优化简化版"
last_updated: "2026-02-06"
difficulty: "中级"
tags: ["golang", "fx", "gin", "simplified", "clean-architecture"]
source_repo: "https://github.com/yangjian102621/geekai"
---

# 🎯 技能目标

掌握简化版 Uber FX 依赖注入架构,采用 **Handler 直接访问数据库** 的模式,每个 Handler 自管理路由,适合快速开发的中小型项目。

## 架构原则

✅ **扁平化设计** - 移除 Repository 层  
✅ **自治路由** - 每个 Handler 自带 `RegisterRoutes`  
✅ **职责清晰** - Handler 处理 HTTP + 数据访问,Service 处理复杂业务逻辑  
✅ **快速开发** - 减少抽象层,提高开发效率  

---

# 📁 简化后的项目结构

```
project/
├── cmd/
│   └── server/
│       └── main.go              # 应用入口 + FX 配置
├── internal/
│   ├── handler/                 # HTTP Handler层(含数据访问)
│   │   ├── health.go           # 健康检查 Handler
│   │   ├── user.go             # 用户 Handler
│   │   ├── chat.go             # 聊天 Handler
│   │   └── admin/              # 管理后台
│   │       ├── dashboard.go    # 仪表盘 Handler
│   │       └── user_manage.go  # 用户管理 Handler
│   ├── service/                 # 业务逻辑层(复杂逻辑)
│   │   ├── email.go            # 邮件服务
│   │   ├── sms.go              # 短信服务
│   │   └── payment/            # 支付服务
│   │       ├── alipay.go
│   │       └── wechat.go
│   ├── model/                   # 数据模型
│   │   └── entities.go
│   ├── middleware/              # 中间件
│   │   ├── auth.go
│   │   ├── cors.go
│   │   └── logger.go
│   ├── config/                  # 配置管理
│   │   └── config.go
│   └── pkg/                     # 内部工具
│       └── database/
│           └── db.go
├── config/
│   └── config.yaml
├── go.mod
└── go.sum
```

---

# 1️⃣ 核心模式:Handler 自管理路由

## 标准 Handler 模板

```go name=internal/handler/health.go
package handler

import (
    "github.com/gin-gonic/gin"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
    return &HealthHandler{}
}

// Health godoc
// @Summary      Health check
// @Description  Check if the API service is running
// @Tags         health
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]bool  "ok: true"
// @Router       /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
    c.JSON(200, gin.H{"ok": true})
}

// RegisterRoutes 注册路由
func (h *HealthHandler) RegisterRoutes(r *gin.Engine) {
    r.GET("/health", h.Health)
}
```

---

## 带数据库访问的 Handler

```go name=internal/handler/user.go
package handler

import (
    "net/http"
    "strconv"
    
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
    
    "your-project/internal/middleware"
    "your-project/internal/model"
    "your-project/internal/service"
)

type UserHandler struct {
    db           *gorm.DB
    emailService *service.EmailService  // 可选:注入复杂服务
}

func NewUserHandler(db *gorm.DB, emailService *service.EmailService) *UserHandler {
    return &UserHandler{
        db:           db,
        emailService: emailService,
    }
}

// Register godoc
// @Summary      User registration
// @Description  Register a new user account
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        request  body      RegisterRequest  true  "Registration info"
// @Success      200      {object}  UserResponse
// @Failure      400      {object}  ErrorResponse
// @Router       /api/user/register [post]
func (h *UserHandler) Register(c *gin.Context) {
    var req RegisterRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // 数据验证
    if len(req.Password) < 8 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "password too short"})
        return
    }
    
    // 直接操作数据库
    user := &model.User{
        Username: req.Username,
        Email:    req.Email,
        Password: hashPassword(req.Password),
    }
    
    if err := h.db.Create(user).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
        return
    }
    
    // 调用复杂服务
    go h.emailService.SendWelcome(user.Email)
    
    c.JSON(http.StatusOK, gin.H{"data": toUserResponse(user)})
}

// Login godoc
// @Summary      User login
// @Description  Authenticate user and return token
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        request  body      LoginRequest  true  "Login credentials"
// @Success      200      {object}  TokenResponse
// @Failure      401      {object}  ErrorResponse
// @Router       /api/user/login [post]
func (h *UserHandler) Login(c *gin.Context) {
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // 查询数据库
    var user model.User
    if err := h.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
        return
    }
    
    // 验证密码
    if !verifyPassword(user.Password, req.Password) {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
        return
    }
    
    // 生成 token
    token := generateToken(user.ID)
    
    c.JSON(http.StatusOK, gin.H{"token": token})
}

// GetProfile godoc
// @Summary      Get user profile
// @Description  Get current user profile information
// @Tags         user
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  UserResponse
// @Failure      401  {object}  ErrorResponse
// @Router       /api/user/profile [get]
func (h *UserHandler) GetProfile(c *gin.Context) {
    userID := c.GetUint("user_id")  // 从中间件获取
    
    var user model.User
    if err := h.db.First(&user, userID).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"data": toUserResponse(&user)})
}

// UpdateProfile godoc
// @Summary      Update user profile
// @Description  Update current user profile information
// @Tags         user
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      UpdateProfileRequest  true  "Profile info"
// @Success      200      {object}  UserResponse
// @Failure      400      {object}  ErrorResponse
// @Router       /api/user/profile [put]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
    userID := c.GetUint("user_id")
    
    var req UpdateProfileRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // 更新数据库
    updates := map[string]interface{}{
        "nickname": req.Nickname,
        "avatar":   req.Avatar,
    }
    
    if err := h.db.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update profile"})
        return
    }
    
    // 返回更新后的用户信息
    var user model.User
    h.db.First(&user, userID)
    
    c.JSON(http.StatusOK, gin.H{"data": toUserResponse(&user)})
}

// DeleteAccount godoc
// @Summary      Delete user account
// @Description  Permanently delete current user account
// @Tags         user
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  map[string]bool
// @Failure      500  {object}  ErrorResponse
// @Router       /api/user/account [delete]
func (h *UserHandler) DeleteAccount(c *gin.Context) {
    userID := c.GetUint("user_id")
    
    if err := h.db.Delete(&model.User{}, userID).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete account"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RegisterRoutes 注册所有用户相关路由
func (h *UserHandler) RegisterRoutes(r *gin.Engine) {
    api := r.Group("/api/user")
    {
        // 公开路由
        api.POST("/register", h.Register)
        api.POST("/login", h.Login)
        
        // 需要认证的路由
        auth := api.Group("", middleware.Auth())
        {
            auth.GET("/profile", h.GetProfile)
            auth.PUT("/profile", h.UpdateProfile)
            auth.DELETE("/account", h.DeleteAccount)
        }
    }
}

// DTO 定义
type RegisterRequest struct {
    Username string `json:"username" binding:"required"`
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

type UpdateProfileRequest struct {
    Nickname string `json:"nickname"`
    Avatar   string `json:"avatar"`
}

type UserResponse struct {
    ID       uint   `json:"id"`
    Username string `json:"username"`
    Email    string `json:"email"`
    Nickname string `json:"nickname"`
    Avatar   string `json:"avatar"`
}

type TokenResponse struct {
    Token string `json:"token"`
}

type ErrorResponse struct {
    Error string `json:"error"`
}

// 辅助函数
func toUserResponse(user *model.User) UserResponse {
    return UserResponse{
        ID:       user.ID,
        Username: user.Username,
        Email:    user.Email,
        Nickname: user.Nickname,
        Avatar:   user.Avatar,
    }
}

func hashPassword(password string) string {
    // 实现密码哈希
    return password
}

func verifyPassword(hashedPassword, password string) bool {
    // 实现密码验证
    return hashedPassword == password
}

func generateToken(userID uint) string {
    // 实现 JWT token 生成
    return "token"
}
```

---

## 带关联查询的 Handler

```go name=internal/handler/chat.go
package handler

import (
    "net/http"
    
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
    
    "your-project/internal/middleware"
    "your-project/internal/model"
)

type ChatHandler struct {
    db *gorm.DB
}

func NewChatHandler(db *gorm.DB) *ChatHandler {
    return &ChatHandler{db: db}
}

// List godoc
// @Summary      List chat sessions
// @Description  Get all chat sessions for current user
// @Tags         chat
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   ChatResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/chat/list [get]
func (h *ChatHandler) List(c *gin.Context) {
    userID := c.GetUint("user_id")
    
    var chats []model.Chat
    // 预加载关联数据
    if err := h.db.Where("user_id = ?", userID).
        Preload("Messages").
        Order("updated_at DESC").
        Find(&chats).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch chats"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"data": chats})
}

// Create godoc
// @Summary      Create chat session
// @Description  Create a new chat session
// @Tags         chat
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      CreateChatRequest  true  "Chat info"
// @Success      200      {object}  ChatResponse
// @Failure      400      {object}  ErrorResponse
// @Router       /api/chat [post]
func (h *ChatHandler) Create(c *gin.Context) {
    userID := c.GetUint("user_id")
    
    var req CreateChatRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    chat := &model.Chat{
        UserID: userID,
        Title:  req.Title,
    }
    
    if err := h.db.Create(chat).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create chat"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"data": chat})
}

// SendMessage godoc
// @Summary      Send message
// @Description  Send a message in a chat session
// @Tags         chat
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      int                 true  "Chat ID"
// @Param        request  body      SendMessageRequest  true  "Message content"
// @Success      200      {object}  MessageResponse
// @Failure      400      {object}  ErrorResponse
// @Router       /api/chat/{id}/message [post]
func (h *ChatHandler) SendMessage(c *gin.Context) {
    userID := c.GetUint("user_id")
    chatID := c.Param("id")
    
    var req SendMessageRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // 验证 chat 归属
    var chat model.Chat
    if err := h.db.Where("id = ? AND user_id = ?", chatID, userID).First(&chat).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "chat not found"})
        return
    }
    
    // 创建消息
    message := &model.ChatMessage{
        ChatID:  chat.ID,
        Content: req.Content,
        Role:    "user",
    }
    
    if err := h.db.Create(message).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send message"})
        return
    }
    
    // TODO: 调用 AI 服务生成回复
    
    c.JSON(http.StatusOK, gin.H{"data": message})
}

// Delete godoc
// @Summary      Delete chat session
// @Description  Delete a chat session and all its messages
// @Tags         chat
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      int  true  "Chat ID"
// @Success      200  {object}  map[string]bool
// @Failure      404  {object}  ErrorResponse
// @Router       /api/chat/{id} [delete]
func (h *ChatHandler) Delete(c *gin.Context) {
    userID := c.GetUint("user_id")
    chatID := c.Param("id")
    
    // 使用事务删除
    err := h.db.Transaction(func(tx *gorm.DB) error {
        // 先删除消息
        if err := tx.Where("chat_id = ?", chatID).Delete(&model.ChatMessage{}).Error; err != nil {
            return err
        }
        
        // 再删除会话
        if err := tx.Where("id = ? AND user_id = ?", chatID, userID).Delete(&model.Chat{}).Error; err != nil {
            return err
        }
        
        return nil
    })
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete chat"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RegisterRoutes 注册聊天相关路由
func (h *ChatHandler) RegisterRoutes(r *gin.Engine) {
    api := r.Group("/api/chat", middleware.Auth())
    {
        api.GET("/list", h.List)
        api.POST("", h.Create)
        api.POST("/:id/message", h.SendMessage)
        api.DELETE("/:id", h.Delete)
    }
}

// DTO 定义
type CreateChatRequest struct {
    Title string `json:"title" binding:"required"`
}

type SendMessageRequest struct {
    Content string `json:"content" binding:"required"`
}

type ChatResponse struct {
    ID        uint      `json:"id"`
    Title     string    `json:"title"`
    CreatedAt time.Time `json:"created_at"`
}

type MessageResponse struct {
    ID        uint      `json:"id"`
    Content   string    `json:"content"`
    Role      string    `json:"role"`
    CreatedAt time.Time `json:"created_at"`
}
```

---

## 管理后台 Handler

```go name=internal/handler/admin/dashboard.go
package admin

import (
    "net/http"
    
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
    
    "your-project/internal/middleware"
    "your-project/internal/model"
)

type DashboardHandler struct {
    db *gorm.DB
}

func NewDashboardHandler(db *gorm.DB) *DashboardHandler {
    return &DashboardHandler{db: db}
}

// Stats godoc
// @Summary      Dashboard statistics
// @Description  Get dashboard statistics data
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  StatsResponse
// @Router       /api/admin/dashboard/stats [get]
func (h *DashboardHandler) Stats(c *gin.Context) {
    var stats StatsResponse
    
    // 用户总数
    h.db.Model(&model.User{}).Count(&stats.TotalUsers)
    
    // 今日新增用户
    today := time.Now().Truncate(24 * time.Hour)
    h.db.Model(&model.User{}).Where("created_at >= ?", today).Count(&stats.TodayUsers)
    
    // 聊天总数
    h.db.Model(&model.Chat{}).Count(&stats.TotalChats)
    
    // 消息总数
    h.db.Model(&model.ChatMessage{}).Count(&stats.TotalMessages)
    
    c.JSON(http.StatusOK, gin.H{"data": stats})
}

// UserList godoc
// @Summary      User list
// @Description  Get paginated user list
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page      query     int  false  "Page number"
// @Param        pageSize  query     int  false  "Page size"
// @Success      200       {object}  UserListResponse
// @Router       /api/admin/users [get]
func (h *DashboardHandler) UserList(c *gin.Context) {
    page := c.DefaultQuery("page", "1")
    pageSize := c.DefaultQuery("pageSize", "20")
    
    var users []model.User
    var total int64
    
    offset := (page - 1) * pageSize
    
    h.db.Model(&model.User{}).Count(&total)
    h.db.Offset(offset).Limit(pageSize).Find(&users)
    
    c.JSON(http.StatusOK, gin.H{
        "data":  users,
        "total": total,
        "page":  page,
    })
}

// RegisterRoutes 注册管理后台路由
func (h *DashboardHandler) RegisterRoutes(r *gin.Engine) {
    admin := r.Group("/api/admin", middleware.AdminAuth())
    {
        admin.GET("/dashboard/stats", h.Stats)
        admin.GET("/users", h.UserList)
    }
}

// DTO 定义
type StatsResponse struct {
    TotalUsers    int64 `json:"total_users"`
    TodayUsers    int64 `json:"today_users"`
    TotalChats    int64 `json:"total_chats"`
    TotalMessages int64 `json:"total_messages"`
}

type UserListResponse struct {
    Data  []model.User `json:"data"`
    Total int64        `json:"total"`
    Page  int          `json:"page"`
}
```

---

# 2️⃣ Main.go 完整示例

```go name=cmd/server/main.go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/gin-gonic/gin"
    "go.uber.org/fx"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
    
    "your-project/internal/config"
    "your-project/internal/handler"
    "your-project/internal/handler/admin"
    "your-project/internal/middleware"
    "your-project/internal/model"
    "your-project/internal/service"
)

func main() {
    app := fx.New(
        // ==================== 配置层 ====================
        fx.Provide(config.Load),
        
        // ==================== 数据库层 ====================
        fx.Provide(NewDatabase),
        
        // ==================== 服务层 ====================
        fx.Provide(service.NewEmailService),
        fx.Provide(service.NewSMSService),
        fx.Provide(service.NewPaymentService),
        
        // ==================== Handler 层 ====================
        fx.Provide(handler.NewHealthHandler),
        fx.Provide(handler.NewUserHandler),
        fx.Provide(handler.NewChatHandler),
        fx.Provide(admin.NewDashboardHandler),
        
        // ==================== HTTP 服务器 ====================
        fx.Provide(NewGinEngine),
        fx.Provide(NewHTTPServer),
        
        // ==================== 路由注册 (分散式) ====================
        // 每个 Handler 独立注册，方便模块化管理
        fx.Invoke(func(h *handler.HealthHandler, engine *gin.Engine) {
            h.RegisterRoutes(engine)
            log.Println("✓ Health routes registered")
        }),
        fx.Invoke(func(h *handler.UserHandler, engine *gin.Engine) {
            h.RegisterRoutes(engine)
            log.Println("✓ User routes registered")
        }),
        fx.Invoke(func(h *handler.ChatHandler, engine *gin.Engine) {
            h.RegisterRoutes(engine)
            log.Println("✓ Chat routes registered")
        }),
        fx.Invoke(func(h *admin.DashboardHandler, engine *gin.Engine) {
            h.RegisterRoutes(engine)
            log.Println("✓ Admin routes registered")
        }),
        
        // ==================== 生命周期 ====================
        fx.Invoke(RegisterLifecycle),
    )
    
    // 启动应用
    startCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    
    if err := app.Start(startCtx); err != nil {
        log.Fatal("failed to start application:", err)
    }
    
    log.Println("✅ Application started successfully")
    
    // 等待退出信号
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    sig := <-quit
    log.Printf("📡 Received signal: %v, shutting down...", sig)
    
    // 优雅关闭
    stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := app.Stop(stopCtx); err != nil {
        log.Fatal("failed to stop application:", err)
    }
    
    log.Println("👋 Application stopped gracefully")
}

// NewDatabase 创建数据库连接
func NewDatabase(lc fx.Lifecycle, cfg *config.Config) (*gorm.DB, error) {
    dsn := cfg.Database.DSN
    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    if err != nil {
        return nil, err
    }
    
    sqlDB, err := db.DB()
    if err != nil {
        return nil, err
    }
    
    // 配置连接池
    sqlDB.SetMaxIdleConns(10)
    sqlDB.SetMaxOpenConns(100)
    sqlDB.SetConnMaxLifetime(time.Hour)
    
    // 自动迁移
    if err := db.AutoMigrate(
        &model.User{},
        &model.Chat{},
        &model.ChatMessage{},
    ); err != nil {
        return nil, err
    }
    
    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            log.Println("📊 Database connected")
            return sqlDB.Ping()
        },
        OnStop: func(ctx context.Context) error {
            log.Println("📊 Database closing...")
            return sqlDB.Close()
        },
    })
    
    return db, nil
}

// NewGinEngine 创建 Gin 引擎
func NewGinEngine(cfg *config.Config) *gin.Engine {
    if cfg.App.Env == "production" {
        gin.SetMode(gin.ReleaseMode)
    }
    
    engine := gin.New()
    engine.Use(gin.Recovery())
    engine.Use(middleware.Logger())
    engine.Use(middleware.CORS())
    
    return engine
}

// NewHTTPServer 创建 HTTP 服务器
func NewHTTPServer(lc fx.Lifecycle, engine *gin.Engine, cfg *config.Config) *http.Server {
    srv := &http.Server{
        Addr:           cfg.Server.Address,
        Handler:        engine,
        ReadTimeout:    cfg.Server.ReadTimeout,
        WriteTimeout:   cfg.Server.WriteTimeout,
        MaxHeaderBytes: 1 << 20,
    }
    
    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            go func() {
                log.Printf("🚀 HTTP server started on %s", cfg.Server.Address)
                if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                    log.Fatal("failed to start HTTP server:", err)
                }
            }()
            return nil
        },
        OnStop: func(ctx context.Context) error {
            log.Println("🛑 HTTP server shutting down...")
            return srv.Shutdown(ctx)
        },
    })
    
    return srv
}

// RegisterLifecycle 注册全局生命周期
func RegisterLifecycle(lc fx.Lifecycle) {
    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            log.Println("🎯 Application lifecycle: OnStart")
            return nil
        },
        OnStop: func(ctx context.Context) error {
            log.Println("🎯 Application lifecycle: OnStop")
            return nil
        },
    })
}
```

---

# 3️⃣ 中间件示例

```go name=internal/middleware/auth.go
package middleware

import (
    "net/http"
    "strings"
    
    "github.com/gin-gonic/gin"
)

// Auth 用户认证中间件
func Auth() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        token = strings.TrimPrefix(token, "Bearer ")
        
        if token == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
            c.Abort()
            return
        }
        
        // 验证 token 并提取用户 ID
        userID := verifyToken(token)
        if userID == 0 {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
            c.Abort()
            return
        }
        
        c.Set("user_id", userID)
        c.Next()
    }
}

// AdminAuth 管理员认证中间件
func AdminAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        token = strings.TrimPrefix(token, "Bearer ")
        
        if token == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
            c.Abort()
            return
        }
        
        // 验证管理员 token
        if !isAdminToken(token) {
            c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
            c.Abort()
            return
        }
        
        c.Next()
    }
}

func verifyToken(token string) uint {
    // TODO: 实现 JWT 验证
    return 1
}

func isAdminToken(token string) bool {
    // TODO: 实现管理员验证
    return true
}
```

---

# 4️⃣ 配置管理

```go name=internal/config/config.go
package config

import (
    "time"
    
    "github.com/spf13/viper"
)

type Config struct {
    App      AppConfig      `mapstructure:"app"`
    Server   ServerConfig   `mapstructure:"server"`
    Database DatabaseConfig `mapstructure:"database"`
}

type AppConfig struct {
    Name string `mapstructure:"name"`
    Env  string `mapstructure:"env"`
}

type ServerConfig struct {
    Address      string        `mapstructure:"address"`
    ReadTimeout  time.Duration `mapstructure:"read_timeout"`
    WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
    DSN string `mapstructure:"dsn"`
}

func Load() (*Config, error) {
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath("./config")
    viper.AddConfigPath(".")
    
    if err := viper.ReadInConfig(); err != nil {
        return nil, err
    }
    
    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    
    return &cfg, nil
}
```

```yaml name=config/config.yaml
app:
  name: "MyApp"
  env: "development"

server:
  address: ":8080"
  read_timeout: 30s
  write_timeout: 30s

database:
  dsn: "user:password@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
```

---

# 5️⃣ 数据模型

```go name=internal/model/entities.go
package model

import (
    "time"
    
    "gorm.io/gorm"
)

type User struct {
    ID        uint           `gorm:"primarykey" json:"id"`
    Username  string         `gorm:"uniqueIndex;size:50" json:"username"`
    Email     string         `gorm:"uniqueIndex;size:100" json:"email"`
    Password  string         `gorm:"size:255" json:"-"`
    Nickname  string         `gorm:"size:50" json:"nickname"`
    Avatar    string         `gorm:"size:255" json:"avatar"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Chat struct {
    ID        uint           `gorm:"primarykey" json:"id"`
    UserID    uint           `gorm:"index" json:"user_id"`
    Title     string         `gorm:"size:255" json:"title"`
    Messages  []ChatMessage  `gorm:"foreignKey:ChatID" json:"messages,omitempty"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type ChatMessage struct {
    ID        uint      `gorm:"primarykey" json:"id"`
    ChatID    uint      `gorm:"index" json:"chat_id"`
    Content   string    `gorm:"type:text" json:"content"`
    Role      string    `gorm:"size:20" json:"role"` // user, assistant
    CreatedAt time.Time `json:"created_at"`
}
```

---

# 6️⃣ 与传统架构对比

| 特性 | 传统三层架构 | 简化架构 |
|------|------------|---------|
| **层级** | Controller → Service → Repository | Handler → Database |
| **代码量** | 多 | 少 |
| **抽象程度** | 高 | 适中 |
| **学习曲线** | 陡峭 | 平缓 |
| **适用场景** | 大型项目 | 中小型项目 |
| **开发速度** | 慢 | 快 |
| **测试难度** | 简单(易 Mock) | 中等 |
| **路由管理** | 集中式 | 分布式(Handler 自管理) |

---

# 7️⃣ 最佳实践

## ✅ DO - 推荐做法

1. **简单查询直接在 Handler 中完成**
```go
func (h *UserHandler) GetUser(c *gin.Context) {
    var user model.User
    h.db.First(&user, c.Param("id"))
    c.JSON(200, user)
}
```

2. **复杂业务逻辑提取到 Service**
```go
// Handler 调用 Service
func (h *PaymentHandler) CreateOrder(c *gin.Context) {
    order, err := h.paymentService.CreateAndNotify(...)
}
```

3. **每个 Handler 自管理路由**
```go
func (h *UserHandler) RegisterRoutes(r *gin.Engine) {
    api := r.Group("/api/user")
    api.POST("/login", h.Login)
}
```

---

## ❌ DON'T - 避免做法

1. **不要在 Handler 中写复杂算法**
```go
// ❌ 不推荐
func (h *Handler) Calculate(c *gin.Context) {
    // 100 行复杂计算...
}

// ✅ 推荐:提取到 Service
func (h *Handler) Calculate(c *gin.Context) {
    result := h.calculatorService.Compute(...)
}
```

2. **不要在 Service 中访问 gin.Context**
```go
// ❌ 不推荐
func (s *Service) Process(c *gin.Context) {
    c.JSON(...)
}

// ✅ 推荐
func (s *Service) Process(data Data) (Result, error) {
    return result, nil
}
```

---

# 8️⃣ 完整项目启动流程

```bash
# 1. 克隆项目
git clone your-project
cd your-project

# 2. 安装依赖
go mod download

# 3. 配置数据库
cp config/config.example.yaml config/config.yaml
vim config/config.yaml

# 4. 运行项目
go run cmd/server/main.go

# 5. 测试健康检查
curl http://localhost:8080/health
```

---

# 📋 技能检查清单

- [ ] Go 1.18+
- [ ] 已安装 `go.uber.org/fx`
- [ ] 已安装 `github.com/gin-gonic/gin`
- [ ] 已安装 `gorm.io/gorm`
- [ ] 理解 HTTP Handler 基本概念
- [ ] 了解 GORM 基本用法
- [ ] 熟悉 Gin 路由注册

---

# 🔗 参考资源

- **Uber FX**: https://uber-go.github.io/fx/
- **Gin 框架**: https://gin-gonic.com/
- **GORM**: https://gorm.io/
- **源码参考**: https://github.com/yangjian102621/geekai

---

**难度**: ⭐⭐⭐ (中级)  
**学习时间**: 1-2 天  
**适合人群**: 快速开发 MVP 的 Golang 开发者

---