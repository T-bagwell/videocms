package db

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func SeedAdmin(ctx context.Context, pool *pgxpool.Pool, username, password string) error {
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	user := username
	if user == "" {
		user = "admin"
	}
	pass := password
	if pass == "" {
		pass = "admin123"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (username, password_hash, role, display_name) VALUES ($1, $2, 'admin', $3)`,
		user, string(hash), user); err != nil {
		return err
	}
	log.Printf("created initial admin user %q — change its password after first login", user)
	return nil
}
