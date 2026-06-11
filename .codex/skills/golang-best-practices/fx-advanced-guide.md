---
skill_name: "Golang Uber FX 依赖注入框架高级实战技能"
version: "2.0"
author: "基于 GeekAI 开源项目整理"
last_updated: "2026-02-06"
difficulty: "中级到高级"
tags: ["golang", "fx", "dependency-injection", "gin", "gorm", "microservices"]
source_repo: "https://github.com/yangjian102621/geekai"
---

# 🎯 技能目标

掌握 Uber FX 依赖注入框架在大型 Golang Web 项目中的高级应用,包括模块化设计、路由管理、服务生命周期控制等企业级开发技能。

## 适用场景

✅ 构建中大型 Web API 服务  
✅ 微服务架构项目  
✅ 需要清晰依赖关系的复杂系统  
✅ 团队协作的 Golang 项目  
✅ 从传统架构重构到依赖注入架构  

---

# 📚 目录

1. [核心概念速查](#核心概念速查)
2. [项目初始化模板](#项目初始化模板)
3. [分层架构实战](#分层架构实战)
4. [路由管理最佳实践](#路由管理最佳实践)
5. [服务生命周期管理](#服务生命周期管理)
6. [高级模式库](#高级模式库)
7. [性能优化技巧](#性能优化技巧)
8. [测试策略](#测试策略)
9. [故障排查指南](#故障排查指南)
10. [完整项目示例](#完整项目示例)

---

# 1️⃣ 核心概念速查

## FX 三大核心 API

| API | 用途 | 执行时机 | 示例 |
|-----|------|---------|------|
| `fx.Provide` | 注册构造函数到容器 | 应用启动前 | `fx.Provide(NewService)` |
| `fx.Invoke` | 触发依赖注入并执行函数 | 应用启动时 | `fx.Invoke(func(s *Service) { s.Init() })` |
| `fx.Module` | 组织相关的 Provide/Invoke | 编译时定义 | `fx.Module("user", ...)` |

## 依赖注入流程图

```
┌─────────────┐
│ fx.Provide  │ ──► 注册构造函数
└─────────────┘
       │
       ▼
┌─────────────┐
│ FX 容器     │ ──► 构建依赖图
└─────────────┘
       │
       ▼
┌─────────────┐
│ fx.Invoke   │ ──► 解析依赖 & 执行
└─────────────┘
```

---

# 2️⃣ 项目初始化模板

## 标准目录结构

```
project/
├── cmd/
│   └── server/
│       └── main.go          # 应用入口
├── internal/
│   ├── handler/             # HTTP 处理器层
│   │   ├── user.go
│   │   ├── chat.go
│   │   └── admin/           # 管理后台
│   │       └── dashboard.go
│   ├── service/             # 业务逻辑层
│   │   ├── user.go
│   │   ├── payment/
│   │   │   ├── alipay.go
│   │   │   └── wechat.go
│   │   └── ai/
│   │       ├── openai.go
│   │       └── claude.go
│   ├── repository/          # 数据访问层
│   │   ├── user_repo.go
│   │   └── order_repo.go
│   ├── model/               # 数据模型
│   │   └── entities.go
│   ├── config/              # 配置管理
│   │   └── config.go
│   └── pkg/                 # 内部工具包
│       ├── validator/
│       └── middleware/
├── pkg/                     # 可导出的公共包
│   └── utils/
├── config/
│   └── config.toml
├── go.mod
└── go.sum
```

---

## 极简启动模板

```go name=cmd/server/main.go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "go.uber.org/fx"
    "your-project/internal/config"
    "your-project/internal/handler"
    "your-project/internal/service"
    "your-project/internal/repository"
)

func main() {
    app := fx.New(
        // 1. 配置层
        fx.Provide(config.Load),
        
        // 2. 基础设施层
        fx.Provide(repository.NewDatabase),
        fx.Provide(repository.NewRedis),
        
        // 3. Repository 层
        fx.Provide(repository.NewUserRepository),
        
        // 4. Service 层
        fx.Provide(service.NewUserService),
        
        // 5. Handler 层
        fx.Provide(handler.NewUserHandler),
        
        // 6. HTTP 服务器
        fx.Provide(NewHTTPServer),
        
        // 7. 路由注册
        fx.Invoke(RegisterRoutes),
        
        // 8. 生命周期
        fx.Invoke(RegisterLifecycle),
    )
    
    // 启动应用
    startCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    
    if err := app.Start(startCtx); err != nil {
        log.Fatal(err)
    }
    
    // 等待退出信号
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    // 优雅关闭
    stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    
    if err := app.Stop(stopCtx); err != nil {
        log.Fatal(err)
    }
}
```

---

# 3️⃣ 分层架构实战

## Layer 1: Repository 层

```go name=internal/repository/user_repo.go
package repository

import (
    "context"
    "gorm.io/gorm"
    "your-project/internal/model"
)

type UserRepository interface {
    Create(ctx context.Context, user *model.User) error
    FindByID(ctx context.Context, id uint) (*model.User, error)
    Update(ctx context.Context, user *model.User) error
}

type userRepository struct {
    db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
    return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
    return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) FindByID(ctx context.Context, id uint) (*model.User, error) {
    var user model.User
    err := r.db.WithContext(ctx).First(&user, id).Error
    return &user, err
}

func (r *userRepository) Update(ctx context.Context, user *model.User) error {
    return r.db.WithContext(ctx).Save(user).Error
}
```

---

## Layer 2: Service 层

```go name=internal/service/user_service.go
package service

import (
    "context"
    "errors"
    "your-project/internal/model"
    "your-project/internal/repository"
)

type UserService interface {
    Register(ctx context.Context, username, password string) (*model.User, error)
    GetProfile(ctx context.Context, userID uint) (*model.User, error)
    UpdateProfile(ctx context.Context, user *model.User) error
}

type userService struct {
    userRepo repository.UserRepository
    // 可以注入其他服务
    emailService EmailService
}

func NewUserService(
    userRepo repository.UserRepository,
    emailService EmailService,
) UserService {
    return &userService{
        userRepo:     userRepo,
        emailService: emailService,
    }
}

func (s *userService) Register(ctx context.Context, username, password string) (*model.User, error) {
    // 业务逻辑验证
    if len(password) < 8 {
        return nil, errors.New("password too short")
    }
    
    user := &model.User{
        Username: username,
        Password: hashPassword(password),
    }
    
    if err := s.userRepo.Create(ctx, user); err != nil {
        return nil, err
    }
    
    // 发送欢迎邮件
    go s.emailService.SendWelcome(user.Email)
    
    return user, nil
}

func (s *userService) GetProfile(ctx context.Context, userID uint) (*model.User, error) {
    return s.userRepo.FindByID(ctx, userID)
}

func (s *userService) UpdateProfile(ctx context.Context, user *model.User) error {
    return s.userRepo.Update(ctx, user)
}
```

---

## Layer 3: Handler 层

```go name=internal/handler/user_handler.go
package handler

import (
    "net/http"
    "strconv"
    
    "github.com/gin-gonic/gin"
    "your-project/internal/service"
)

type UserHandler struct {
    userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
    return &UserHandler{
        userService: userService,
    }
}

// Register 用户注册
func (h *UserHandler) Register(c *gin.Context) {
    var req struct {
        Username string `json:"username" binding:"required"`
        Password string `json:"password" binding:"required,min=8"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    user, err := h.userService.Register(c.Request.Context(), req.Username, req.Password)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"data": user})
}

// GetProfile 获取用户信息
func (h *UserHandler) GetProfile(c *gin.Context) {
    userID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
    
    user, err := h.userService.GetProfile(c.Request.Context(), uint(userID))
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"data": user})
}
```

---

# 4️⃣ 路由管理最佳实践

## 模式一:集中式路由注册

```go name=internal/handler/routes.go
package handler

import (
    "github.com/gin-gonic/gin"
    "go.uber.org/fx"
)

type RouteParams struct {
    fx.In
    
    Engine *gin.Engine
    
    // Handlers
    UserHandler    *UserHandler
    ChatHandler    *ChatHandler
    PaymentHandler *PaymentHandler
}

func RegisterRoutes(p RouteParams) {
    // API v1 版本
    v1 := p.Engine.Group("/api/v1")
    {
        // 用户模块
        user := v1.Group("/user")
        {
            user.POST("/register", p.UserHandler.Register)
            user.POST("/login", p.UserHandler.Login)
            user.GET("/profile/:id", p.UserHandler.GetProfile)
            user.PUT("/profile", p.UserHandler.UpdateProfile)
        }
        
        // 聊天模块
        chat := v1.Group("/chat")
        {
            chat.GET("/list", p.ChatHandler.List)
            chat.POST("/send", p.ChatHandler.Send)
            chat.DELETE("/:id", p.ChatHandler.Delete)
        }
        
        // 支付模块
        payment := v1.Group("/payment")
        {
            payment.POST("/create", p.PaymentHandler.CreateOrder)
            payment.POST("/notify/alipay", p.PaymentHandler.AlipayNotify)
            payment.POST("/notify/wechat", p.PaymentHandler.WechatNotify)
        }
    }
    
    // Admin API
    admin := p.Engine.Group("/api/admin")
    {
        admin.GET("/dashboard", p.AdminHandler.Dashboard)
    }
}
```

---

## 模式二:分布式路由注册(推荐大型项目)

```go name=internal/handler/user/routes.go
package user

import (
    "github.com/gin-gonic/gin"
    "your-project/pkg/middleware"
)

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
    user := group.Group("/user")
    user.POST("/register", h.Register)
    user.POST("/login", h.Login)
    
    // 需要认证的路由
    auth := user.Group("", middleware.Auth())
    {
        auth.GET("/profile", h.GetProfile)
        auth.PUT("/profile", h.UpdateProfile)
        auth.POST("/avatar", h.UploadAvatar)
    }
}
```

```go name=cmd/server/main.go
fx.Invoke(func(
    engine *gin.Engine,
    userHandler *user.Handler,
    chatHandler *chat.Handler,
    paymentHandler *payment.Handler,
) {
    v1 := engine.Group("/api/v1")
    
    userHandler.RegisterRoutes(v1)
    chatHandler.RegisterRoutes(v1)
    paymentHandler.RegisterRoutes(v1)
})
```

---

## 模式三:基于 GeekAI 的内联路由注册

```go name=cmd/server/main.go
fx.Provide(handler.NewUserHandler),
fx.Invoke(func(s *gin.Engine, h *handler.UserHandler) {
    group := s.Group("/api/user")
    group.POST("/register", h.Register)
    group.POST("/login", h.Login)
    group.GET("/profile", h.GetProfile)
    group.POST("/profile/update", h.UpdateProfile)
    group.POST("/password", h.ChangePassword)
}),

fx.Provide(handler.NewChatHandler),
fx.Invoke(func(s *gin.Engine, h *handler.ChatHandler) {
    group := s.Group("/api/chat")
    group.GET("/list", h.List)
    group.GET("/detail", h.Detail)
    group.POST("/send", h.Send)
    group.DELETE("/:id", h.Delete)
}),
```

**优势**: 路由和 Handler 声明靠近,易于定位  
**劣势**: main.go 会变得很长,适合中小型项目

---

# 5️⃣ 服务生命周期管理

## 生命周期钩子完整示例

```go name=internal/service/background_service.go
package service

import (
    "context"
    "time"
    
    "go.uber.org/fx"
)

type BackgroundService struct {
    ticker *time.Ticker
    done   chan bool
}

func NewBackgroundService(lc fx.Lifecycle) *BackgroundService {
    s := &BackgroundService{
        ticker: time.NewTicker(1 * time.Minute),
        done:   make(chan bool),
    }
    
    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            go s.run()
            logger.Info("Background service started")
            return nil
        },
        OnStop: func(ctx context.Context) error {
            s.ticker.Stop()
            s.done <- true
            logger.Info("Background service stopped")
            return nil
        },
    })
    
    return s
}

func (s *BackgroundService) run() {
    for {
        select {
        case <-s.ticker.C:
            s.doWork()
        case <-s.done:
            return
        }
    }
}

func (s *BackgroundService) doWork() {
    // 执行定时任务
    logger.Debug("Running background task...")
}
```

---

## 数据库连接的生命周期管理

```go name=internal/repository/database.go
package repository

import (
    "context"
    "fmt"
    "time"
    
    "go.uber.org/fx"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
    "your-project/internal/config"
)

func NewDatabase(lc fx.Lifecycle, cfg *config.Config) (*gorm.DB, error) {
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True",
        cfg.DB.User,
        cfg.DB.Password,
        cfg.DB.Host,
        cfg.DB.Port,
        cfg.DB.Database,
    )
    
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
    
    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            return sqlDB.Ping()
        },
        OnStop: func(ctx context.Context) error {
            return sqlDB.Close()
        },
    })
    
    return db, nil
}
```

---

# 6️⃣ 高级模式库

## 模式 1: 条件服务注册

```go
type ServiceConfig struct {
    AlipayEnabled bool
    WechatEnabled bool
}

