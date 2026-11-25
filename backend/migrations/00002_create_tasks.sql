-- +goose Up
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (char_length(regexp_replace(title, '^[[:space:]]+|[[:space:]]+$', '', 'g')) BETWEEN 1 AND 120),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000),
    status TEXT NOT NULL DEFAULT 'not_started' CHECK (status IN ('not_started', 'in_progress', 'finish', 'done')),
    position INTEGER,
    due_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT tasks_position_check CHECK (
        (deleted_at IS NULL AND position IS NOT NULL AND position >= 0)
        OR (deleted_at IS NOT NULL AND position IS NULL)
    ),
    CONSTRAINT tasks_owner_status_position_key
        UNIQUE (user_id, status, position) DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX tasks_active_board_idx ON tasks (user_id, status, position) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE tasks;
