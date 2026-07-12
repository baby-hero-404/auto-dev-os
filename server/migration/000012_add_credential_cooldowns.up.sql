CREATE TABLE IF NOT EXISTS credential_cooldowns (
    credential_id UUID NOT NULL,
    model TEXT NOT NULL,
    cooldown_until TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (credential_id, model)
);
