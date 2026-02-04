CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    api_key_hash VARCHAR(64) NOT NULL UNIQUE,
    api_key_encrypted BYTEA,
    budget_usd DECIMAL(10, 2) DEFAULT 0,
    rate_limit_rpm INTEGER DEFAULT 60,
    allowed_models TEXT[] DEFAULT '{}',
    default_provider VARCHAR(50),
    fallback_providers TEXT[] DEFAULT '{}',
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_tenants_api_key_hash ON tenants(api_key_hash);
CREATE INDEX idx_tenants_enabled ON tenants(enabled);

CREATE TABLE IF NOT EXISTS usage_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    request_id VARCHAR(255) NOT NULL,
    model VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost_usd DECIMAL(10, 6) DEFAULT 0,
    cached BOOLEAN DEFAULT false,
    latency_ms INTEGER DEFAULT 0,
    status VARCHAR(20) DEFAULT 'success',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_usage_records_tenant_id ON usage_records(tenant_id);
CREATE INDEX idx_usage_records_created_at ON usage_records(created_at);
CREATE INDEX idx_usage_records_tenant_created ON usage_records(tenant_id, created_at);

CREATE TABLE IF NOT EXISTS admin_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'viewer',
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_admin_users_username ON admin_users(username);

COMMENT ON COLUMN tenants.api_key_hash IS 'SHA-256 hash of the API key for lookup';
COMMENT ON COLUMN tenants.api_key_encrypted IS 'AES-256 encrypted API key for rotation/display';
COMMENT ON COLUMN admin_users.role IS 'admin, editor, or viewer';
