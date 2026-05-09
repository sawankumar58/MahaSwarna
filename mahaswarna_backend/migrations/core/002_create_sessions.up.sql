-- sessions: stores refresh token JTIs for revocation
-- Access tokens are stateless (RS256 JWT); only refresh tokens are tracked here.
CREATE TABLE IF NOT EXISTS sessions (
  jti        UUID        PRIMARY KEY DEFAULT gen_random_uuid(),  -- = JWT jti claim
  user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  revoked    BOOLEAN     NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL                                -- NOW() + 30 days
);

CREATE INDEX idx_sessions_user_id   ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- Partial index for active (non-revoked, non-expired) sessions
CREATE INDEX idx_sessions_active ON sessions(jti)
  WHERE revoked = FALSE AND expires_at > NOW();
