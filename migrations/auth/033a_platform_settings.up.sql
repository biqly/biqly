CREATE TABLE platform_settings (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    self_signup_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL
);

INSERT INTO platform_settings (id, self_signup_enabled) VALUES (1, TRUE);