func NewPaymentServices(cfg *ServiceConfig) fx.Option {
    var options []fx.Option
    
    if cfg.AlipayEnabled {
        options = append(options, fx.Provide(NewAlipayService))
    }
    
    if cfg.WechatEnabled {
        options = append(options, fx.Provide(NewWechatService))
    }
    
    return fx.Options(options...)
}

// 使用
fx.New(
    fx.Provide(LoadConfig),
    fx.Invoke(func(cfg *ServiceConfig) fx.Option {
        return NewPaymentServices(cfg)
    }),
)
```

---

## 模式 2: 服务工厂模式

```go name=internal/service/ai_factory.go
package service

type AIProvider string

const (
    OpenAI  AIProvider = "openai"
    Claude  AIProvider = "claude"
    Gemini  AIProvider = "gemini"
)

type AIService interface {
    Chat(ctx context.Context, messages []Message) (string, error)
}

type AIServiceFactory struct {
    config *Config
}

func NewAIServiceFactory(config *Config) *AIServiceFactory {
    return &AIServiceFactory{config: config}
}

func (f *AIServiceFactory) Create(provider AIProvider) (AIService, error) {
    switch provider {
    case OpenAI:
        return NewOpenAIService(f.config.OpenAI)
    case Claude:
        return NewClaudeService(f.config.Claude)
    case Gemini:
        return NewGeminiService(f.config.Gemini)
    default:
        return nil, fmt.Errorf("unsupported AI provider: %s", provider)
    }
}
```

---

## 模式 3: 服务装饰器模式

```go name=internal/service/cache_decorator.go
package service

