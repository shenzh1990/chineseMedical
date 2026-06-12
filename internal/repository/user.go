package repository

import (
	"context"
	"errors"
	"fmt"

	"chinese-medical/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return UserRepository{db: db}
}

func (r UserRepository) GetByUsername(ctx context.Context, username string) (model.User, error) {
	var user model.User
	if err := r.db.QueryRow(ctx, `
		SELECT id, username, password_hash, role, created_at, updated_at
		FROM app_users
		WHERE username = $1
	`, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, fmt.Errorf("user %q not found: %w", username, err)
		}
		return model.User{}, fmt.Errorf("get user by username: %w", err)
	}
	return user, nil
}

func (r UserRepository) UpdatePasswordHash(ctx context.Context, username string, passwordHash string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE app_users
		SET password_hash = $2, updated_at = now()
		WHERE username = $1
	`, username, passwordHash)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user %q not found", username)
	}
	return nil
}
