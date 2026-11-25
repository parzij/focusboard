-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (char_length(regexp_replace(name, '^[[:space:]]+|[[:space:]]+$', '', 'g')) BETWEEN 1 AND 80),
    email TEXT NOT NULL UNIQUE CHECK (
        char_length(email) BETWEEN 1 AND 254 AND email = lower(email) AND email !~ '[[:space:]]'
    ),
    password_hash TEXT NOT NULL CHECK (password_hash ~ '^[$]argon2id[$]v=[0-9]+[$]m=[0-9]+,t=[0-9]+,p=[0-9]+[$][A-Za-z0-9+/]+[$][A-Za-z0-9+/]+$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
