# Essential Go Standard Library Packages

## I/O Operations

### `io` Package
Core interfaces for I/O operations:
```go
// Copy data from reader to writer
n, err := io.Copy(dst, src)

// Read all data from reader
data, err := io.ReadAll(reader)

// Create multi-reader
r := io.MultiReader(r1, r2, r3)

// Create multi-writer
w := io.MultiWriter(w1, w2, w3)

// Limit reader
limited := io.LimitReader(reader, 1024)
```

### `os` Package
Operating system functionality:
```go
// File operations
file, err := os.Open("file.txt")
file, err := os.Create("file.txt")
file, err := os.OpenFile("file.txt", os.O_RDWR|os.O_CREATE, 0644)

// Read file
data, err := os.ReadFile("file.txt")

// Write file
err := os.WriteFile("file.txt", data, 0644)

// File info
info, err := os.Stat("file.txt")
fmt.Println(info.Size(), info.ModTime())

// Directory operations
err := os.Mkdir("dir", 0755)
err := os.MkdirAll("path/to/dir", 0755)
entries, err := os.ReadDir("dir")
err := os.Remove("file.txt")
err := os.RemoveAll("dir")

// Environment variables
value := os.Getenv("PATH")
err := os.Setenv("KEY", "value")
envs := os.Environ()

// Working directory
dir, err := os.Getwd()
err := os.Chdir("/path/to/dir")
```

### `bufio` Package
Buffered I/O:
```go
// Buffered reader
reader := bufio.NewReader(file)
line, err := reader.ReadString('\n')
line, err := reader.ReadBytes('\n')

// Scanner (for line-by-line reading)
scanner := bufio.NewScanner(file)
for scanner.Scan() {
    line := scanner.Text()
    fmt.Println(line)
}

// Buffered writer
writer := bufio.NewWriter(file)
_, err := writer.WriteString("hello\n")
err := writer.Flush()
```

## String Manipulation

### `strings` Package
```go
// Searching
contains := strings.Contains(s, "substring")
index := strings.Index(s, "substring")
count := strings.Count(s, "substring")
hasPrefix := strings.HasPrefix(s, "prefix")
hasSuffix := strings.HasSuffix(s, "suffix")

// Splitting and joining
parts := strings.Split(s, ",")
joined := strings.Join(parts, ",")
fields := strings.Fields(s) // split on whitespace

// Trimming
trimmed := strings.TrimSpace(s)
trimmed := strings.Trim(s, "cutset")
trimmed := strings.TrimPrefix(s, "prefix")
trimmed := strings.TrimSuffix(s, "suffix")

// Replacing
replaced := strings.Replace(s, "old", "new", n)
replaced := strings.ReplaceAll(s, "old", "new")

// Case conversion
upper := strings.ToUpper(s)
lower := strings.ToLower(s)
title := strings.Title(s)

// String builder (efficient concatenation)
var builder strings.Builder
builder.WriteString("hello")
builder.WriteString(" world")
result := builder.String()
```

### `strconv` Package
String conversions:
```go
// String to int
i, err := strconv.Atoi("123")
i, err := strconv.ParseInt("123", 10, 64)

// Int to string
s := strconv.Itoa(123)
s := strconv.FormatInt(123, 10)

// String to float
f, err := strconv.ParseFloat("3.14", 64)

// Float to string
s := strconv.FormatFloat(3.14, 'f', 2, 64)

// String to bool
b, err := strconv.ParseBool("true")

// Bool to string
s := strconv.FormatBool(true)
```

## Formatting

### `fmt` Package
```go
// Print functions
fmt.Print("hello")
fmt.Println("hello")
fmt.Printf("Hello %s\n", name)

// Format to string
s := fmt.Sprintf("Hello %s", name)

// Scan functions
var name string
fmt.Scan(&name)
fmt.Scanf("%s", &name)

// Error formatting
err := fmt.Errorf("error: %w", originalErr)

// Common format verbs:
// %v - default format
// %+v - with field names
// %#v - Go syntax
// %T - type
// %t - bool
// %d - decimal int
// %f - float
// %s - string
// %q - quoted string
// %p - pointer
```

