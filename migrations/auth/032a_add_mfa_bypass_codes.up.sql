ALTER TABLE user_mfa ADD COLUMN bypass_codes TEXT[] NOT NULL DEFAULT '{}';
