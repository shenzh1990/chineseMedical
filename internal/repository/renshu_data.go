package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RenShuDataRepository struct {
	db *pgxpool.Pool
}

type RenShuDataset struct {
	Key         string
	Name        string
	Table       string
	Description string
	Columns     []RenShuColumn
	SearchSQL   string
	OrderBy     string
}

type RenShuColumn struct {
	Key   string
	Label string
}

type RenShuDatasetSummary struct {
	Key   string
	Name  string
	Count int64
}

type RenShuDataPage struct {
	Dataset RenShuDataset
	Rows    []map[string]string
	Total   int64
}

func NewRenShuDataRepository(db *pgxpool.Pool) RenShuDataRepository {
	return RenShuDataRepository{db: db}
}

func RenShuDatasets() []RenShuDataset {
	return []RenShuDataset{
		{
			Key:         "herbs",
			Name:        "药材库",
			Table:       "herbs",
			Description: "药材功效、性味归经、禁忌与配伍信息。",
			Columns: []RenShuColumn{
				{Key: "name", Label: "药材"},
				{Key: "category", Label: "分类"},
				{Key: "nature", Label: "药性"},
				{Key: "flavor", Label: "五味"},
				{Key: "meridian", Label: "归经"},
				{Key: "effect", Label: "功效"},
				{Key: "contraindications", Label: "禁忌"},
			},
			SearchSQL: "name ILIKE $1 OR category ILIKE $1 OR effect ILIKE $1 OR indication ILIKE $1",
			OrderBy:   "created_at DESC, name",
		},
		{
			Key:         "prescriptions",
			Name:        "方剂库",
			Table:       "prescriptions",
			Description: "方剂出处、组成、制法、主治和证候适应。",
			Columns: []RenShuColumn{
				{Key: "name", Label: "方剂"},
				{Key: "source", Label: "出处"},
				{Key: "category", Label: "分类"},
				{Key: "composition", Label: "组成"},
				{Key: "indication", Label: "主治"},
				{Key: "syndrome_adaptation", Label: "适应证候"},
			},
			SearchSQL: "name ILIKE $1 OR source ILIKE $1 OR category ILIKE $1 OR indication ILIKE $1",
			OrderBy:   "created_at DESC, name",
		},
		{
			Key:         "classic_texts",
			Name:        "古籍条文",
			Table:       "classic_texts",
			Description: "中医经典原文、译文、关键词与关联方剂。",
			Columns: []RenShuColumn{
				{Key: "title", Label: "标题"},
				{Key: "chapter", Label: "章节"},
				{Key: "article_number", Label: "条文号"},
				{Key: "content", Label: "原文"},
				{Key: "translation", Label: "译文"},
				{Key: "annotation", Label: "注释"},
			},
			SearchSQL: "title ILIKE $1 OR chapter ILIKE $1 OR content ILIKE $1 OR translation ILIKE $1 OR annotation ILIKE $1",
			OrderBy:   "created_at DESC, title",
		},
		{
			Key:         "medical_records",
			Name:        "医案库",
			Table:       "medical_records",
			Description: "临床医案、症状、辨证、治法、方药和疗效。",
			Columns: []RenShuColumn{
				{Key: "case_title", Label: "医案标题"},
				{Key: "patient_age", Label: "年龄"},
				{Key: "patient_gender", Label: "性别"},
				{Key: "chief_complaint", Label: "主诉"},
				{Key: "symptoms", Label: "症状"},
				{Key: "syndrome_diagnosis", Label: "辨证"},
				{Key: "treatment_principle", Label: "治法"},
				{Key: "prescription", Label: "方药"},
			},
			SearchSQL: "case_title ILIKE $1 OR chief_complaint ILIKE $1 OR symptoms ILIKE $1 OR syndrome_diagnosis ILIKE $1 OR prescription ILIKE $1",
			OrderBy:   "created_at DESC, case_title",
		},
		{
			Key:         "symptoms",
			Name:        "症状库",
			Table:       "symptoms",
			Description: "症状分类、描述、相关证型和严重程度。",
			Columns: []RenShuColumn{
				{Key: "name", Label: "症状"},
				{Key: "category", Label: "分类"},
				{Key: "description", Label: "描述"},
				{Key: "related_syndromes", Label: "相关证型"},
				{Key: "severity_levels", Label: "严重程度"},
			},
			SearchSQL: "name ILIKE $1 OR category ILIKE $1 OR description ILIKE $1 OR related_syndromes ILIKE $1",
			OrderBy:   "created_at DESC, name",
		},
		{
			Key:         "syndromes",
			Name:        "证型库",
			Table:       "syndromes",
			Description: "证型描述、症状特征、治则治法和常用方剂。",
			Columns: []RenShuColumn{
				{Key: "name", Label: "证型"},
				{Key: "category", Label: "分类"},
				{Key: "description", Label: "描述"},
				{Key: "main_symptoms", Label: "主要症状"},
				{Key: "treatment_principle", Label: "治则治法"},
				{Key: "common_prescriptions", Label: "常用方剂"},
			},
			SearchSQL: "name ILIKE $1 OR category ILIKE $1 OR description ILIKE $1 OR main_symptoms ILIKE $1 OR treatment_principle ILIKE $1",
			OrderBy:   "created_at DESC, name",
		},
		{
			Key:         "system_configs",
			Name:        "系统配置",
			Table:       "system_configs",
			Description: "RenShu-AI 初始化脚本内置的系统能力开关和默认配置。",
			Columns: []RenShuColumn{
				{Key: "config_key", Label: "配置键"},
				{Key: "config_value", Label: "配置值"},
				{Key: "config_type", Label: "类型"},
				{Key: "description", Label: "说明"},
				{Key: "is_active", Label: "启用"},
			},
			SearchSQL: "config_key ILIKE $1 OR config_value ILIKE $1 OR description ILIKE $1",
			OrderBy:   "config_key",
		},
	}
}

