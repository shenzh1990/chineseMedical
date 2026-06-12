# chinese-medical

Gin web service with embedded HTML assets, PostgreSQL, and Redis.

## Quick start

```bash
docker compose up -d postgres redis
go run ./cmd/server
```

Open http://localhost:8080.

## Configuration

The service reads configuration from `configs/config.yaml` by default. Set `CONFIG_FILE` to use another YAML file:

```bash
CONFIG_FILE=configs/config.production.yaml go run ./cmd/server
```

Database and Redis addresses are configured in YAML:

```yaml
database:
  url: postgres://postgres:postgres@localhost:5432/chinese_medical?sslmode=disable

redis:
  addr: localhost:6379
```

The default values target the local `docker-compose.yml` services.

Image generation is configured in the same YAML file:

```yaml
ai:
  base_url: https://api.openai.com/v1
  endpoint_path: /images/generations
  api_key: ""
  api_key_env: OPENAI_API_KEY
  model: gpt-image-1
  image_count: 4
  size: 720x1280
  quality: medium
  output_format: png
  output_dir: generated
  timeout: 120s
```

Set the API key in the environment variable named by `api_key_env`.
You can also set `api_key` directly in YAML. When both are present, `api_key` takes precedence. `base_url` and `endpoint_path` support OpenAI-compatible image generation services, not only OpenAI itself.

## Endpoints

- `GET /login` renders the login page.
- `GET /` renders the embedded HTML page after login.
- `GET /tools/image-splitter` opens the local image splitting tool.
- `GET /foods/:id/images` opens the formula image generation page.
- `POST /foods/:id/images/generate` generates formula introduction images.
- `GET /healthz` checks PostgreSQL and Redis.
- `GET /api/health` returns the same health payload for API consumers.

On startup, the server ensures an `app_users` table exists and creates a default administrator when missing:

- Username: `admin`
- Password: value of `ADMIN_PASSWORD`, or `admin` when the variable is not set.

Set `SESSION_SECRET` in production to sign login cookies with a private secret.

## Sync SQL data

The SQL file under `sql/t_medicated_food.sql` can be synced into the PostgreSQL database configured in YAML:

```bash
go run ./cmd/syncsql -config configs/config.yaml -file sql/t_medicated_food.sql
```

The command creates `t_medicated_food` when it does not exist, then upserts rows by `id`. The configured database user must either have `CREATE` permission on the target schema, or the table must already exist.

If the account cannot create tables, create the table with a privileged account first:

```sql
CREATE TABLE IF NOT EXISTS t_medicated_food (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    food TEXT NOT NULL DEFAULT '',
    method TEXT NOT NULL DEFAULT '',
    effect TEXT NOT NULL DEFAULT '',
    create_by TEXT,
    create_time TIMESTAMPTZ,
    update_by TEXT,
    update_time TIMESTAMPTZ
);
```

## Build

```bash
go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server
```

The HTML templates and static assets under `internal/web` are embedded into the binary.
