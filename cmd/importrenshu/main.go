package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"chinese-medical/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "import renshu sql: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "path to config yaml")
	sqlPath := flag.String("file", "", "path to RenShu-AI PostgreSQL SQL file")
	statusOnly := flag.Bool("status", false, "only print imported RenShu-AI table status")
	flag.Parse()

	if strings.TrimSpace(*sqlPath) == "" && !*statusOnly {
		return errors.New("-file is required")
	}
	if strings.TrimSpace(*configPath) != "" {
		if err := os.Setenv("CONFIG_FILE", *configPath); err != nil {
			return fmt.Errorf("set CONFIG_FILE: %w", err)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	if *statusOnly {
		return printStatus(ctx, pool)
	}

	data, err := os.ReadFile(*sqlPath)
	if err != nil {
		return fmt.Errorf("read sql file: %w", err)
	}

	sql := prepareRenShuSQL(string(data))
	statements := splitSQLStatements(sql)
	if len(statements) == 0 {
		return errors.New("no executable SQL statements found")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin import transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	executed := 0
	for _, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("execute statement %d (%s): %w", executed+1, statementSummary(statement), err)
		}
		executed++
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit import transaction: %w", err)
	}

	fmt.Printf("imported RenShu-AI PostgreSQL schema: %d statements executed\n", executed)
	return nil
}

func printStatus(ctx context.Context, pool *pgxpool.Pool) error {
	tables := []string{
		"users",
		"patients",
		"medical_cases",
		"symptoms",
		"syndromes",
		"herbs",
		"herb_inventory",
		"prescriptions",
		"conversations",
		"messages",
		"tongue_analysis",
		"prescription_recommendations",
		"classic_texts",
		"medical_records",
		"system_configs",
		"user_states",
		"user_sessions",
		"user_activities",
		"refresh_tokens",
	}

	existing := 0
	for _, table := range tables {
		var ok bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)
		`, table).Scan(&ok); err != nil {
			return fmt.Errorf("check table %s: %w", table, err)
		}
		if ok {
			existing++
		}
	}

	var configCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM system_configs`).Scan(&configCount); err != nil {
		return fmt.Errorf("count system_configs: %w", err)
	}

	fmt.Printf("RenShu-AI tables present: %d/%d\n", existing, len(tables))
	fmt.Printf("system_configs rows: %d\n", configCount)
	return nil
}

func prepareRenShuSQL(sql string) string {
	lines := strings.Split(sql, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "CREATE DATABASE ") || strings.HasPrefix(trimmed, `\c `) {
			continue
		}
		kept = append(kept, line)
	}
	sql = strings.Join(kept, "\n")

	tableRe := regexp.MustCompile(`(?m)^CREATE TABLE ([A-Za-z_][A-Za-z0-9_]*)`)
	sql = tableRe.ReplaceAllString(sql, `CREATE TABLE IF NOT EXISTS $1`)

	indexRe := regexp.MustCompile(`(?m)^CREATE INDEX ([A-Za-z_][A-Za-z0-9_]*)`)
	sql = indexRe.ReplaceAllString(sql, `CREATE INDEX IF NOT EXISTS $1`)

	sql = strings.ReplaceAll(sql, "INSERT INTO system_configs", "INSERT INTO system_configs")
	sql = strings.ReplaceAll(sql, "::text]))\n);", "::text])))\n);")
	sql = replaceUpdateHerbInventory(sql)
	sql = strings.Replace(sql,
		"('supported_image_formats', '[\"jpg\", \"jpeg\", \"png\", \"bmp\"]', 'json', '支持的图片格式');",
		"('supported_image_formats', '[\"jpg\", \"jpeg\", \"png\", \"bmp\"]', 'json', '支持的图片格式') ON CONFLICT (config_key) DO NOTHING;",
		1,
	)

	return sql
}