type CachedUserService struct {
    UserService
    cache Cache
}

func NewCachedUserService(base UserService, cache Cache) UserService {
    return &CachedUserService{
        UserService: base,
        cache:       cache,
    }
}

func (s *CachedUserService) GetProfile(ctx context.Context, userID uint) (*model.User, error) {
    // 尝试从缓存获取
    cacheKey := fmt.Sprintf("user:%d", userID)
    if cached, ok := s.cache.Get(cacheKey); ok {
        return cached.(*model.User), nil
    }
    
    // 缓存未命中,调用原始服务
    user, err := s.UserService.GetProfile(ctx, userID)
    if err != nil {
        return nil, err
    }
    
    // 存入缓存
    s.cache.Set(cacheKey, user, 5*time.Minute)
    return user, nil
}
```

```go
// 在 FX 中使用装饰器
fx.New(
    fx.Provide(repository.NewUserRepository),
    fx.Provide(service.NewUserService),
    fx.Provide(NewCache),
    fx.Decorate(service.NewCachedUserService), // 装饰器
)
```

---

## 模式 4: 批量 Handler 注册

```go name=internal/handler/registry.go
package handler

type HandlerRegistry struct {
    fx.In
    
    UserHandler     *UserHandler
    ChatHandler     *ChatHandler
    PaymentHandler  *PaymentHandler
    AdminHandler    *AdminHandler
    // ... 更多 Handler
}

