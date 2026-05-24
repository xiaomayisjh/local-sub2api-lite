package migrations

const SQLiteInitSQL = `
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;
PRAGMA busy_timeout=5000;

-- ============================================================
-- proxies
-- ============================================================
CREATE TABLE IF NOT EXISTS proxies (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            VARCHAR(100) NOT NULL,
    protocol        VARCHAR(20) NOT NULL,
    host            VARCHAR(255) NOT NULL,
    port            INT NOT NULL,
    username        VARCHAR(100),
    password        VARCHAR(100),
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at      DATETIME
);

CREATE INDEX IF NOT EXISTS idx_proxies_status ON proxies(status);
CREATE INDEX IF NOT EXISTS idx_proxies_deleted_at ON proxies(deleted_at);

-- ============================================================
-- groups
-- ============================================================
CREATE TABLE IF NOT EXISTS groups (
    id                                  INTEGER PRIMARY KEY AUTOINCREMENT,
    name                                VARCHAR(100) NOT NULL,
    description                         TEXT,
    rate_multiplier                     DECIMAL(10, 4) NOT NULL DEFAULT 1.0,
    is_exclusive                        BOOLEAN NOT NULL DEFAULT FALSE,
    status                              VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at                          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at                          DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at                          DATETIME,
    platform                            VARCHAR(50) NOT NULL DEFAULT 'anthropic',
    subscription_type                   VARCHAR(20) NOT NULL DEFAULT 'standard',
    daily_limit_usd                     DECIMAL(20, 8) DEFAULT NULL,
    weekly_limit_usd                    DECIMAL(20, 8) DEFAULT NULL,
    monthly_limit_usd                   DECIMAL(20, 8) DEFAULT NULL,
    default_validity_days               INT NOT NULL DEFAULT 30,
    model_routing                       TEXT DEFAULT '{}',
    model_routing_enabled               BOOLEAN NOT NULL DEFAULT FALSE,
    image_price_1k                      DECIMAL(20, 8),
    image_price_2k                      DECIMAL(20, 8),
    image_price_4k                      DECIMAL(20, 8),
    claude_code_only                    BOOLEAN NOT NULL DEFAULT FALSE,
    fallback_group_id                   BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    sort_order                          INT NOT NULL DEFAULT 0,
    allow_messages_dispatch             BOOLEAN NOT NULL DEFAULT FALSE,
    default_mapped_model                VARCHAR(100) NOT NULL DEFAULT '',
    require_oauth_only                  BOOLEAN NOT NULL DEFAULT FALSE,
    require_privacy_set                 BOOLEAN NOT NULL DEFAULT FALSE,
    messages_dispatch_model_config      TEXT NOT NULL DEFAULT '{}',
    rpm_limit                           INT NOT NULL DEFAULT 0,
    allow_image_generation              BOOLEAN NOT NULL DEFAULT FALSE,
    image_rate_independent              BOOLEAN NOT NULL DEFAULT FALSE,
    image_rate_multiplier               DECIMAL(10, 4) NOT NULL DEFAULT 1.0,
    fallback_group_id_on_invalid_request BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    mcp_xml_inject                      BOOLEAN NOT NULL DEFAULT TRUE,
    supported_model_scopes              TEXT NOT NULL DEFAULT '["claude", "gemini_text", "gemini_image"]'
);

CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);
CREATE INDEX IF NOT EXISTS idx_groups_status ON groups(status);
CREATE INDEX IF NOT EXISTS idx_groups_is_exclusive ON groups(is_exclusive);
CREATE INDEX IF NOT EXISTS idx_groups_deleted_at ON groups(deleted_at);
CREATE INDEX IF NOT EXISTS idx_groups_platform ON groups(platform);
CREATE INDEX IF NOT EXISTS idx_groups_subscription_type ON groups(subscription_type);
CREATE INDEX IF NOT EXISTS idx_groups_sort_order ON groups(sort_order);
CREATE INDEX IF NOT EXISTS idx_groups_claude_code_only ON groups(claude_code_only);
CREATE INDEX IF NOT EXISTS idx_groups_fallback_group_id ON groups(fallback_group_id);
CREATE INDEX IF NOT EXISTS idx_groups_fallback_group_id_on_invalid_request ON groups(fallback_group_id_on_invalid_request);

-- ============================================================
-- users
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id                              INTEGER PRIMARY KEY AUTOINCREMENT,
    email                           VARCHAR(255) NOT NULL,
    password_hash                   VARCHAR(255) NOT NULL,
    role                            VARCHAR(20) NOT NULL DEFAULT 'user',
    balance                         DECIMAL(20, 8) NOT NULL DEFAULT 0,
    concurrency                     INT NOT NULL DEFAULT 5,
    status                          VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at                      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at                      DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at                      DATETIME,
    username                        VARCHAR(100) NOT NULL DEFAULT '',
    notes                           TEXT NOT NULL DEFAULT '',
    totp_secret_encrypted           TEXT DEFAULT NULL,
    totp_enabled                    BOOLEAN NOT NULL DEFAULT FALSE,
    totp_enabled_at                 DATETIME DEFAULT NULL,
    balance_notify_enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    balance_notify_threshold        DECIMAL(20, 8) DEFAULT NULL,
    balance_notify_extra_emails     TEXT NOT NULL DEFAULT '[]',
    balance_notify_threshold_type   VARCHAR(10) NOT NULL DEFAULT 'fixed',
    total_recharged                 DECIMAL(20, 8) NOT NULL DEFAULT 0,
    signup_source                   VARCHAR(20) NOT NULL DEFAULT 'email',
    last_login_at                   DATETIME,
    last_active_at                  DATETIME,
    rpm_limit                       INT NOT NULL DEFAULT 0,
    CONSTRAINT users_signup_source_check CHECK (signup_source IN ('email', 'linuxdo', 'wechat', 'oidc', 'github', 'google', 'dingtalk'))
);

CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique_active ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
CREATE INDEX IF NOT EXISTS idx_users_totp_enabled ON users(totp_enabled);

-- ============================================================
-- accounts
-- ============================================================
CREATE TABLE IF NOT EXISTS accounts (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    name                    VARCHAR(100) NOT NULL,
    platform                VARCHAR(50) NOT NULL,
    type                    VARCHAR(20) NOT NULL,
    credentials             TEXT NOT NULL DEFAULT '{}',
    extra                   TEXT NOT NULL DEFAULT '{}',
    proxy_id                BIGINT REFERENCES proxies(id) ON DELETE SET NULL,
    concurrency             INT NOT NULL DEFAULT 3,
    priority                INT NOT NULL DEFAULT 50,
    status                  VARCHAR(20) NOT NULL DEFAULT 'active',
    error_message           TEXT,
    last_used_at            DATETIME,
    created_at              DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at              DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at              DATETIME,
    schedulable             BOOLEAN NOT NULL DEFAULT TRUE,
    rate_limited_at         DATETIME,
    rate_limit_reset_at     DATETIME,
    overload_until          DATETIME,
    session_window_start    DATETIME,
    session_window_end      DATETIME,
    session_window_status   VARCHAR(20),
    temp_unschedulable_until DATETIME,
    temp_unschedulable_reason TEXT,
    notes                   TEXT,
    expires_at              DATETIME,
    auto_pause_on_expired   BOOLEAN NOT NULL DEFAULT TRUE,
    load_factor             INTEGER
);

CREATE INDEX IF NOT EXISTS idx_accounts_platform ON accounts(platform);
CREATE INDEX IF NOT EXISTS idx_accounts_type ON accounts(type);
CREATE INDEX IF NOT EXISTS idx_accounts_status ON accounts(status);
CREATE INDEX IF NOT EXISTS idx_accounts_proxy_id ON accounts(proxy_id);
CREATE INDEX IF NOT EXISTS idx_accounts_priority ON accounts(priority);
CREATE INDEX IF NOT EXISTS idx_accounts_last_used_at ON accounts(last_used_at);
CREATE INDEX IF NOT EXISTS idx_accounts_deleted_at ON accounts(deleted_at);
CREATE INDEX IF NOT EXISTS idx_accounts_schedulable ON accounts(schedulable);
CREATE INDEX IF NOT EXISTS idx_accounts_rate_limited_at ON accounts(rate_limited_at);
CREATE INDEX IF NOT EXISTS idx_accounts_rate_limit_reset_at ON accounts(rate_limit_reset_at);
CREATE INDEX IF NOT EXISTS idx_accounts_overload_until ON accounts(overload_until);
CREATE INDEX IF NOT EXISTS idx_accounts_temp_unschedulable_until ON accounts(temp_unschedulable_until);

-- ============================================================
-- api_keys
-- ============================================================
CREATE TABLE IF NOT EXISTS api_keys (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id             BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key                 VARCHAR(128) NOT NULL UNIQUE,
    name                VARCHAR(100) NOT NULL,
    group_id            BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    status              VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    deleted_at          DATETIME,
    quota               DECIMAL(20, 8) NOT NULL DEFAULT 0,
    quota_used          DECIMAL(20, 8) NOT NULL DEFAULT 0,
    expires_at          DATETIME,
    last_used_at        DATETIME,
    ip_whitelist        TEXT DEFAULT NULL,
    ip_blacklist        TEXT DEFAULT NULL,
    rate_limit_5h       DECIMAL(20, 8) NOT NULL DEFAULT 0,
    rate_limit_1d       DECIMAL(20, 8) NOT NULL DEFAULT 0,
    rate_limit_7d       DECIMAL(20, 8) NOT NULL DEFAULT 0,
    usage_5h            DECIMAL(20, 8) NOT NULL DEFAULT 0,
    usage_1d            DECIMAL(20, 8) NOT NULL DEFAULT 0,
    usage_7d            DECIMAL(20, 8) NOT NULL DEFAULT 0,
    window_5h_start     DATETIME,
    window_1d_start     DATETIME,
    window_7d_start     DATETIME
);

CREATE INDEX IF NOT EXISTS idx_api_keys_key ON api_keys(key);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_group_id ON api_keys(group_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);
CREATE INDEX IF NOT EXISTS idx_api_keys_deleted_at ON api_keys(deleted_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_last_used_at ON api_keys(last_used_at);

-- ============================================================
-- account_groups
-- ============================================================
CREATE TABLE IF NOT EXISTS account_groups (
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    group_id        BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    priority        INT NOT NULL DEFAULT 50,
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (account_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_account_groups_group_id ON account_groups(group_id);
CREATE INDEX IF NOT EXISTS idx_account_groups_priority ON account_groups(priority);

-- ============================================================
-- usage_logs
-- ============================================================
CREATE TABLE IF NOT EXISTS usage_logs (
    id                          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id                     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id                  BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    account_id                  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    request_id                  VARCHAR(64),
    model                       VARCHAR(100) NOT NULL,
    input_tokens                INT NOT NULL DEFAULT 0,
    output_tokens               INT NOT NULL DEFAULT 0,
    cache_creation_tokens       INT NOT NULL DEFAULT 0,
    cache_read_tokens           INT NOT NULL DEFAULT 0,
    cache_creation_5m_tokens    INT NOT NULL DEFAULT 0,
    cache_creation_1h_tokens    INT NOT NULL DEFAULT 0,
    input_cost                  DECIMAL(20, 10) NOT NULL DEFAULT 0,
    output_cost                 DECIMAL(20, 10) NOT NULL DEFAULT 0,
    cache_creation_cost         DECIMAL(20, 10) NOT NULL DEFAULT 0,
    cache_read_cost             DECIMAL(20, 10) NOT NULL DEFAULT 0,
    total_cost                  DECIMAL(20, 10) NOT NULL DEFAULT 0,
    actual_cost                 DECIMAL(20, 10) NOT NULL DEFAULT 0,
    stream                      BOOLEAN NOT NULL DEFAULT FALSE,
    duration_ms                 INT,
    created_at                  DATETIME NOT NULL DEFAULT (datetime('now')),
    billing_type                SMALLINT NOT NULL DEFAULT 0,
    user_agent                  VARCHAR(512),
    image_count                 INT DEFAULT 0,
    image_size                  VARCHAR(10),
    ip_address                  VARCHAR(45),
    cache_ttl_overridden        BOOLEAN NOT NULL DEFAULT FALSE,
    openai_ws_mode              BOOLEAN NOT NULL DEFAULT FALSE,
    request_type                SMALLINT NOT NULL DEFAULT 0,
    reasoning_effort            VARCHAR(20),
    service_tier                VARCHAR(16),
    inbound_endpoint            VARCHAR(128),
    upstream_endpoint           VARCHAR(128),
    upstream_model              VARCHAR(100),
    requested_model             VARCHAR(100),
    channel_id                  BIGINT,
    model_mapping_chain         VARCHAR(500),
    billing_tier                VARCHAR(50),
    billing_mode                VARCHAR(20),
    image_output_tokens         INTEGER NOT NULL DEFAULT 0,
    image_output_cost           DECIMAL(20, 10) NOT NULL DEFAULT 0,
    group_id                    BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    rate_multiplier             DECIMAL(10, 4) NOT NULL DEFAULT 1,
    first_token_ms              INT,
    account_stats_cost          DECIMAL(20, 10),
    image_input_size            VARCHAR(32),
    image_output_size           VARCHAR(32),
    image_size_source           VARCHAR(16),
    image_size_breakdown        TEXT,
    CONSTRAINT usage_logs_request_type_check CHECK (request_type IN (0, 1, 2, 3)),
    CONSTRAINT usage_logs_image_size_source_check CHECK (image_size_source IS NULL OR image_size_source IN ('output', 'input', 'default', 'legacy'))
);

CREATE INDEX IF NOT EXISTS idx_usage_logs_user_id ON usage_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_api_key_id ON usage_logs(api_key_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_account_id ON usage_logs(account_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_model ON usage_logs(model);
CREATE INDEX IF NOT EXISTS idx_usage_logs_created_at ON usage_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_usage_logs_user_created ON usage_logs(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_usage_logs_billing_type ON usage_logs(billing_type);
CREATE INDEX IF NOT EXISTS idx_usage_logs_ip_address ON usage_logs(ip_address);
CREATE INDEX IF NOT EXISTS idx_usage_logs_request_type_created_at ON usage_logs(request_type, created_at);
CREATE INDEX IF NOT EXISTS idx_usage_logs_service_tier_created_at ON usage_logs(service_tier, created_at);
CREATE INDEX IF NOT EXISTS idx_usage_logs_group_id ON usage_logs(group_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_channel_id ON usage_logs(channel_id);

-- ============================================================
-- settings
-- ============================================================
CREATE TABLE IF NOT EXISTS settings (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key         VARCHAR(100) NOT NULL UNIQUE,
    value       TEXT NOT NULL,
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- ============================================================
-- channels
-- ============================================================
CREATE TABLE IF NOT EXISTS channels (
    id                              INTEGER PRIMARY KEY AUTOINCREMENT,
    name                            VARCHAR(100) NOT NULL,
    description                     TEXT DEFAULT '',
    status                          VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at                      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at                      DATETIME NOT NULL DEFAULT (datetime('now')),
    billing_model_source            VARCHAR(20) DEFAULT 'channel_mapped',
    apply_pricing_to_account_stats  BOOLEAN NOT NULL DEFAULT FALSE,
    features                        TEXT NOT NULL DEFAULT '',
    features_config                 TEXT NOT NULL DEFAULT '{}',
    model_mapping                   TEXT DEFAULT '{}',
    restrict_models                 BOOLEAN DEFAULT FALSE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_channels_name ON channels(name);
CREATE INDEX IF NOT EXISTS idx_channels_status ON channels(status);

-- ============================================================
-- channel_groups
-- ============================================================
CREATE TABLE IF NOT EXISTS channel_groups (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id  BIGINT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    group_id    BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_channel_groups_group_id ON channel_groups(group_id);
CREATE INDEX IF NOT EXISTS idx_channel_groups_channel_id ON channel_groups(channel_id);

-- ============================================================
-- channel_model_pricing
-- ============================================================
CREATE TABLE IF NOT EXISTS channel_model_pricing (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id          BIGINT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    models              TEXT NOT NULL DEFAULT '[]',
    input_price         DECIMAL(20, 12),
    output_price        DECIMAL(20, 12),
    cache_write_price   DECIMAL(20, 12),
    cache_read_price    DECIMAL(20, 12),
    image_output_price  DECIMAL(20, 8),
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    billing_mode        VARCHAR(20) NOT NULL DEFAULT 'token',
    per_request_price   DECIMAL(20, 10),
    platform            VARCHAR(50) NOT NULL DEFAULT 'anthropic'
);

CREATE INDEX IF NOT EXISTS idx_channel_model_pricing_channel_id ON channel_model_pricing(channel_id);
CREATE INDEX IF NOT EXISTS idx_channel_model_pricing_platform ON channel_model_pricing(platform);

-- ============================================================
-- channel_pricing_intervals
-- ============================================================
CREATE TABLE IF NOT EXISTS channel_pricing_intervals (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    pricing_id          BIGINT NOT NULL REFERENCES channel_model_pricing(id) ON DELETE CASCADE,
    min_tokens          INT NOT NULL DEFAULT 0,
    max_tokens          INT,
    tier_label          VARCHAR(50),
    input_price         DECIMAL(20, 12),
    output_price        DECIMAL(20, 12),
    cache_write_price   DECIMAL(20, 12),
    cache_read_price    DECIMAL(20, 12),
    per_request_price   DECIMAL(20, 12),
    sort_order          INT NOT NULL DEFAULT 0,
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_channel_pricing_intervals_pricing_id ON channel_pricing_intervals(pricing_id);

-- ============================================================
-- channel_account_stats_pricing_rules
-- ============================================================
CREATE TABLE IF NOT EXISTS channel_account_stats_pricing_rules (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id  BIGINT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL DEFAULT '',
    group_ids   TEXT NOT NULL DEFAULT '[]',
    account_ids TEXT NOT NULL DEFAULT '[]',
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_cas_pricing_rules_channel_id ON channel_account_stats_pricing_rules(channel_id);

-- ============================================================
-- channel_account_stats_model_pricing
-- ============================================================
CREATE TABLE IF NOT EXISTS channel_account_stats_model_pricing (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id             BIGINT NOT NULL REFERENCES channel_account_stats_pricing_rules(id) ON DELETE CASCADE,
    platform            VARCHAR(50) NOT NULL DEFAULT '',
    models              TEXT NOT NULL DEFAULT '[]',
    billing_mode        VARCHAR(20) NOT NULL DEFAULT 'token',
    input_price         DECIMAL(20, 10),
    output_price        DECIMAL(20, 10),
    cache_write_price   DECIMAL(20, 10),
    cache_read_price    DECIMAL(20, 10),
    image_output_price  DECIMAL(20, 10),
    per_request_price   DECIMAL(20, 10),
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_cas_model_pricing_rule_id ON channel_account_stats_model_pricing(rule_id);

-- ============================================================
-- tls_fingerprint_profiles
-- ============================================================
CREATE TABLE IF NOT EXISTS tls_fingerprint_profiles (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    name                  VARCHAR(100) NOT NULL UNIQUE,
    description           TEXT,
    enable_grease         BOOLEAN NOT NULL DEFAULT FALSE,
    cipher_suites         TEXT,
    curves                TEXT,
    point_formats         TEXT,
    signature_algorithms  TEXT,
    alpn_protocols        TEXT,
    supported_versions    TEXT,
    key_share_groups      TEXT,
    psk_modes             TEXT,
    extensions            TEXT,
    created_at            DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at            DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- ============================================================
-- error_passthrough_rules
-- ============================================================
CREATE TABLE IF NOT EXISTS error_passthrough_rules (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            VARCHAR(100) NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    priority        INTEGER NOT NULL DEFAULT 0,
    error_codes     TEXT DEFAULT '[]',
    keywords        TEXT DEFAULT '[]',
    match_mode      VARCHAR(10) NOT NULL DEFAULT 'any',
    platforms       TEXT DEFAULT '[]',
    passthrough_code BOOLEAN NOT NULL DEFAULT TRUE,
    response_code   INTEGER,
    passthrough_body BOOLEAN NOT NULL DEFAULT TRUE,
    custom_message  TEXT,
    description     TEXT,
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_error_passthrough_rules_enabled ON error_passthrough_rules(enabled);
CREATE INDEX IF NOT EXISTS idx_error_passthrough_rules_priority ON error_passthrough_rules(priority);

-- ============================================================
-- idempotency_records
-- ============================================================
CREATE TABLE IF NOT EXISTS idempotency_records (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    scope               VARCHAR(128) NOT NULL,
    idempotency_key_hash VARCHAR(64) NOT NULL,
    request_fingerprint VARCHAR(64) NOT NULL,
    status              VARCHAR(32) NOT NULL,
    response_status     INTEGER,
    response_body       TEXT,
    error_reason        VARCHAR(128),
    locked_until        DATETIME,
    expires_at          DATETIME NOT NULL,
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_idempotency_records_scope_key ON idempotency_records(scope, idempotency_key_hash);
CREATE INDEX IF NOT EXISTS idx_idempotency_records_expires_at ON idempotency_records(expires_at);
CREATE INDEX IF NOT EXISTS idx_idempotency_records_status_locked_until ON idempotency_records(status, locked_until);

-- ============================================================
-- security_secrets
-- ============================================================
CREATE TABLE IF NOT EXISTS security_secrets (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key         VARCHAR(100) NOT NULL UNIQUE,
    value       TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_security_secrets_key ON security_secrets(key);

-- ============================================================
-- announcements
-- ============================================================
CREATE TABLE IF NOT EXISTS announcements (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       VARCHAR(200) NOT NULL,
    content     TEXT NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'draft',
    targeting   TEXT NOT NULL DEFAULT '{}',
    starts_at   DATETIME DEFAULT NULL,
    ends_at     DATETIME DEFAULT NULL,
    created_by  BIGINT DEFAULT NULL REFERENCES users(id) ON DELETE SET NULL,
    updated_by  BIGINT DEFAULT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    notify_mode VARCHAR(20) NOT NULL DEFAULT 'silent'
);

CREATE INDEX IF NOT EXISTS idx_announcements_status ON announcements(status);
CREATE INDEX IF NOT EXISTS idx_announcements_starts_at ON announcements(starts_at);
CREATE INDEX IF NOT EXISTS idx_announcements_ends_at ON announcements(ends_at);
CREATE INDEX IF NOT EXISTS idx_announcements_created_at ON announcements(created_at);

-- ============================================================
-- announcement_reads
-- ============================================================
CREATE TABLE IF NOT EXISTS announcement_reads (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    announcement_id BIGINT NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    read_at         DATETIME NOT NULL DEFAULT (datetime('now')),
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(announcement_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_announcement_reads_announcement_id ON announcement_reads(announcement_id);
CREATE INDEX IF NOT EXISTS idx_announcement_reads_user_id ON announcement_reads(user_id);
CREATE INDEX IF NOT EXISTS idx_announcement_reads_read_at ON announcement_reads(read_at);

-- ============================================================
-- auth_identities
-- ============================================================
CREATE TABLE IF NOT EXISTS auth_identities (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider_type   VARCHAR(20) NOT NULL,
    provider_key    TEXT NOT NULL,
    provider_subject TEXT NOT NULL,
    verified_at     DATETIME,
    issuer          TEXT,
    metadata        TEXT NOT NULL DEFAULT '{}',
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    CONSTRAINT auth_identities_provider_type_check CHECK (provider_type IN ('email', 'linuxdo', 'wechat', 'oidc', 'github', 'google', 'dingtalk'))
);

CREATE UNIQUE INDEX IF NOT EXISTS auth_identities_provider_subject_key ON auth_identities(provider_type, provider_key, provider_subject);
CREATE INDEX IF NOT EXISTS auth_identities_user_id_idx ON auth_identities(user_id);
CREATE INDEX IF NOT EXISTS auth_identities_user_provider_idx ON auth_identities(user_id, provider_type);

-- ============================================================
-- auth_identity_channels
-- ============================================================
CREATE TABLE IF NOT EXISTS auth_identity_channels (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    identity_id     BIGINT NOT NULL REFERENCES auth_identities(id) ON DELETE CASCADE,
    provider_type   VARCHAR(20) NOT NULL,
    provider_key    TEXT NOT NULL,
    channel         VARCHAR(20) NOT NULL,
    channel_app_id  TEXT NOT NULL,
    channel_subject TEXT NOT NULL,
    metadata        TEXT NOT NULL DEFAULT '{}',
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    CONSTRAINT auth_identity_channels_provider_type_check CHECK (provider_type IN ('email', 'linuxdo', 'wechat', 'oidc', 'github', 'google', 'dingtalk'))
);

CREATE UNIQUE INDEX IF NOT EXISTS auth_identity_channels_channel_key ON auth_identity_channels(provider_type, provider_key, channel, channel_app_id, channel_subject);
CREATE INDEX IF NOT EXISTS auth_identity_channels_identity_id_idx ON auth_identity_channels(identity_id);

-- ============================================================
-- pending_auth_sessions
-- ============================================================
CREATE TABLE IF NOT EXISTS pending_auth_sessions (
    id                          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_token               VARCHAR(255) NOT NULL,
    intent                      VARCHAR(40) NOT NULL,
    provider_type               VARCHAR(20) NOT NULL,
    provider_key                TEXT NOT NULL,
    provider_subject            TEXT NOT NULL,
    target_user_id              BIGINT REFERENCES users(id) ON DELETE SET NULL,
    redirect_to                 TEXT NOT NULL DEFAULT '',
    resolved_email              TEXT NOT NULL DEFAULT '',
    registration_password_hash  TEXT NOT NULL DEFAULT '',
    upstream_identity_claims    TEXT NOT NULL DEFAULT '{}',
    local_flow_state            TEXT NOT NULL DEFAULT '{}',
    browser_session_key         TEXT NOT NULL DEFAULT '',
    completion_code_hash        TEXT NOT NULL DEFAULT '',
    completion_code_expires_at  DATETIME,
    email_verified_at           DATETIME,
    password_verified_at        DATETIME,
    totp_verified_at            DATETIME,
    expires_at                  DATETIME NOT NULL,
    consumed_at                 DATETIME,
    created_at                  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at                  DATETIME NOT NULL DEFAULT (datetime('now')),
    CONSTRAINT pending_auth_sessions_intent_check CHECK (intent IN ('login', 'bind_current_user', 'adopt_existing_user_by_email')),
    CONSTRAINT pending_auth_sessions_provider_type_check CHECK (provider_type IN ('email', 'linuxdo', 'wechat', 'oidc', 'github', 'google', 'dingtalk'))
);

CREATE UNIQUE INDEX IF NOT EXISTS pending_auth_sessions_session_token_key ON pending_auth_sessions(session_token);
CREATE INDEX IF NOT EXISTS pending_auth_sessions_target_user_id_idx ON pending_auth_sessions(target_user_id);
CREATE INDEX IF NOT EXISTS pending_auth_sessions_expires_at_idx ON pending_auth_sessions(expires_at);
CREATE INDEX IF NOT EXISTS pending_auth_sessions_provider_idx ON pending_auth_sessions(provider_type, provider_key, provider_subject);
CREATE INDEX IF NOT EXISTS pending_auth_sessions_completion_code_idx ON pending_auth_sessions(completion_code_hash);

-- ============================================================
-- identity_adoption_decisions
-- ============================================================
CREATE TABLE IF NOT EXISTS identity_adoption_decisions (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    pending_auth_session_id BIGINT NOT NULL REFERENCES pending_auth_sessions(id) ON DELETE CASCADE,
    identity_id             BIGINT REFERENCES auth_identities(id) ON DELETE SET NULL,
    adopt_display_name      BOOLEAN NOT NULL DEFAULT FALSE,
    adopt_avatar            BOOLEAN NOT NULL DEFAULT FALSE,
    decided_at              DATETIME NOT NULL DEFAULT (datetime('now')),
    created_at              DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at              DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS identity_adoption_decisions_pending_auth_session_id_key ON identity_adoption_decisions(pending_auth_session_id);
CREATE INDEX IF NOT EXISTS identity_adoption_decisions_identity_id_idx ON identity_adoption_decisions(identity_id);

-- ============================================================
-- auth_identity_migration_reports
-- ============================================================
CREATE TABLE IF NOT EXISTS auth_identity_migration_reports (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    report_type VARCHAR(40) NOT NULL,
    report_key  TEXT NOT NULL,
    details     TEXT NOT NULL DEFAULT '{}',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS auth_identity_migration_reports_type_idx ON auth_identity_migration_reports(report_type);
CREATE UNIQUE INDEX IF NOT EXISTS auth_identity_migration_reports_type_key ON auth_identity_migration_reports(report_type, report_key);

-- ============================================================
-- channel_monitors
-- ============================================================
CREATE TABLE IF NOT EXISTS channel_monitors (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    name                VARCHAR(100) NOT NULL,
    provider            VARCHAR(20) NOT NULL,
    endpoint            VARCHAR(500) NOT NULL,
    api_key_encrypted   TEXT NOT NULL,
    primary_model       VARCHAR(200) NOT NULL,
    extra_models        TEXT NOT NULL DEFAULT '[]',
    group_name          VARCHAR(100) NOT NULL DEFAULT '',
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,
    interval_seconds    INT NOT NULL,
    last_checked_at     DATETIME,
    created_by          BIGINT NOT NULL,
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    template_id         BIGINT,
    extra_headers       TEXT NOT NULL DEFAULT '{}',
    body_override_mode  VARCHAR(10) NOT NULL DEFAULT 'off',
    body_override       TEXT,
    api_mode            VARCHAR(32) NOT NULL DEFAULT 'chat_completions',
    CONSTRAINT channel_monitors_provider_check CHECK (provider IN ('openai', 'anthropic', 'gemini')),
    CONSTRAINT channel_monitors_interval_check CHECK (interval_seconds BETWEEN 15 AND 3600),
    CONSTRAINT channel_monitors_body_mode_check CHECK (body_override_mode IN ('off', 'merge', 'replace')),
    CONSTRAINT channel_monitors_api_mode_check CHECK (api_mode IN ('chat_completions', 'responses'))
);

CREATE INDEX IF NOT EXISTS idx_channel_monitors_enabled_last_checked ON channel_monitors(enabled, last_checked_at);
CREATE INDEX IF NOT EXISTS idx_channel_monitors_provider ON channel_monitors(provider);
CREATE INDEX IF NOT EXISTS idx_channel_monitors_group_name ON channel_monitors(group_name);
CREATE INDEX IF NOT EXISTS idx_channel_monitors_template_id ON channel_monitors(template_id);
CREATE INDEX IF NOT EXISTS idx_channel_monitors_provider_api_mode ON channel_monitors(provider, api_mode);

-- ============================================================
-- channel_monitor_histories
-- ============================================================
CREATE TABLE IF NOT EXISTS channel_monitor_histories (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    monitor_id      BIGINT NOT NULL REFERENCES channel_monitors(id) ON DELETE CASCADE,
    model           VARCHAR(200) NOT NULL,
    status          VARCHAR(20) NOT NULL,
    latency_ms      INT,
    ping_latency_ms INT,
    message         VARCHAR(500) NOT NULL DEFAULT '',
    checked_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    CONSTRAINT channel_monitor_histories_status_check CHECK (status IN ('operational', 'degraded', 'failed', 'error'))
);

CREATE INDEX IF NOT EXISTS idx_channel_monitor_histories_monitor_model_checked ON channel_monitor_histories(monitor_id, model, checked_at DESC);
CREATE INDEX IF NOT EXISTS idx_channel_monitor_histories_checked_at ON channel_monitor_histories(checked_at);

-- ============================================================
-- channel_monitor_daily_rollups
-- ============================================================
CREATE TABLE IF NOT EXISTS channel_monitor_daily_rollups (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    monitor_id          BIGINT NOT NULL REFERENCES channel_monitors(id) ON DELETE CASCADE,
    model               VARCHAR(200) NOT NULL,
    bucket_date         TEXT NOT NULL,
    total_checks        INT NOT NULL DEFAULT 0,
    ok_count            INT NOT NULL DEFAULT 0,
    operational_count   INT NOT NULL DEFAULT 0,
    degraded_count      INT NOT NULL DEFAULT 0,
    failed_count        INT NOT NULL DEFAULT 0,
    error_count         INT NOT NULL DEFAULT 0,
    sum_latency_ms      BIGINT NOT NULL DEFAULT 0,
    count_latency       INT NOT NULL DEFAULT 0,
    sum_ping_latency_ms BIGINT NOT NULL DEFAULT 0,
    count_ping_latency  INT NOT NULL DEFAULT 0,
    computed_at         DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_channel_monitor_daily_rollups_unique ON channel_monitor_daily_rollups(monitor_id, model, bucket_date);
CREATE INDEX IF NOT EXISTS idx_channel_monitor_daily_rollups_bucket ON channel_monitor_daily_rollups(bucket_date);

-- ============================================================
-- channel_monitor_aggregation_watermark
-- ============================================================
CREATE TABLE IF NOT EXISTS channel_monitor_aggregation_watermark (
    id                   INTEGER PRIMARY KEY DEFAULT 1,
    last_aggregated_date TEXT,
    updated_at           DATETIME NOT NULL DEFAULT (datetime('now')),
    CONSTRAINT channel_monitor_aggregation_watermark_singleton CHECK (id = 1)
);

INSERT OR IGNORE INTO channel_monitor_aggregation_watermark (id, last_aggregated_date, updated_at)
VALUES (1, NULL, datetime('now'));

-- ============================================================
-- channel_monitor_request_templates
-- ============================================================
CREATE TABLE IF NOT EXISTS channel_monitor_request_templates (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    name                VARCHAR(100) NOT NULL,
    provider            VARCHAR(20) NOT NULL,
    description         VARCHAR(500) NOT NULL DEFAULT '',
    extra_headers       TEXT NOT NULL DEFAULT '{}',
    body_override_mode  VARCHAR(10) NOT NULL DEFAULT 'off',
    body_override       TEXT,
    api_mode            VARCHAR(32) NOT NULL DEFAULT 'chat_completions',
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    CONSTRAINT channel_monitor_request_templates_provider_check CHECK (provider IN ('openai', 'anthropic', 'gemini')),
    CONSTRAINT channel_monitor_request_templates_body_mode_check CHECK (body_override_mode IN ('off', 'merge', 'replace')),
    CONSTRAINT channel_monitor_request_templates_api_mode_check CHECK (api_mode IN ('chat_completions', 'responses'))
);

CREATE UNIQUE INDEX IF NOT EXISTS channel_monitor_request_templates_provider_name ON channel_monitor_request_templates(provider, name);
CREATE INDEX IF NOT EXISTS idx_channel_monitor_templates_provider_api_mode ON channel_monitor_request_templates(provider, api_mode);

-- ============================================================
-- usage_cleanup_tasks
-- ============================================================
CREATE TABLE IF NOT EXISTS usage_cleanup_tasks (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    status          VARCHAR(20) NOT NULL,
    filters         TEXT NOT NULL,
    created_by      BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    deleted_rows    BIGINT NOT NULL DEFAULT 0,
    error_message   TEXT,
    started_at      DATETIME,
    finished_at     DATETIME,
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_usage_cleanup_tasks_status_created_at ON usage_cleanup_tasks(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_cleanup_tasks_created_at ON usage_cleanup_tasks(created_at DESC);

-- ============================================================
-- scheduled_test_plans
-- ============================================================
CREATE TABLE IF NOT EXISTS scheduled_test_plans (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    model_id        VARCHAR(100) NOT NULL DEFAULT '',
    cron_expression VARCHAR(100) NOT NULL DEFAULT '*/30 * * * *',
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    max_results     INT NOT NULL DEFAULT 50,
    last_run_at     DATETIME,
    next_run_at     DATETIME,
    auto_recover    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_stp_account_id ON scheduled_test_plans(account_id);
CREATE INDEX IF NOT EXISTS idx_stp_enabled_next_run ON scheduled_test_plans(enabled, next_run_at);

-- ============================================================
-- scheduled_test_results
-- ============================================================
CREATE TABLE IF NOT EXISTS scheduled_test_results (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_id       BIGINT NOT NULL REFERENCES scheduled_test_plans(id) ON DELETE CASCADE,
    status        VARCHAR(20) NOT NULL DEFAULT 'success',
    response_text TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    latency_ms    BIGINT NOT NULL DEFAULT 0,
    started_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    finished_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    created_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_str_plan_created ON scheduled_test_results(plan_id, created_at DESC);

-- ============================================================
-- user_group_rate_multipliers
-- ============================================================
CREATE TABLE IF NOT EXISTS user_group_rate_multipliers (
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id        BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    rate_multiplier DECIMAL(10, 4),
    rpm_override    INT,
    created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at      DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (user_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_user_group_rate_multipliers_group_id ON user_group_rate_multipliers(group_id);
`
