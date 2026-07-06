package importer

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"chinese-medical/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/html"
)

type PatentFoodSyncResult struct {
	Parsed   int
	Inserted int
	Skipped  int
}

type patentFoodEntry struct {
	model.MedicatedFood
	Category string
	Doctor   string
	Patent   string
}

var patentFoodHeaderPattern = regexp.MustCompile(`^\d+\.\s*(.*?)\s*——\s*(.*?)（专利号：([^）]+)）$`)
var genericFoodHeaderPattern = regexp.MustCompile(`^\d+\.\s*(.+?)(?:（([^）]+)）)?$`)

func SyncPatentFoodHTML(ctx context.Context, db *pgxpool.Pool, path string) (PatentFoodSyncResult, error) {
	items, err := ParsePatentFoodHTML(path)
	if err != nil {
		return PatentFoodSyncResult{}, err
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return PatentFoodSyncResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	exists, err := medicatedFoodTableExists(ctx, tx)
	if err != nil {
		return PatentFoodSyncResult{}, fmt.Errorf("check t_medicated_food: %w", err)
	}
	if !exists {
		if _, err := tx.Exec(ctx, medicatedFoodSchema); err != nil {
			return PatentFoodSyncResult{}, fmt.Errorf("create t_medicated_food: %w", err)
		}
	}
	if err := ensureMedicatedFoodCategory(ctx, tx); err != nil {
		return PatentFoodSyncResult{}, err
	}

	result := PatentFoodSyncResult{Parsed: len(items)}
	for _, item := range items {
		tag, err := tx.Exec(ctx, `
			INSERT INTO t_medicated_food (id, category, name, source, food, method, effect)
			SELECT COALESCE((SELECT MAX(id) FROM t_medicated_food), 0) + 1, $1, $2, $3, $4, $5, $6
			WHERE NOT EXISTS (
				SELECT 1 FROM t_medicated_food WHERE name = $2
			)
		`, model.DefaultFoodCategory, item.Name, formatPatentFoodSource(item), item.Food, item.Method, item.Effect)
		if err != nil {
			return PatentFoodSyncResult{}, fmt.Errorf("insert patent food %q: %w", item.Name, err)
		}

		if tag.RowsAffected() == 0 {
			result.Skipped++
		} else {
			result.Inserted++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return PatentFoodSyncResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	return result, nil
}

func ParsePatentFoodHTML(path string) ([]patentFoodEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open html file %q: %w", path, err)
	}
	defer file.Close()

	doc, err := html.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("parse html file %q: %w", path, err)
	}

	content := findNodeByID(doc, "js_content")
	if content == nil {
		content = doc
	}

	lines := strings.Split(extractText(content), "\n")
	items := []patentFoodEntry{}
	var current *patentFoodEntry
	category := ""

	flush := func() {
		if current == nil || current.Name == "" {
			return
		}
		items = append(items, *current)
		current = nil
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if isPatentFoodCategory(line) {
			flush()
			category = line
			continue
		}

		if matches := patentFoodHeaderPattern.FindStringSubmatch(line); len(matches) == 4 {
			flush()
			current = &patentFoodEntry{
				MedicatedFood: model.MedicatedFood{
					Name: strings.TrimSpace(matches[2]),
				},
				Category: category,
				Doctor:   strings.TrimSpace(matches[1]),
				Patent:   strings.TrimSpace(matches[3]),
			}
			continue
		}

		if matches := genericFoodHeaderPattern.FindStringSubmatch(line); len(matches) == 3 && looksLikeFoodHeader(line) {
			flush()
			current = &patentFoodEntry{
				MedicatedFood: model.MedicatedFood{
					Name: strings.TrimSpace(matches[1]),
				},
				Category: category,
				Patent:   strings.TrimSpace(matches[2]),
			}
			continue
		}

		if current == nil {
			continue
		}

		key, value, ok := strings.Cut(line, "：")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "出处":
			current.Source = value
		case "组方", "配方":
			current.Food = value
		case "功效", "主治":
			current.Effect = value
		case "适用", "用法":
			current.Method = value
		}
	}
	flush()

	if len(items) == 0 {
		return nil, fmt.Errorf("no patent food entries found in %q", path)
	}

	return items, nil
}

func formatPatentFoodSource(item patentFoodEntry) string {
	parts := []string{}
	if item.Category != "" {
		parts = append(parts, item.Category)
	}
	if item.Doctor != "" {
		parts = append(parts, item.Doctor)
	}
	if item.Patent != "" {
		parts = append(parts, "专利号："+item.Patent)
	}
	if item.Source != "" {
		parts = append(parts, item.Source)
	}
	return strings.Join(parts, "；")
}

func isPatentFoodCategory(line string) bool {
	return regexp.MustCompile(`^[一二三四五六七八九十]+、`).MatchString(line)
}

func looksLikeFoodHeader(line string) bool {
	if strings.Contains(line, "：") {
		return false
	}
	return !isPatentFoodCategory(line)
}

func findNodeByID(node *html.Node, id string) *html.Node {
	if node.Type == html.ElementNode {
		for _, attr := range node.Attr {
			if attr.Key == "id" && attr.Val == id {
				return node
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findNodeByID(child, id); found != nil {
			return found
		}
	}
	return nil
}

func extractText(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				builder.WriteString(text)
				builder.WriteByte('\n')
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "section", "div", "br", "li", "tr", "h1", "h2", "h3", "h4":
				builder.WriteByte('\n')
			}
		}
	}
	walk(node)
	return builder.String()
}
