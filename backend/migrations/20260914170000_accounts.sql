-- atlas:txmode file

CREATE TABLE users (
    id uuid PRIMARY KEY,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT users_display_name_check CHECK (
        display_name = btrim(display_name)
        AND char_length(display_name) BETWEEN 1 AND 80
    ),
    CONSTRAINT users_timestamps_check CHECK (updated_at >= created_at)
);

CREATE TABLE user_credentials (
    user_id uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    email text NOT NULL,
    password_hash text NOT NULL,
    CONSTRAINT user_credentials_email_unique UNIQUE (email),
    CONSTRAINT user_credentials_email_check CHECK (
        email = lower(btrim(email))
        AND char_length(email) BETWEEN 3 AND 254
    ),
    CONSTRAINT user_credentials_password_hash_check CHECK (
        char_length(password_hash) BETWEEN 59 AND 72
        AND password_hash LIKE '$2%'
    )
);

CREATE TABLE user_sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT user_sessions_token_hash_unique UNIQUE (token_hash),
    CONSTRAINT user_sessions_token_hash_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT user_sessions_expiry_check CHECK (expires_at > created_at)
);

CREATE INDEX user_sessions_user_id_idx ON user_sessions (user_id);
CREATE INDEX user_sessions_expires_at_idx ON user_sessions (expires_at);
