package tasks

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

func (s *Service) Board(ctx context.Context, owner string) ([]Task, error) {
	return board(ctx, s.repo.pool, owner)
}

func (s *Service) Find(ctx context.Context, owner, id string) (Task, error) {
	return find(ctx, s.repo.pool, owner, id)
}

func mutate[T any](ctx context.Context, repo *Repository, owner string, fn func(pgx.Tx) (T, error)) (T, error) {
	var zero T
	tx, err := repo.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return zero, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()

	var result T
	err = lockOwner(ctx, tx, owner)
	if err == nil {
		result, err = fn(tx)
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "tasks_owner_status_position_key" {
		return zero, ErrOrderConflict
	}
	if err != nil {
		// A canceled statement can surface as a driver error.
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}
		return zero, err
	}
	return result, nil
}

func (s *Service) Create(ctx context.Context, owner string, changes Changes) (Task, error) {
	if changes.Title == nil {
		return Task{}, InvalidFields{"title": "Title is required."}
	}
	date, err := changes.validate()
	if err != nil {
		return Task{}, err
	}
	task := Task{Title: *changes.Title, DueDate: date, Status: NotStarted}
	if changes.Description != nil {
		task.Description = *changes.Description
	}
	if changes.Status != nil {
		task.Status = *changes.Status
	}
	return mutate(ctx, s.repo, owner, func(tx pgx.Tx) (Task, error) {
		size, err := columnSize(ctx, tx, owner, task.Status)
		if err != nil {
			return Task{}, err
		}
		task.Position = size
		created, err := insert(ctx, tx, owner, task)
		if err != nil {
			return Task{}, err
		}
		return created, recordStatus(ctx, tx, owner, created.ID, nil, created.Status)
	})
}

func (s *Service) Update(ctx context.Context, owner, id string, changes Changes) (Task, error) {
	if changes.Title == nil && changes.Description == nil && changes.Status == nil && changes.DueDate == nil && !changes.ClearDate {
		return Task{}, InvalidFields{"task": "Provide at least one field."}
	}
	date, err := changes.validate()
	if err != nil {
		return Task{}, err
	}
	return mutate(ctx, s.repo, owner, func(tx pgx.Tx) (Task, error) {
		task, err := find(ctx, tx, owner, id)
		if err != nil {
			return Task{}, err
		}
		old := task.Status
		if changes.Status != nil && *changes.Status != old {
			end, err := columnSize(ctx, tx, owner, *changes.Status)
			if err != nil {
				return Task{}, err
			}
			if err := shift(ctx, tx, owner, old, task.Position+1, maxPosition, -1); err != nil {
				return Task{}, err
			}
			task.Status, task.Position = *changes.Status, end
		}
		if changes.Title != nil {
			task.Title = *changes.Title
		}
		if changes.Description != nil {
			task.Description = *changes.Description
		}
		if changes.DueDate != nil || changes.ClearDate {
			task.DueDate = date
		}
		return saveTransition(ctx, tx, owner, task, old)
	})
}

const maxPosition = 1<<31 - 1

type positionShift struct {
	first, last, delta int
}

func moveShifts(sameColumn bool, current, target, targetSize int) (source, destination positionShift, err error) {
	limit := targetSize
	if sameColumn {
		limit--
	}
	if target < 0 || target > limit {
		return source, destination, InvalidFields{"position": "Position is outside the destination column."}
	}
	switch {
	case !sameColumn:
		source = positionShift{current + 1, maxPosition, -1}
		destination = positionShift{target, targetSize - 1, 1}
	case target < current:
		source = positionShift{target, current - 1, 1}
	case target > current:
		source = positionShift{current + 1, target, -1}
	}
	return source, destination, nil
}

func (s *Service) Move(ctx context.Context, owner, id string, next Status, position *int64) ([]Task, error) {
	if err := validateMove(next, position); err != nil {
		return nil, err
	}
	target := int(*position)
	return mutate(ctx, s.repo, owner, func(tx pgx.Tx) ([]Task, error) {
		task, err := find(ctx, tx, owner, id)
		if err != nil {
			return nil, err
		}
		old := task.Status
		sourceSize, err := columnSize(ctx, tx, owner, old)
		if err != nil {
			return nil, err
		}
		targetSize := sourceSize
		if old != next {
			targetSize, err = columnSize(ctx, tx, owner, next)
		}
		if err != nil {
			return nil, err
		}
		source, destination, err := moveShifts(old == next, task.Position, target, targetSize)
		if err != nil {
			return nil, err
		}
		if source.delta != 0 {
			if err := shift(ctx, tx, owner, old, source.first, source.last, source.delta); err != nil {
				return nil, err
			}
		}
		if destination.delta != 0 {
			if err := shift(ctx, tx, owner, next, destination.first, destination.last, destination.delta); err != nil {
				return nil, err
			}
		}
		if old != next || task.Position != target {
			task.Status, task.Position = next, target
			if _, err := saveTransition(ctx, tx, owner, task, old); err != nil {
				return nil, err
			}
		}
		return board(ctx, tx, owner)
	})
}

func (s *Service) Delete(ctx context.Context, owner, id string) error {
	_, err := mutate(ctx, s.repo, owner, func(tx pgx.Tx) (struct{}, error) {
		var result struct{}
		task, err := find(ctx, tx, owner, id)
		if err != nil {
			return result, err
		}
		if err := softDelete(ctx, tx, owner, id); err != nil {
			return result, err
		}
		return result, shift(ctx, tx, owner, task.Status, task.Position+1, maxPosition, -1)
	})
	return err
}

func saveTransition(ctx context.Context, tx pgx.Tx, owner string, task Task, previous Status) (Task, error) {
	saved, err := save(ctx, tx, owner, task)
	if err == nil && previous != task.Status {
		err = recordStatus(ctx, tx, owner, task.ID, &previous, task.Status)
	}
	return saved, err
}