func replaceUpdateHerbInventory(sql string) string {
	re := regexp.MustCompile(`(?s)CREATE OR REPLACE FUNCTION UpdateHerbInventory\(.*?\$\$ LANGUAGE plpgsql;`)
	replacement := `CREATE OR REPLACE FUNCTION UpdateHerbInventory(
    herb_id_param UUID,
    quantity_change_param DECIMAL(10,3),
    operation_type_param VARCHAR(10)
)
RETURNS VOID AS $$
BEGIN
    IF operation_type_param = 'add' THEN
        UPDATE herb_inventory
        SET quantity = quantity + quantity_change_param,
            updated_at = CURRENT_TIMESTAMP
        WHERE ctid = (
            SELECT ctid
            FROM herb_inventory
            WHERE herb_id = herb_id_param AND status = 'available'
            ORDER BY created_at DESC
            LIMIT 1
        );
    ELSE
        UPDATE herb_inventory
        SET quantity = GREATEST(0, quantity - quantity_change_param),
            status = CASE
                WHEN quantity - quantity_change_param <= 0 THEN 'out_of_stock'
                WHEN quantity - quantity_change_param <= 10 THEN 'low_stock'
                ELSE 'available'
            END,
            updated_at = CURRENT_TIMESTAMP
        WHERE ctid = (
            SELECT ctid
            FROM herb_inventory
            WHERE herb_id = herb_id_param AND status = 'available'
            ORDER BY created_at DESC
            LIMIT 1
        );
    END IF;
END;
$$ LANGUAGE plpgsql;`
	return re.ReplaceAllStringFunc(sql, func(string) string {
		return replacement
	})
}

func splitSQLStatements(sql string) []string {
	var statements []string
	var builder strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	dollarTag := ""

	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		next := byte(0)
		if i+1 < len(sql) {
			next = sql[i+1]
		}

		if !inSingleQuote && !inDoubleQuote && dollarTag == "" && ch == '-' && next == '-' {
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			if i < len(sql) {
				builder.WriteByte('\n')
			}
			continue
		}

		if !inSingleQuote && !inDoubleQuote && ch == '$' {
			if dollarTag == "" {
				if tag, ok := readDollarTag(sql[i:]); ok {
					dollarTag = tag
					builder.WriteString(tag)
					i += len(tag) - 1
					continue
				}
			} else if strings.HasPrefix(sql[i:], dollarTag) {
				builder.WriteString(dollarTag)
				i += len(dollarTag) - 1
				dollarTag = ""
				continue
			}
		}

		if dollarTag == "" && !inDoubleQuote && ch == '\'' {
			builder.WriteByte(ch)
			if inSingleQuote && next == '\'' {
				i++
				builder.WriteByte(next)
				continue
			}
			inSingleQuote = !inSingleQuote
			continue
		}

		if dollarTag == "" && !inSingleQuote && ch == '"' {
			inDoubleQuote = !inDoubleQuote
			builder.WriteByte(ch)
			continue
		}

		if dollarTag == "" && !inSingleQuote && !inDoubleQuote && ch == ';' {
			if statement := strings.TrimSpace(builder.String()); statement != "" {
				statements = append(statements, statement)
			}
			builder.Reset()
			continue
		}

		builder.WriteByte(ch)
	}

	if statement := strings.TrimSpace(builder.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}

func readDollarTag(sql string) (string, bool) {
	if len(sql) < 2 || sql[0] != '$' {
		return "", false
	}
	for i := 1; i < len(sql); i++ {
		if sql[i] == '$' {
			return sql[:i+1], true
		}
		if sql[i] != '_' && !unicode.IsLetter(rune(sql[i])) && !unicode.IsDigit(rune(sql[i])) {
			return "", false
		}
	}
	return "", false
}

func statementSummary(statement string) string {
	words := strings.Fields(statement)
	if len(words) > 8 {
		words = words[:8]
	}
	summary := strings.Join(words, " ")
	if len(summary) > 120 {
		summary = summary[:120] + "..."
	}
	return summary
}
