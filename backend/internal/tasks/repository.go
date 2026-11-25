package tasks

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

type reader interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

const columns = "id::text, title, description, status, position, due_date, created_at, updated_at"

func scanTask(row pgx.Row) (Task, error) {
	var task Task
	err := row.Scan(&task.ID, &task.Title, &task.Description, &task.Status, &task.Position,
		&task.DueDate, &task.CreatedAt, &task.UpdatedAt)
	return task, err
}

func board(ctx context.Context, db reader, owner string) ([]Task, error) {
	rows, err := db.Query(ctx, "SELECT "+columns+` FROM tasks WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY array_position(ARRAY[$2, $3, $4, $5]::text[], status), position, id`,
		owner, NotStarted, InProgress, Finish, Done)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Task, error) { return scanTask(row) })
}

func find(ctx context.Context, db reader, owner, id string) (Task, error) {
	task, err := scanTask(db.QueryRow(ctx, "SELECT "+columns+
		" FROM tasks WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL", owner, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return task, ErrNotFound
	}
	return task, err
}

func lockOwner(ctx context.Context, tx pgx.Tx, owner string) error {
	var id string
	err := tx.QueryRow(ctx, "SELECT id::text FROM users WHERE id = $1 FOR UPDATE", owner).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrOwnerNotFound
	}
	return err
}

func columnSize(ctx context.Context, tx pgx.Tx, owner string, status Status) (int, error) {
	var size, last int
	err := tx.QueryRow(ctx, `SELECT count(*), coalesce(max(position), -1) FROM tasks
		WHERE user_id = $1 AND status = $2 AND deleted_at IS NULL`, owner, status).Scan(&size, &last)
	// With unique non-negative positions, count and max establish density.
	if err == nil && last != size-1 {
		return 0, ErrOrderConflict
	}
	return size, err
}

func insert(ctx context.Context, tx pgx.Tx, owner string, task Task) (Task, error) {
	return scanTask(tx.QueryRow(ctx, `INSERT INTO tasks (user_id, title, description, status, position, due_date,
		created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, statement_timestamp(), statement_timestamp())
		RETURNING `+columns, owner, task.Title, task.Description, task.Status, task.Position, task.DueDate))
}

func save(ctx context.Context, tx pgx.Tx, owner string, task Task) (Task, error) {
	return scanTask(tx.QueryRow(ctx, `UPDATE tasks SET title = $3, description = $4, status = $5,
		position = $6, due_date = $7, updated_at = clock_timestamp()
		WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL RETURNING `+columns,
		owner, task.ID, task.Title, task.Description, task.Status, task.Position, task.DueDate))
}

func shift(ctx context.Context, tx pgx.Tx, owner string, status Status, first, last, delta int) error {
	if first > last {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE tasks SET position = position + $5, updated_at = clock_timestamp()
		WHERE user_id = $1 AND status = $2 AND deleted_at IS NULL AND position BETWEEN $3 AND $4`,
		owner, status, first, last, delta)
	return err
}

func softDelete(ctx context.Context, tx pgx.Tx, owner, id string) error {
	_, err := tx.Exec(ctx, `UPDATE tasks SET deleted_at = statement_timestamp(), position = NULL,
		updated_at = statement_timestamp() WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL`, owner, id)
	return err
}

func recordStatus(ctx context.Context, tx pgx.Tx, owner, id string, old *Status, next Status) error {
	result, err := tx.Exec(ctx, `INSERT INTO task_history (task_id, old_status, new_status, changed_at)
		SELECT id, $3, $4, clock_timestamp() FROM tasks WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL`,
		owner, id, old, next)
	if err == nil && result.RowsAffected() != 1 {
		return ErrNotFound
	}
	return err
}