func RegisterAllRoutes(engine *gin.Engine, registry HandlerRegistry) {
    api := engine.Group("/api/v1")
    
    // 用户路由
    registerUserRoutes(api, registry.UserHandler)
    
    // 聊天路由
    registerChatRoutes(api, registry.ChatHandler)
    
    // 支付路由
    registerPaymentRoutes(api, registry.PaymentHandler)
    
    // 管理路由
    admin := engine.Group("/api/admin")
    registerAdminRoutes(admin, registry.AdminHandler)
}

func registerUserRoutes(group *gin.RouterGroup, h *UserHandler) {
    user := group.Group("/user")
    user.POST("/register", h.Register)
    user.POST("/login", h.Login)
    // ...
}
```

---

# 7️⃣ 性能优化技巧

## 优化 1: 延迟初始化

```go
type LazyService struct {
    initOnce sync.Once
    client   *http.Client
    config   *Config
}

func NewLazyService(config *Config) *LazyService {
    return &LazyService{config: config}
}

func (s *LazyService) getClient() *http.Client {
    s.initOnce.Do(func() {
        s.client = &http.Client{
            Timeout: s.config.Timeout,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
            },
        }
    })
    return s.client
}
```

---

## 优化 2: 并发安全的服务

```go
type ConcurrentCacheService struct {
    mu    sync.RWMutex
    cache map[string]interface{}
}

