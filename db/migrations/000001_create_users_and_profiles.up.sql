CREATE TYPE user_role AS ENUM ('admin', 'member');

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    public_id VARCHAR(16) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    role user_role NOT NULL DEFAULT 'member',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    deleted_at BIGINT
);

CREATE TABLE user_profiles (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    full_name VARCHAR(255),
    nik VARCHAR(32),
    birth_date DATE,
    gender VARCHAR(50),
    district VARCHAR(128),
    phone VARCHAR(32),
    address TEXT,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    deleted_at BIGINT
);

CREATE INDEX idx_users_deleted_at ON users (deleted_at);

CREATE INDEX idx_user_profiles_district ON user_profiles (district);

CREATE INDEX idx_user_profiles_gender ON user_profiles (gender);