## Time Operations

### `time` Package
```go
// Current time
now := time.Now()

// Create time
t := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

// Duration
d := 5 * time.Second
d := time.ParseDuration("1h30m")

// Time arithmetic
future := now.Add(24 * time.Hour)
past := now.Add(-24 * time.Hour)
diff := t2.Sub(t1)

// Formatting and parsing
formatted := now.Format("2006-01-02 15:04:05")
parsed, err := time.Parse("2006-01-02", "2024-01-01")

// Common layouts
time.RFC3339     // "2006-01-02T15:04:05Z07:00"
time.RFC822      // "02 Jan 06 15:04 MST"
time.Kitchen     // "3:04PM"

// Timers and tickers
timer := time.NewTimer(5 * time.Second)
<-timer.C

ticker := time.NewTicker(1 * time.Second)
for t := range ticker.C {
    fmt.Println(t)
}
ticker.Stop()

// Sleep
time.Sleep(1 * time.Second)
```

## Encoding/Decoding

### `encoding/json` Package
```go
type Person struct {
    Name  string `json:"name"`
    Age   int    `json:"age"`
    Email string `json:"email,omitempty"`
}

// Marshal (encode)
data, err := json.Marshal(person)
data, err := json.MarshalIndent(person, "", "  ")

// Unmarshal (decode)
var person Person
err := json.Unmarshal(data, &person)

// Encode to writer
encoder := json.NewEncoder(writer)
err := encoder.Encode(person)

// Decode from reader
decoder := json.NewDecoder(reader)
err := decoder.Decode(&person)

// Raw message (delay parsing)
type Response struct {
    Status string          `json:"status"`
    Data   json.RawMessage `json:"data"`
}
```

### `encoding/xml` Package
```go
type Person struct {
    XMLName xml.Name `xml:"person"`
    Name    string   `xml:"name"`
    Age     int      `xml:"age"`
}

data, err := xml.Marshal(person)
data, err := xml.MarshalIndent(person, "", "  ")
err := xml.Unmarshal(data, &person)
```

### `encoding/base64` Package
```go
// Standard encoding
encoded := base64.StdEncoding.EncodeToString(data)
decoded, err := base64.StdEncoding.DecodeString(encoded)

// URL encoding (URL-safe)
encoded := base64.URLEncoding.EncodeToString(data)
decoded, err := base64.URLEncoding.DecodeString(encoded)
```

## Networking

### `net/http` Package

**HTTP Server:**
```go
// Simple handler
http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello, %s!", r.URL.Path[1:])
})

// Start server
log.Fatal(http.ListenAndServe(":8080", nil))

// With custom server
server := &http.Server{
    Addr:         ":8080",
    Handler:      handler,
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 10 * time.Second,
}
log.Fatal(server.ListenAndServe())

// Middleware
func logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Printf("%s %s", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
    })
}
```

**HTTP Client:**
```go
// Simple GET
resp, err := http.Get("https://api.example.com/data")
defer resp.Body.Close()
body, err := io.ReadAll(resp.Body)

// Custom request
req, err := http.NewRequest("POST", url, body)
req.Header.Set("Content-Type", "application/json")
resp, err := http.DefaultClient.Do(req)

// Custom client
client := &http.Client{
    Timeout: 10 * time.Second,
}
resp, err := client.Do(req)
```

### `net/url` Package
```go
// Parse URL
u, err := url.Parse("https://example.com/path?key=value")
fmt.Println(u.Scheme, u.Host, u.Path)

// Build query string
params := url.Values{}
params.Add("key", "value")
params.Add("foo", "bar")
queryString := params.Encode()

// URL encoding
encoded := url.QueryEscape("hello world")
decoded, err := url.QueryUnescape(encoded)
```

## Concurrency

