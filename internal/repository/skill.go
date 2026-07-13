package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"chinese-medical/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SkillRepository struct {
	db *pgxpool.Pool
}

type SkillFilter struct {
	Query    string
	Category string
	Enabled  string
}

func NewSkillRepository(db *pgxpool.Pool) SkillRepository {
	return SkillRepository{db: db}
}

func (r SkillRepository) Get(ctx context.Context, id int64) (model.Skill, error) {
	var skill model.Skill
	if err := r.db.QueryRow(ctx, `
		SELECT id, name, slug, category, description, instruction, tags, enabled, created_at, updated_at
		FROM ai_skills
		WHERE id = $1
	`, id).Scan(
		&skill.ID,
		&skill.Name,
		&skill.Slug,
		&skill.Category,
		&skill.Description,
		&skill.Instruction,
		&skill.Tags,
		&skill.Enabled,
		&skill.CreatedAt,
		&skill.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Skill{}, fmt.Errorf("skill %d not found: %w", id, err)
		}
		return model.Skill{}, fmt.Errorf("get skill: %w", err)
	}
	return skill, nil
}

func (r SkillRepository) List(ctx context.Context, filter SkillFilter, limit, offset int) ([]model.Skill, model.SkillHubSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	filter.Query = strings.TrimSpace(filter.Query)
	filter.Category = strings.TrimSpace(filter.Category)
	filter.Enabled = strings.TrimSpace(filter.Enabled)

	args := []any{}
	whereParts := []string{}
	if filter.Query != "" {
		args = append(args, "%"+filter.Query+"%")
		whereParts = append(whereParts, fmt.Sprintf(`(
			name ILIKE $%d
			OR slug ILIKE $%d
			OR category ILIKE $%d
			OR description ILIKE $%d
			OR tags ILIKE $%d
		)`, len(args), len(args), len(args), len(args), len(args)))
	}
	if filter.Category != "" {
		args = append(args, filter.Category)
		whereParts = append(whereParts, fmt.Sprintf("category = $%d", len(args)))
	}
	if filter.Enabled == "enabled" || filter.Enabled == "disabled" {
		args = append(args, filter.Enabled == "enabled")
		whereParts = append(whereParts, fmt.Sprintf("enabled = $%d", len(args)))
	}

	where := ""
	if len(whereParts) > 0 {
		where = "WHERE " + strings.Join(whereParts, " AND ")
	}

	var summary model.SkillHubSummary
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COUNT(*) FILTER (WHERE enabled)
		FROM ai_skills
		%s
	`, where), args...).Scan(&summary.Total, &summary.Enabled); err != nil {
		return nil, model.SkillHubSummary{}, fmt.Errorf("count skills: %w", err)
	}

	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, name, slug, category, description, instruction, tags, enabled, created_at, updated_at
		FROM ai_skills
		%s
		ORDER BY enabled DESC, updated_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, where, limitArg, offsetArg), args...)
	if err != nil {
		return nil, model.SkillHubSummary{}, fmt.Errorf("query skills: %w", err)
	}
	defer rows.Close()

	skills := make([]model.Skill, 0, limit)
	for rows.Next() {
		var skill model.Skill
		if err := rows.Scan(
			&skill.ID,
			&skill.Name,
			&skill.Slug,
			&skill.Category,
			&skill.Description,
			&skill.Instruction,
			&skill.Tags,
			&skill.Enabled,
			&skill.CreatedAt,
			&skill.UpdatedAt,
		); err != nil {
			return nil, model.SkillHubSummary{}, fmt.Errorf("scan skill: %w", err)
		}
		skills = append(skills, skill)
	}
	if err := rows.Err(); err != nil {
		return nil, model.SkillHubSummary{}, fmt.Errorf("iterate skills: %w", err)
	}

	return skills, summary, nil
}

func (r SkillRepository) Categories(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT category
		FROM ai_skills
		WHERE btrim(category) <> ''
		ORDER BY category
	`)
	if err != nil {
		return nil, fmt.Errorf("query skill categories: %w", err)
	}
	defer rows.Close()

	categories := []string{}
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			return nil, fmt.Errorf("scan skill category: %w", err)
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate skill categories: %w", err)
	}

	return categories, nil
}

func (r SkillRepository) Enabled(ctx context.Context) ([]model.Skill, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, slug, category, description, instruction, tags, enabled, created_at, updated_at
		FROM ai_skills
		WHERE enabled = TRUE
		ORDER BY category, name
	`)
	if err != nil {
		return nil, fmt.Errorf("query enabled skills: %w", err)
	}
	defer rows.Close()

	skills := []model.Skill{}
	for rows.Next() {
		var skill model.Skill
		if err := rows.Scan(
			&skill.ID,
			&skill.Name,
			&skill.Slug,
			&skill.Category,
			&skill.Description,
			&skill.Instruction,
			&skill.Tags,
			&skill.Enabled,
			&skill.CreatedAt,
			&skill.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan enabled skill: %w", err)
		}
		skills = append(skills, skill)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enabled skills: %w", err)
	}

	return skills, nil
}

func (r SkillRepository) Create(ctx context.Context, skill model.Skill) (model.Skill, error) {
	if err := r.db.QueryRow(ctx, `
		INSERT INTO ai_skills (name, slug, category, description, instruction, tags, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`, skill.Name, skill.Slug, skill.Category, skill.Description, skill.Instruction, skill.Tags, skill.Enabled).Scan(
		&skill.ID,
		&skill.CreatedAt,
		&skill.UpdatedAt,
	); err != nil {
		return model.Skill{}, fmt.Errorf("create skill: %w", err)
	}
	return skill, nil
}

func (r SkillRepository) Update(ctx context.Context, skill model.Skill) (model.Skill, error) {
	if err := r.db.QueryRow(ctx, `
		UPDATE ai_skills
		SET name = $2,
			slug = $3,
			category = $4,
			description = $5,
			instruction = $6,
			tags = $7,
			enabled = $8
		WHERE id = $1
		RETURNING created_at, updated_at
	`, skill.ID, skill.Name, skill.Slug, skill.Category, skill.Description, skill.Instruction, skill.Tags, skill.Enabled).Scan(
		&skill.CreatedAt,
		&skill.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Skill{}, fmt.Errorf("skill %d not found: %w", skill.ID, err)
		}
		return model.Skill{}, fmt.Errorf("update skill: %w", err)
	}
	return skill, nil
}

func (r SkillRepository) SetEnabled(ctx context.Context, id int64, enabled bool) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE ai_skills
		SET enabled = $2
		WHERE id = $1
	`, id, enabled)
	if err != nil {
		return fmt.Errorf("set skill enabled: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("skill %d not found: %w", id, pgx.ErrNoRows)
	}
	return nil
}

func (r SkillRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM ai_skills WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete skill: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("skill %d not found: %w", id, pgx.ErrNoRows)
	}
	return nil
}
