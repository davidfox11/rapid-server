#!/bin/bash
set -e

DB_URL="${DATABASE_URL:-postgres://rapid:rapid@localhost:5432/rapid?sslmode=disable}"

echo "Seeding categories..."
psql "$DB_URL" -f seeds/001_categories.sql
echo "Seeding questions..."
psql "$DB_URL" -f seeds/002_questions.sql
echo "Done."
