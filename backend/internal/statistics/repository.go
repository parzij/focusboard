package statistics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const countSQL = `
WITH first_done AS MATERIALIZED (
    SELECT first_event.changed_at
    FROM tasks AS t
    CROSS JOIN LATERAL (
        SELECT h.changed_at
        FROM task_history AS h
        WHERE h.task_id = t.id AND h.new_status = 'done'
        ORDER BY h.changed_at
        LIMIT 1
    ) AS first_event
    WHERE t.user_id = $1
      AND first_event.changed_at <= $2
      AND first_event.changed_at >= (SELECT min(start_at) FROM unnest($4::timestamptz[]) AS u(start_at))
)
SELECT w.id, count(*) AS total
FROM unnest($3::integer[], $4::timestamptz[], $5::timestamptz[]) AS w(id, start_at, end_at)
JOIN first_done AS f ON f.changed_at >= w.start_at AND f.changed_at < w.end_at
GROUP BY w.id
`

func (r *Repository) count(ctx context.Context, owner string, now time.Time, windows []window) (map[int]int64, error) {
	ids := make([]int32, len(windows))
	starts, ends := make([]time.Time, len(windows)), make([]time.Time, len(windows))
	for i, w := range windows {
		ids[i], starts[i], ends[i] = int32(w.id), w.start, w.end
	}
	rows, err := r.pool.Query(ctx, countSQL, owner, now, ids, starts, ends)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[int]int64, len(windows))
	for rows.Next() {
		var id int
		var total int64
		if err := rows.Scan(&id, &total); err != nil {
			return nil, err
		}
		counts[id] = total
	}
	return counts, rows.Err()
}