func (s *ConcurrentCacheService) Get(key string) (interface{}, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    val, ok := s.cache[key]
    return val, ok
}

func (s *ConcurrentCacheService) Set(key string, value interface{}) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.cache[key] = value
}
```

---

## 优化 3: 避免循环依赖

```go
// ❌ 错误示例 - 循环依赖
type UserService struct {
    orderService *OrderService
}

type OrderService struct {
    userService *UserService  // 循环依赖!
}

// ✅ 正确示例 - 使用接口打破循环
type UserService struct {
    orderRepo OrderRepository
}

type OrderService struct {
    userRepo UserRepository
}
```

---

# 8️⃣ 测试策略

## 单元测试 - 直接调用构造函数

```go name=internal/service/user_service_test.go
package service_test

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "your-project/internal/service"
)

type MockUserRepository struct {
    mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *model.User) error {
    args := m.Called(ctx, user)
    return args.Error(0)
}

func TestUserService_Register(t *testing.T) {
    // Arrange
    mockRepo := new(MockUserRepository)
    mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
    
    svc := service.NewUserService(mockRepo, nil)
    
    // Act
    user, err := svc.Register(context.Background(), "test", "password123")
    
    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, user)
    mockRepo.AssertExpectations(t)
}
```

---

## 集成测试 - 使用 FX 测试容器

```go name=internal/handler/user_handler_test.go
package handler_test

import (
    "testing"
    
    "go.uber.org/fx"
    "go.uber.org/fx/fxtest"
    "your-project/internal/handler"
    "your-project/internal/service"
)

