-- +goose Up
CREATE TABLE task_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    old_status TEXT CHECK (old_status IN ('not_started', 'in_progress', 'finish', 'done')),
    new_status TEXT NOT NULL CHECK (new_status IN ('not_started', 'in_progress', 'finish', 'done')),
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (old_status IS NULL OR old_status <> new_status)
);

CREATE INDEX task_history_task_time_idx ON task_history (task_id, changed_at);
CREATE INDEX task_history_done_idx ON task_history (task_id, changed_at) WHERE new_status = 'done';
CREATE UNIQUE INDEX task_history_initial_idx ON task_history (task_id) WHERE old_status IS NULL;

-- +goose Down
DROP TABLE task_history;
