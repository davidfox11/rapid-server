package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://rapid:rapid@localhost:5432/rapid?sslmode=disable"
	}

	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer conn.Close(ctx)

	files, err := filepath.Glob(fmt.Sprintf("migrations/*.%s.sql", direction))
	if err != nil {
		log.Fatalf("find migration files: %v", err)
	}
	sort.Strings(files)

	if len(files) == 0 {
		log.Printf("no %s migration files found", direction)
		return
	}

	for _, f := range files {
		name := filepath.Base(f)
		log.Printf("running migration: %s", name)

		sql, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("read %s: %v", name, err)
		}

		for _, stmt := range splitStatements(string(sql)) {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := conn.Exec(ctx, stmt); err != nil {
				log.Fatalf("execute %s: %v", name, err)
			}
		}

		log.Printf("completed: %s", name)
	}
}

func splitStatements(sql string) []string {
	return strings.Split(sql, ";")
}
