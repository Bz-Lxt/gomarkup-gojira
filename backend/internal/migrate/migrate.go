package migrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func Up(databaseURL, migrationsPath string) error {
	path, err := resolve(migrationsPath)
	if err != nil {
		return err
	}
	src := "file://" + path
	m, err := migrate.New(src, databaseURL)
	if err != nil {
		return fmt.Errorf("migrate open: %w", err)
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func resolve(p string) (string, error) {
	candidates := []string{p, "migrations", "/app/migrations", filepath.Join("..", "migrations")}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if st, err := os.Stat(abs); err == nil && st.IsDir() {
			return filepath.ToSlash(abs), nil
		}
	}
	return "", fmt.Errorf("migrations directory not found (tried %q)", p)
}