func TestUserHandler_Integration(t *testing.T) {
    var h *handler.UserHandler
    
    app := fxtest.New(t,
        fx.Provide(setupTestDB),
        fx.Provide(repository.NewUserRepository),
        fx.Provide(service.NewUserService),
        fx.Provide(handler.NewUserHandler),
        fx.Populate(&h),
    )
    defer app.RequireStart().RequireStop()
    
    // 使用 h 进行测试
    assert.NotNil(t, h)
}
```

---

# 9️⃣ 故障排查指南

## 常见错误 1: 依赖未注册

```
Error: missing type: *service.UserService
```

**解决方案**:
```go
// 确保已经 Provide
fx.Provide(service.NewUserService)
```

---

## 常见错误 2: 循环依赖

```
Error: cycle detected in dependency graph
```

**解决方案**:
1. 使用接口替代具体类型
2. 重新设计依赖关系
3. 使用事件驱动架构

---

## 常见错误 3: 参数类型不匹配

```
Error: cannot provide function: argument type mismatch
```

**解决方案**:
```go
// ❌ 错误
func NewService(db gorm.DB) *Service  // 缺少指针

// ✅ 正确
func NewService(db *gorm.DB) *Service
```

---

# 🔟 完整项目示例

## 完整 main.go

```go name=cmd/server/main.go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/gin-gonic/gin"
    "go.uber.org/fx"
    
    "your-project/internal/config"
    "your-project/internal/handler"
    "your-project/internal/handler/admin"
    "your-project/internal/middleware"
    "your-project/internal/repository"
    "your-project/internal/service"
    "your-project/internal/service/payment"
    "your-project/internal/service/ai"
)

func main() {
    app := fx.New(
        // ==================== 配置层 ====================
        fx.Provide(config.Load),
        
        // ==================== 基础设施层 ====================
        fx.Provide(repository.NewDatabase),
        fx.Provide(repository.NewRedis),
        fx.Provide(repository.NewLevelDB),
        
        // ==================== Repository 层 ====================
        fx.Provide(repository.NewUserRepository),
        fx.Provide(repository.NewOrderRepository),
        fx.Provide(repository.NewChatRepository),
        
        // ==================== Service 层 ====================
        // 核心服务
        fx.Provide(service.NewUserService),
        fx.Provide(service.NewChatService),
        fx.Provide(service.NewOrderService),
        
        // 支付服务
        fx.Provide(payment.NewAlipayService),
        fx.Provide(payment.NewWechatService),
        fx.Provide(payment.NewPaymentManager),
        
        // AI 服务
        fx.Provide(ai.NewOpenAIService),
        fx.Provide(ai.NewClaudeService),
        fx.Provide(ai.NewAIManager),
        
        // 工具服务
        fx.Provide(service.NewEmailService),
        fx.Provide(service.NewSMSService),
        fx.Provide(service.NewCacheService),
        
        // ==================== Handler 层 ====================
        // 前台 Handler
        fx.Provide(handler.NewUserHandler),
        fx.Provide(handler.NewChatHandler),
        fx.Provide(handler.NewPaymentHandler),
        
        // 后台 Handler
        fx.Provide(admin.NewDashboardHandler),
        fx.Provide(admin.NewUserManageHandler),
        
        // ==================== HTTP 服务器 ====================
        fx.Provide(NewGinEngine),
        fx.Provide(NewHTTPServer),
        
        // ==================== 路由注册 ====================
        fx.Invoke(RegisterFrontendRoutes),
        fx.Invoke(RegisterAdminRoutes),
        
        // ==================== 后台任务 ====================
        fx.Provide(service.NewScheduler),
        fx.Invoke(func(scheduler *service.Scheduler) {
            scheduler.Start()
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
    
    log.Println("Application started successfully")
    
    // 等待退出信号
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    sig := <-quit
    log.Printf("Received signal: %v, shutting down...", sig)
    
    // 优雅关闭
    stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := app.Stop(stopCtx); err != nil {
        log.Fatal("failed to stop application:", err)
    }
    
    log.Println("Application stopped gracefully")
}

// NewGinEngine 创建 Gin 引擎
func NewGinEngine() *gin.Engine {
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
                if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                    log.Fatal("failed to start HTTP server:", err)
                }
            }()
            log.Printf("HTTP server started on %s", cfg.Server.Address)
            return nil
        },
        OnStop: func(ctx context.Context) error {
            return srv.Shutdown(ctx)
        },
    })
    
    return srv
}