### `sync` Package
```go
// Mutex
var mu sync.Mutex
mu.Lock()
// critical section
mu.Unlock()

// RWMutex
var rwmu sync.RWMutex
rwmu.RLock()
// read operation
rwmu.RUnlock()

rwmu.Lock()
// write operation
rwmu.Unlock()

// WaitGroup
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        // work
    }()
}
wg.Wait()

// Once
var once sync.Once
once.Do(func() {
    // executed only once
})

// Pool
var pool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}
buf := pool.Get().(*bytes.Buffer)
defer pool.Put(buf)
```

### `context` Package
```go
// Background context
ctx := context.Background()

// With timeout
ctx, cancel := context.WithTimeout(parentCtx, 5*time.Second)
defer cancel()

// With deadline
deadline := time.Now().Add(10 * time.Second)
ctx, cancel := context.WithDeadline(parentCtx, deadline)
defer cancel()

// With cancel
ctx, cancel := context.WithCancel(parentCtx)
defer cancel()

// With value
ctx = context.WithValue(ctx, "key", "value")
value := ctx.Value("key").(string)

// Using context
select {
case <-ctx.Done():
    return ctx.Err() // context.Canceled or context.DeadlineExceeded
case result := <-ch:
    // process result
}
```

## Testing

### `testing` Package
```go
// Test function
func TestAdd(t *testing.T) {
    result := Add(2, 3)
    if result != 5 {
        t.Errorf("Add(2, 3) = %d; want 5", result)
    }
}

// Subtests
func TestMath(t *testing.T) {
    t.Run("Addition", func(t *testing.T) {
        // test addition
    })
    t.Run("Subtraction", func(t *testing.T) {
        // test subtraction
    })
}

// Benchmark
func BenchmarkAdd(b *testing.B) {
    for i := 0; i < b.N; i++ {
        Add(2, 3)
    }
}

// Test helpers
func TestMain(m *testing.M) {
    // setup
    code := m.Run()
    // teardown
    os.Exit(code)
}

// Skip test
if testing.Short() {
    t.Skip("skipping test in short mode")
}

// Parallel tests
func TestParallel(t *testing.T) {
    t.Parallel()
    // test code
}
```

## Error Handling

### `errors` Package
```go
// Create error
err := errors.New("error message")

// Wrap error
err = fmt.Errorf("context: %w", originalErr)

// Check error type
if errors.Is(err, os.ErrNotExist) {
    // handle not exist error
}

// As - type assertion for errors
var pathErr *os.PathError
if errors.As(err, &pathErr) {
    fmt.Println("Path error:", pathErr.Path)
}

// Unwrap
originalErr := errors.Unwrap(err)
```

## Regular Expressions

### `regexp` Package
```go
// Compile regex
re := regexp.MustCompile(`\d+`)

// Match
matched := re.MatchString("abc123")

// Find
result := re.FindString("abc123def")
results := re.FindAllString("abc123def456", -1)

// Replace
replaced := re.ReplaceAllString("abc123def", "XXX")

// Submatch
re := regexp.MustCompile(`(\w+)@(\w+)\.(\w+)`)
matches := re.FindStringSubmatch("user@example.com")
// matches[0] = "user@example.com"
// matches[1] = "user"
// matches[2] = "example"
// matches[3] = "com"
```

## Math Operations

### `math` Package
```go
// Basic functions
abs := math.Abs(-5.5)
ceil := math.Ceil(3.14)
floor := math.Floor(3.14)
round := math.Round(3.5)
max := math.Max(5, 10)
min := math.Min(5, 10)

// Power and roots
pow := math.Pow(2, 3)
sqrt := math.Sqrt(16)

// Trigonometry
sin := math.Sin(math.Pi / 2)
cos := math.Cos(0)

// Constants
pi := math.Pi
e := math.E
```

### `math/rand` Package
```go
// Seed (use crypto/rand for security)
rand.Seed(time.Now().UnixNano())

// Random numbers
n := rand.Int()
n := rand.Intn(100)        // 0 <= n < 100
f := rand.Float64()        // 0.0 <= f < 1.0

// Random choice
choices := []string{"a", "b", "c"}
choice := choices[rand.Intn(len(choices))]

// Shuffle
rand.Shuffle(len(slice), func(i, j int) {
    slice[i], slice[j] = slice[j], slice[i]
})
```
