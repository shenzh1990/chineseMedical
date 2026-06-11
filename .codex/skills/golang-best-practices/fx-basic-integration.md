# Golang FX 依赖注入框架集成指南

## 技能概述

本技能文档基于开源项目 [yangjian102621/geekai](https://github.com/yangjian102621/geekai),提供 Golang 项目中使用 Uber FX 依赖注入框架的完整实践指南。

**适用场景**:
- 构建 Gin Web 应用
- 需要依赖注入的微服务项目
- 重构现有 Golang 项目到 FX ��构

**技术栈**: Gin + MySQL + FX + GORM

---

## 一、FX 核心概念

### 1.1 fx.Provide - 注册依赖

用于向 FX 容器注册服务构造函数。

**语法**:
```go
fx.Provide(构造函数)
```

**示例**:
```go
// 简单服务注册
fx.Provide(store.NewRedisClient)

// 带配置参数的服务
fx.Provide(func(config *types.AppConfig) *service.CaptchaService {
    return service.NewCaptchaService(config.ApiConfig)
})

// 返回错误的构造函数
fx.Provide(func() (*xdb.Searcher, error) {
    file, err := xdbFS.Open("res/ip2region.xdb")
    if err != nil {
        return nil, err // FX 会自动处理错误
    }
    return xdb.NewWithBuffer(buffer)
})
```

---

### 1.2 fx.Invoke - 触发依赖注入

用于执行需要依赖注入的初始化代码。

**语法**:
```go
fx.Invoke(func(依赖1, 依赖2) {
    // 初始化逻辑
})
```

**示例**:
```go
// 应用初始化
fx.Invoke(func(s *core.AppServer, client *redis.Client) {
    s.Init(debug, client)
})

// 启动后台任务
fx.Invoke(func(s *mj.Service) {
    s.Run()
    s.SyncTaskProgress()
    s.DownloadImages()
})

// 条件启动
fx.Invoke(func(exec *service.XXLJobExecutor, config *types.AppConfig) {
    if config.XXLConfig.Enabled {
        go func() {
            log.Fatal(exec.Run())
        }()
    }
})
```

---

## 二、项目结构规范

### 2.1 推荐目录结构

```
api/
├── main.go              # FX 启动入口
├── handler/             # HTTP 控制器层
│   ├── base_handler.go  # 基础 Handler
│   ├── user_handler.go
│   ├── chat_handler.go
│   └── admin/           # 后台管理 Handler
│       ├── config_handler.go
│       └── dashboard_handler.go
├── service/             # 业务逻辑层
│   ├── user_service.go
│   ├── captcha_service.go
│   ├── mj/              # MidJourney 服务
│   │   └── service.go
│   ├── payment/         # 支付服务
│   │   ├── alipay_service.go
│   │   └── wepay_service.go
│   └── sms/             # 短信服务
│       └── service_manager.go
├── store/               # 数据访问层
│   ├── mysql.go
│   ├── redis.go
│   └── model/           # 数据模型
└── core/                # 核心模块
    ├── server.go
    └── config.go
```

---

## 三、实战模式

### 3.1 应用启动模板

```go name=main.go
package main

import (
    "go.uber.org/fx"
    "gorm.io/gorm"
)

func main() {
    app := fx.New(
        // 1. 配置层
        fx.Provide(LoadConfig),
        
        // 2. 基础设施层
        fx.Provide(store.NewMysql),
        fx.Provide(store.NewRedisClient),
        
        // 3. 服务层
        fx.Provide(service.NewUserService),
        
        // 4. 控制器层
        fx.Provide(handler.NewUserHandler),
        
        // 5. 路由注册
        fx.Invoke(RegisterRoutes),
        
        // 6. 生命周期管理
        fx.Invoke(RegisterLifecycle),
    )
    
    // 启动应用
    go func() {
        if err := app.Start(context.Background()); err != nil {
            log.Fatal(err)
        }
    }()
    
    // 优雅退出
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    app.Stop(ctx)
}

func LoadConfig() (*types.AppConfig, error) {
    config, err := core.LoadConfig("config.toml")
    return config, err
}
```

---

### 3.2 Handler 层模式

#### BaseHandler 设计

```go name=handler/base_handler.go
package handler

type BaseHandler struct {
    App *core.AppServer
    DB  *gorm.DB
}

func (h *BaseHandler) GetLoginUserId(c *gin.Context) uint {
    userId, ok := c.Get(types.LoginUserID)
    if !ok {
        return 0
    }
    return uint(utils.IntValue(utils.InterfaceToString(userId), 0))
}

func (h *BaseHandler) GetLoginUser(c *gin.Context) (model.User, error) {
    userId, ok := c.Get(types.LoginUserID)
    if !ok {
        return model.User{}, errors.New("user not login")
    }
    
    var user model.User
    res := h.DB.Where("id", userId).First(&user)
    return user, res.Error
}
```

#### 业务 Handler 实现

```go name=handler/user_handler.go
package handler

type UserHandler struct {
    BaseHandler
    userService *service.UserService
}

// 构造函数 - FX 自动注入依赖
func NewUserHandler(
    app *core.AppServer,
    db *gorm.DB,
    userService *service.UserService,
) *UserHandler {
    return &UserHandler{
        BaseHandler: BaseHandler{App: app, DB: db},
        userService: userService,
    }
}

func (h *UserHandler) Register(c *gin.Context) {
    // 业务逻辑
}
```

#### 在 main.go 中注册

```go
fx.Provide(handler.NewUserHandler),
fx.Invoke(func(s *core.AppServer, h *handler.UserHandler) {
    group := s.Engine.Group("/api/user/")
    group.POST("register", h.Register)
    group.POST("login", h.Login)
    group.GET("profile", h.Profile)
})
```

---

### 3.3 Service 层模式

#### 简单服务

```go name=service/user_service.go
package service

type UserService struct {
    db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
    return &UserService{db: db}
}

func (s *UserService) DecreasePower(userId uint, power int) error {
    return s.db.Model(&model.User{}).
        Where("id = ?", userId).
        UpdateColumn("power", gorm.Expr("power - ?", power)).Error
}
```

#### 复杂服务 - 带配置和多依赖

```go name=service/mj/service.go
package mj

type Service struct {
    client          *Client
    taskQueue       *store.RedisQueue
    db              *gorm.DB
    uploaderManager *oss.UploaderManager
    userService     *service.UserService
}

func NewService(
    redisCli *redis.Client,
    db *gorm.DB,
    client *Client,
    manager *oss.UploaderManager,
    userService *service.UserService,
) *Service {
    return &Service{
        db:              db,
        taskQueue:       store.NewRedisQueue("MidJourney_Task_Queue", redisCli),
        client:          client,
        uploaderManager: manager,
        userService:     userService,
    }
}

func (s *Service) Run() {
    // 后台任务逻辑
}
```

#### 服务注册和启动

```go
// 注册服务
fx.Provide(mj.NewService),
fx.Provide(mj.NewClient),

// 启动后台任务
fx.Invoke(func(s *mj.Service) {
    s.Run()
    s.SyncTaskProgress()
    s.DownloadImages()
})
```

---

### 3.4 服务管理器模式

适用于多种实现的服务(如短信、支付)。

```go name=service/sms/service_manager.go
package sms

type ServiceManager struct {
    handler Service
}

func NewSendServiceManager(config *types.AppConfig) (*ServiceManager, error) {
    active := strings.ToUpper(config.SMS.Active)
    var handler Service
    
    switch active {
    case "ALI":
        client, err := NewAliYunSmsService(config)
        if err != nil {
            return nil, err
        }
        handler = client
    case "BAO":
        handler = NewSmsBaoSmsService(config)
    default:
        return nil, errors.New("unknown SMS provider")
    }
    
    return &ServiceManager{handler: handler}, nil
}

func (m *ServiceManager) GetService() Service {
    return m.handler
}
```

---

### 3.5 数据库迁移模式

```go name=service/migration_service.go
package service

type MigrationService struct {
    db *gorm.DB
}

func NewMigrationService(db *gorm.DB) *MigrationService {
    return &MigrationService{db: db}
}

func (s *MigrationService) Migrate() error {
    return s.db.AutoMigrate(
        &model.User{},
        &model.ChatItem{},
        &model.Order{},
        // ...更多模型
    )
}
```

#### 在 main.go 中调用

```go
fx.Provide(service.NewMigrationService),
fx.Invoke(func(s *service.MigrationService) {
    s.Migrate()
})
```

---

## 四、高级技巧

### 4.1 嵌入式资源注入

```go
//go:embed res
var xdbFS embed.FS

fx.Provide(func() embed.FS {
    return xdbFS
})

fx.Provide(func(fs embed.FS) (*xdb.Searcher, error) {
    file, err := fs.Open("res/ip2region.xdb")
    if err != nil {
        return nil, err
    }
    buffer, _ := io.ReadAll(file)
    return xdb.NewWithBuffer(buffer)
})
```

---

### 4.2 配置适配器模式

当服务需要的配置字段与全局配置不一致时:

```go
// 全局配置包含多个子配置
type AppConfig struct {
    ApiConfig     types.ApiConfig
    AlipayConfig  types.AlipayConfig
    WechatConfig  types.WechatPayConfig
}

// 服务只需要部分配置
fx.Provide(func(config *types.AppConfig) *service.CaptchaService {
    return service.NewCaptchaService(config.ApiConfig)
})
```

---

### 4.3 生命周期钩子

```go
type AppLifecycle struct{}

func (l *AppLifecycle) OnStart(ctx context.Context) error {
    logger.Info("Application starting...")
    return nil
}

func (l *AppLifecycle) OnStop(ctx context.Context) error {
    logger.Info("Application stopping...")
    return nil
}

func NewAppLifeCycle() *AppLifecycle {
    return &AppLifecycle{}
}

// 注册生命周期
fx.Provide(NewAppLifeCycle),
fx.Invoke(func(lc fx.Lifecycle, app *AppLifecycle) {
    lc.Append(fx.Hook{
        OnStart: app.OnStart,
        OnStop:  app.OnStop,
    })
})
```

---

## 五、完整示例

### 5.1 支付服务集成

```go name=service/payment/alipay_service.go
package payment

type AlipayService struct {
    config *types.AlipayConfig
    client *alipay.Client
}

func NewAlipayService(appConfig *types.AppConfig) (*AlipayService, error) {
    config := appConfig.AlipayConfig
    if !config.Enabled {
        logger.Info("Disabled Alipay service")
        return nil, nil
    }
    
    client, err := alipay.NewClient(config.AppId, priKey, !config.SandBox)
    if err != nil {
        return nil, fmt.Errorf("error with initialize alipay: %v", err)
    }
    
    return &AlipayService{config: &config, client: client}, nil
}

func (s *AlipayService) PayPC(params AlipayParams) (string, error) {
    bm := make(gopay.BodyMap)
    bm.Set("subject", params.Subject)
    bm.Set("out_trade_no", params.OutTradeNo)
    bm.Set("total_amount", params.TotalFee)
    return s.client.TradePagePay(context.Background(), bm)
}
```

#### 注册和使用

```go
// main.go
fx.Provide(payment.NewAlipayService),
fx.Provide(handler.NewPaymentHandler),

fx.Invoke(func(s *core.AppServer, h *handler.PaymentHandler) {
    group := s.Engine.Group("/api/payment/")
    group.POST("doPay", h.Pay)
    group.POST("notify/alipay", h.AlipayNotify)
})
```

---

## 六、最佳实践

### ✅ DO - 推荐做法

1. **明确依赖声明**
```go
func NewUserService(
    db *gorm.DB,
    redis *redis.Client,
    config *types.AppConfig,
) *UserService
```

2. **错误处理**
```go
fx.Provide(func() (*sql.DB, error) {
    db, err := sql.Open("mysql", dsn)
    if err != nil {
        return nil, err // FX 会自动中止启动
    }
    return db, nil
})
```

3. **条件服务注册**
```go
fx.Provide(func(config *types.AppConfig) (*AlipayService, error) {
    if !config.AlipayConfig.Enabled {
        return nil, nil // 返回 nil 表示服务不可用
    }
    return NewAlipayService(config)
})
```

---

### ❌ DON'T - 避免做法

1. **避免全局变量**
```go
// ❌ 不推荐
var globalDB *gorm.DB

func init() {
    globalDB = connectDB()
}
```

2. **避免隐式依赖**
```go
// ❌ 不推荐
func NewService() *Service {
    db := getGlobalDB()
    return &Service{db: db}
}

// ✅ 推荐
func NewService(db *gorm.DB) *Service {
    return &Service{db: db}
}
```

---

## 七、常见问题

### Q1: 如何注入多个相同类型的服务?

**方法 1: 使用类型别名**
```go
type MySQLDB *gorm.DB
type PostgresDB *gorm.DB

fx.Provide(func() (MySQLDB, error) {
    return gorm.Open(mysql.Open(dsn))
})

fx.Provide(func() (PostgresDB, error) {
    return gorm.Open(postgres.Open(dsn))
})
```

**方法 2: 使用结构体包装**
```go
type DatabaseClients struct {
    MySQL    *gorm.DB
    Postgres *gorm.DB
}

fx.Provide(func() (*DatabaseClients, error) {
    mysql, _ := gorm.Open(mysql.Open(dsn1))
    postgres, _ := gorm.Open(postgres.Open(dsn2))
    return &DatabaseClients{
        MySQL:    mysql,
        Postgres: postgres,
    }, nil
})
```

---

### Q2: fx.Invoke 执行顺序如何控制?

FX 会按照依赖关系自动排序,但 Invoke 的声明顺序也会影响执行:

```go
fx.New(
    fx.Provide(NewDB),
    fx.Provide(NewService),
    
    // 先执行数据库迁移
    fx.Invoke(func(db *gorm.DB) {
        db.AutoMigrate(&User{})
    }),
    
    // 再启动服务
    fx.Invoke(func(svc *Service) {
        svc.Run()
    }),
)
```

---

### Q3: 如何测试使用 FX 的代码?

**方法 1: 直接调用构造函数**
```go
func TestUserService(t *testing.T) {
    db := setupTestDB()
    svc := service.NewUserService(db)
    
    // 测试逻辑
}
```

**方法 2: 使用 FX 测试容器**
```go
func TestWithFX(t *testing.T) {
    var svc *service.UserService
    
    app := fxtest.New(t,
        fx.Provide(setupTestDB),
        fx.Provide(service.NewUserService),
        fx.Populate(&svc),
    )
    defer app.RequireStart().RequireStop()
    
    // 使用 svc 进行测试
}
```

---

## 八、性能优化

### 8.1 延迟初始化

```go
type LazyService struct {
    initOnce sync.Once
    client   *http.Client
}

func (s *LazyService) GetClient() *http.Client {
    s.initOnce.Do(func() {
        s.client = &http.Client{Timeout: 10 * time.Second}
    })
    return s.client
}
```

---

### 8.2 并发安全的服务

```go
type ConcurrentService struct {
    mu    sync.RWMutex
    cache map[string]interface{}
}

func (s *ConcurrentService) Get(key string) interface{} {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.cache[key]
}
```

---

## 九、参考资源

- **项目源码**: https://github.com/yangjian102621/geekai
- **Uber FX 官方文档**: https://uber-go.github.io/fx/
- **Gin 框架文档**: https://gin-gonic.com/docs/
- **GORM 文档**: https://gorm.io/docs/

---

## 十、技能检查清单

使用本技能前请确保:

- [ ] 已安装 Go 1.18+
- [ ] 理解依赖注入的基本概念
- [ ] 熟悉 Gin 框架基础用法
- [ ] 了解 GORM 基本操作
- [ ] 项目已引入 `go.uber.org/fx`

---

## 附录: 完整项目结构示例

查看完整代码结构请访问:
- [Service 层示例](https://github.com/yangjian102621/geekai/tree/main/api/service)
- [Handler 层示例](https://github.com/yangjian102621/geekai/tree/main/api/handler)
- [Main 入口示例](https://github.com/yangjian102621/geekai/blob/main/api/main.go)

---

**文档版本**: v1.0  
**最后更新**: 2026-02-06  
**维护者**: 基于 GeekAI 项目整理