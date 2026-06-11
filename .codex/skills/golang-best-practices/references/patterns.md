# Go Design Patterns and Idioms

## Creational Patterns

### Builder Pattern
```go
type ServerConfig struct {
    Host     string
    Port     int
    Timeout  time.Duration
    MaxConns int
}

type ServerBuilder struct {
    config ServerConfig
}

func NewServerBuilder() *ServerBuilder {
    return &ServerBuilder{
        config: ServerConfig{
            Host:     "localhost",
            Port:     8080,
            Timeout:  30 * time.Second,
            MaxConns: 100,
        },
    }
}

func (b *ServerBuilder) WithHost(host string) *ServerBuilder {
    b.config.Host = host
    return b
}

func (b *ServerBuilder) WithPort(port int) *ServerBuilder {
    b.config.Port = port
    return b
}

func (b *ServerBuilder) Build() *Server {
    return &Server{config: b.config}
}

// Usage
server := NewServerBuilder().
    WithHost("0.0.0.0").
    WithPort(9090).
    Build()
```

### Functional Options Pattern
```go
type Server struct {
    host     string
    port     int
    timeout  time.Duration
}

type Option func(*Server)

func WithHost(host string) Option {
    return func(s *Server) {
        s.host = host
    }
}

func WithPort(port int) Option {
    return func(s *Server) {
        s.port = port
    }
}

func WithTimeout(timeout time.Duration) Option {
    return func(s *Server) {
        s.timeout = timeout
    }
}

func NewServer(opts ...Option) *Server {
    s := &Server{
        host:    "localhost",
        port:    8080,
        timeout: 30 * time.Second,
    }
    
    for _, opt := range opts {
        opt(s)
    }
    
    return s
}

// Usage
server := NewServer(
    WithHost("0.0.0.0"),
    WithPort(9090),
    WithTimeout(60 * time.Second),
)
```

### Singleton Pattern
```go
type Database struct {
    // fields
}

var (
    instance *Database
    once     sync.Once
)

func GetInstance() *Database {
    once.Do(func() {
        instance = &Database{}
        // Initialize database
    })
    return instance
}
```

## Structural Patterns

### Adapter Pattern
```go
// Legacy interface
type LegacyPrinter interface {
    Print(s string)
}

// New interface
type Printer interface {
    PrintString(s string) error
}

// Adapter
type PrinterAdapter struct {
    legacy LegacyPrinter
}

func (p *PrinterAdapter) PrintString(s string) error {
    p.legacy.Print(s)
    return nil
}
```

### Decorator Pattern
```go
type Handler interface {
    Handle(req Request) Response
}

type LoggingHandler struct {
    next Handler
}

func (h *LoggingHandler) Handle(req Request) Response {
    log.Printf("Request: %+v", req)
    resp := h.next.Handle(req)
    log.Printf("Response: %+v", resp)
    return resp
}

type CachingHandler struct {
    next  Handler
    cache map[string]Response
}

func (h *CachingHandler) Handle(req Request) Response {
    if resp, ok := h.cache[req.Key]; ok {
        return resp
    }
    resp := h.next.Handle(req)
    h.cache[req.Key] = resp
    return resp
}

// Usage
handler := &LoggingHandler{
    next: &CachingHandler{
        next:  &BaseHandler{},
        cache: make(map[string]Response),
    },
}
```

## Behavioral Patterns

### Strategy Pattern
```go
type Strategy interface {
    Execute(data []int) int
}

type SumStrategy struct{}
func (s *SumStrategy) Execute(data []int) int {
    sum := 0
    for _, v := range data {
        sum += v
    }
    return sum
}

type MaxStrategy struct{}
func (s *MaxStrategy) Execute(data []int) int {
    max := data[0]
    for _, v := range data[1:] {
        if v > max {
            max = v
        }
    }
    return max
}

type Calculator struct {
    strategy Strategy
}

func (c *Calculator) SetStrategy(s Strategy) {
    c.strategy = s
}

func (c *Calculator) Calculate(data []int) int {
    return c.strategy.Execute(data)
}
```

### Observer Pattern
```go
type Observer interface {
    Update(data string)
}

type Subject struct {
    observers []Observer
}

func (s *Subject) Attach(o Observer) {
    s.observers = append(s.observers, o)
}

func (s *Subject) Notify(data string) {
    for _, observer := range s.observers {
        observer.Update(data)
    }
}

type ConcreteObserver struct {
    id string
}

func (o *ConcreteObserver) Update(data string) {
    fmt.Printf("Observer %s received: %s\n", o.id, data)
}
```

## Concurrency Patterns