// RegisterFrontendRoutes 注册前台路由
func RegisterFrontendRoutes(
    engine *gin.Engine,
    userHandler *handler.UserHandler,
    chatHandler *handler.ChatHandler,
    paymentHandler *handler.PaymentHandler,
) {
    api := engine.Group("/api/v1")
    
    // 用户路由
    user := api.Group("/user")
    {
        user.POST("/register", userHandler.Register)
        user.POST("/login", userHandler.Login)
        
        auth := user.Group("", middleware.Auth())
        {
            auth.GET("/profile", userHandler.GetProfile)
            auth.PUT("/profile", userHandler.UpdateProfile)
        }
    }
    
    // 聊天路由
    chat := api.Group("/chat", middleware.Auth())
    {
        chat.GET("/list", chatHandler.List)
        chat.POST("/send", chatHandler.Send)
        chat.DELETE("/:id", chatHandler.Delete)
    }
    
    // 支付路由
    payment := api.Group("/payment")
    {
        payment.POST("/create", paymentHandler.CreateOrder)
        payment.POST("/notify/alipay", paymentHandler.AlipayNotify)
        payment.POST("/notify/wechat", paymentHandler.WechatNotify)
    }
}

// RegisterAdminRoutes 注册后台路由
func RegisterAdminRoutes(
    engine *gin.Engine,
    dashboardHandler *admin.DashboardHandler,
    userManageHandler *admin.UserManageHandler,
) {
    admin := engine.Group("/api/admin", middleware.AdminAuth())
    {
        admin.GET("/dashboard", dashboardHandler.Stats)
        admin.GET("/users", userManageHandler.List)
        admin.POST("/users", userManageHandler.Create)
    }
}

// RegisterLifecycle 注册全局生命周期
func RegisterLifecycle(lc fx.Lifecycle) {
    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            log.Println("🚀 Application lifecycle: OnStart")
            return nil
        },
        OnStop: func(ctx context.Context) error {
            log.Println("🛑 Application lifecycle: OnStop")
            return nil
        },
    })
}
```

---

# 📋 最佳实践检查清单

使用本技能前请确认:

- [ ] Go 版本 >= 1.18
- [ ] 已引入 `go.uber.org/fx`
- [ ] 已引入 `github.com/gin-gonic/gin`
- [ ] 已引入 `gorm.io/gorm`
- [ ] 理解依赖注入的基本概念
- [ ] 理解接口和依赖倒置原则
- [ ] 熟悉 Golang 的 Context 用法

---

# 🔗 参考资源

- **Uber FX 官方文档**: https://uber-go.github.io/fx/
- **源码项目**: https://github.com/yangjian102621/geekai
- **Gin 框架**: https://gin-gonic.com/
- **GORM 文档**: https://gorm.io/

---

# 💡 下一步学习

1. 学习 FX 的 `fx.Module` 功能进行模块化设计
2. 研究 `fx.Decorate` 实现装饰器模式
3. 探索 `fx.Replace` 替换已注册的服务
4. 学习使用 `fx.Annotate` 为依赖添加标签
5. 了解 `fxevent` 包进行事件监听

---

**技能等级**: 🌟🌟🌟🌟 (中高级)  
**预计学习时间**: 3-5 天  
**适合人群**: 有 Golang 基础,需要构建中大型 Web 应用的开发者

---

**版权声明**: 本技能文档基于 Apache 2.0 协议的开源项目 GeekAI 整理,遵循相同协议。