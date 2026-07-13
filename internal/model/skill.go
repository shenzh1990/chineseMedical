package model

import "time"

type Skill struct {
	ID          int64
	Name        string
	Slug        string
	Category    string
	Description string
	Instruction string
	Tags        string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SkillHubSummary struct {
	Total   int64
	Enabled int64
}