### Worker Pool
```go
type Job struct {
    ID   int
    Data string
}

type Result struct {
    Job    Job
    Output string
}

func worker(id int, jobs <-chan Job, results chan<- Result) {
    for job := range jobs {
        output := process(job.Data)
        results <- Result{Job: job, Output: output}
    }
}

func WorkerPool(numWorkers int, jobs []Job) []Result {
    jobChan := make(chan Job, len(jobs))
    resultChan := make(chan Result, len(jobs))
    
    // Start workers
    for i := 0; i < numWorkers; i++ {
        go worker(i, jobChan, resultChan)
    }
    
    // Send jobs
    for _, job := range jobs {
        jobChan <- job
    }
    close(jobChan)
    
    // Collect results
    results := make([]Result, 0, len(jobs))
    for i := 0; i < len(jobs); i++ {
        results = append(results, <-resultChan)
    }
    
    return results
}
```

### Pipeline
```go
func generator(nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        for _, n := range nums {
            out <- n
        }
        close(out)
    }()
    return out
}

func square(in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        for n := range in {
            out <- n * n
        }
        close(out)
    }()
    return out
}

func filter(in <-chan int, f func(int) bool) <-chan int {
    out := make(chan int)
    go func() {
        for n := range in {
            if f(n) {
                out <- n
            }
        }
        close(out)
    }()
    return out
}

// Usage
c := generator(1, 2, 3, 4, 5)
c = square(c)
c = filter(c, func(n int) bool { return n > 10 })

for n := range c {
    fmt.Println(n) // 16, 25
}
```

### Rate Limiting
```go
type RateLimiter struct {
    rate     int
    interval time.Duration
    ticker   *time.Ticker
    tokens   chan struct{}
}

func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
    rl := &RateLimiter{
        rate:     rate,
        interval: interval,
        ticker:   time.NewTicker(interval),
        tokens:   make(chan struct{}, rate),
    }
    
    // Fill bucket
    for i := 0; i < rate; i++ {
        rl.tokens <- struct{}{}
    }
    
    // Refill tokens
    go func() {
        for range rl.ticker.C {
            select {
            case rl.tokens <- struct{}{}:
            default:
            }
        }
    }()
    
    return rl
}

func (rl *RateLimiter) Wait() {
    <-rl.tokens
}

func (rl *RateLimiter) Stop() {
    rl.ticker.Stop()
}
```

## Error Handling Patterns

### Wrap Errors with Context
```go
func readConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config file %s: %w", path, err)
    }
    
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse config file %s: %w", path, err)
    }
    
    return &cfg, nil
}
```

### Multiple Error Collection
```go
type MultiError struct {
    Errors []error
}

func (m *MultiError) Error() string {
    var msgs []string
    for _, err := range m.Errors {
        msgs = append(msgs, err.Error())
    }
    return strings.Join(msgs, "; ")
}

func (m *MultiError) Add(err error) {
    if err != nil {
        m.Errors = append(m.Errors, err)
    }
}

func (m *MultiError) ErrorOrNil() error {
    if len(m.Errors) == 0 {
        return nil
    }
    return m
}
```

## Resource Management

### Using defer for Cleanup
```go
func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()
    
    // Process file
    return nil
}
```

### Resource Pool
```go
type Pool struct {
    resources chan Resource
    factory   func() Resource
}

func NewPool(size int, factory func() Resource) *Pool {
    p := &Pool{
        resources: make(chan Resource, size),
        factory:   factory,
    }
    
    for i := 0; i < size; i++ {
        p.resources <- factory()
    }
    
    return p
}

func (p *Pool) Acquire() Resource {
    select {
    case r := <-p.resources:
        return r
    default:
        return p.factory()
    }
}

func (p *Pool) Release(r Resource) {
    select {
    case p.resources <- r:
    default:
        // Pool full, discard resource
    }
}
```

## Go Idioms

### Early Return
```go
// ❌ BAD
func process(data string) error {
    if data != "" {
        if len(data) > 10 {
            if isValid(data) {
                // process
                return nil
            } else {
                return errors.New("invalid data")
            }
        } else {
            return errors.New("data too short")
        }
    } else {
        return errors.New("empty data")
    }
}

// ✅ GOOD
func process(data string) error {
    if data == "" {
        return errors.New("empty data")
    }
    if len(data) <= 10 {
        return errors.New("data too short")
    }
    if !isValid(data) {
        return errors.New("invalid data")
    }
    
    // process
    return nil
}
```

### Embedded Interfaces
```go
type ReadWriter interface {
    io.Reader
    io.Writer
}
```

### Method Chaining
```go
type Builder struct {
    value string
}

func (b *Builder) Add(s string) *Builder {
    b.value += s
    return b
}

func (b *Builder) Build() string {
    return b.value
}

// Usage
result := NewBuilder().Add("Hello").Add(" ").Add("World").Build()
```