func RenShuDatasetByKey(key string) (RenShuDataset, bool) {
	key = strings.TrimSpace(key)
	for _, dataset := range RenShuDatasets() {
		if dataset.Key == key {
			return dataset, true
		}
	}
	return RenShuDataset{}, false
}

func (r RenShuDataRepository) Summaries(ctx context.Context) ([]RenShuDatasetSummary, error) {
	datasets := RenShuDatasets()
	summaries := make([]RenShuDatasetSummary, 0, len(datasets))
	for _, dataset := range datasets {
		var count int64
		if err := r.db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", dataset.Table)).Scan(&count); err != nil {
			return nil, fmt.Errorf("count %s: %w", dataset.Table, err)
		}
		summaries = append(summaries, RenShuDatasetSummary{
			Key:   dataset.Key,
			Name:  dataset.Name,
			Count: count,
		})
	}
	return summaries, nil
}

func (r RenShuDataRepository) List(ctx context.Context, dataset RenShuDataset, query string, limit, offset int) (RenShuDataPage, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	query = strings.TrimSpace(query)
	where := ""
	args := []any{}
	if query != "" {
		args = append(args, "%"+query+"%")
		where = "WHERE " + dataset.SearchSQL
	}

	var total int64
	if err := r.db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s %s", dataset.Table, where), args...).Scan(&total); err != nil {
		return RenShuDataPage{}, fmt.Errorf("count %s rows: %w", dataset.Table, err)
	}

	selectParts := make([]string, 0, len(dataset.Columns))
	for _, column := range dataset.Columns {
		selectParts = append(selectParts, fmt.Sprintf("COALESCE(%s::text, '') AS %s", column.Key, column.Key))
	}

	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	args = append(args, limit, offset)

	sql := fmt.Sprintf(`
		SELECT %s
		FROM %s
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, strings.Join(selectParts, ", "), dataset.Table, where, dataset.OrderBy, limitArg, offsetArg)

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return RenShuDataPage{}, fmt.Errorf("query %s rows: %w", dataset.Table, err)
	}
	defer rows.Close()

	items := []map[string]string{}
	for rows.Next() {
		values := make([]string, len(dataset.Columns))
		scanTargets := make([]any, len(dataset.Columns))
		for i := range values {
			scanTargets[i] = &values[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return RenShuDataPage{}, fmt.Errorf("scan %s row: %w", dataset.Table, err)
		}
		item := map[string]string{}
		for i, column := range dataset.Columns {
			item[column.Key] = values[i]
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return RenShuDataPage{}, fmt.Errorf("iterate %s rows: %w", dataset.Table, err)
	}

	return RenShuDataPage{
		Dataset: dataset,
		Rows:    items,
		Total:   total,
	}, nil
}
