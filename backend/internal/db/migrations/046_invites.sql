-- Registration policies: one-time invite codes for invite-only registration.
CREATE TABLE IF NOT EXISTS invite_codes (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code       text NOT NULL UNIQUE,
    used_by    uuid REFERENCES users(id) ON DELETE SET NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
