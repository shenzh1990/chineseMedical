package model

import "strings"

const DefaultFoodCategory = "药食同源"

var foodCategoryOptions = []string{
	DefaultFoodCategory,
	"经典方",
	"名医方",
}

type MedicatedFood struct {
	ID       int64
	Category string
	Name     string
	Source   string
	Food     string
	Method   string
	Effect   string
}

type FoodCategorySummary struct {
	Name  string
	Count int64
}

func FoodCategoryOptions() []string {
	options := make([]string, len(foodCategoryOptions))
	copy(options, foodCategoryOptions)
	return options
}

func NormalizeFoodCategory(category string) string {
	category = strings.TrimSpace(category)
	if category == "" {
		return DefaultFoodCategory
	}
	return category
}
