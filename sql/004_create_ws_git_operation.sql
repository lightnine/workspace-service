-- ws_git_operation: optional git operation audit trail (P2).
CREATE TABLE IF NOT EXISTS ws_git_operation (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    operation_id VARCHAR(64) NOT NULL COMMENT 'operation uuid',
    source VARCHAR(32) NOT NULL DEFAULT 'frontend' COMMENT 'frontend | terminal',
    operation_type VARCHAR(32) NOT NULL COMMENT 'clone | pull | commit | push | ...',
    repo_path VARCHAR(1024) NOT NULL DEFAULT '' COMMENT 'git folder path in workspace',
    branch VARCHAR(256) NOT NULL DEFAULT '',
    commit_id VARCHAR(64) NOT NULL DEFAULT '',
    result VARCHAR(32) NOT NULL DEFAULT 'ok' COMMENT 'ok | failed',
    error_message TEXT NULL,
    owner_uin VARCHAR(64) NOT NULL DEFAULT '',
    uin VARCHAR(64) NOT NULL DEFAULT '',
    app_id VARCHAR(64) NOT NULL DEFAULT '',
    workspace_id VARCHAR(128) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_ws_git_operation_operation_id (operation_id),
    KEY idx_ws_git_operation_app_workspace (app_id, workspace_id),
    KEY idx_ws_git_operation_repo_path (repo_path(191)),
    KEY idx_ws_git_operation_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='git operation audit log';
