-- ws_file_intent: write-ahead journal for create/write file operations.
-- A pending row is persisted BEFORE the storage write so a crash between the
-- storage write and the ws_file_node upsert can be reconciled on restart:
--   target path exists  -> upsert ws_file_node, mark status=done
--   target path missing -> mark status=aborted (no file + no node row = consistent)
CREATE TABLE IF NOT EXISTS ws_file_intent (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    intent_id VARCHAR(64) NOT NULL COMMENT 'logical intent id (idempotency key)',
    op VARCHAR(32) NOT NULL COMMENT 'create_folder | create_file | write_file | create_notebook',
    abs_path VARCHAR(1024) NOT NULL COMMENT 'absolute path on mount; re-stat target on recovery',
    owner_uin VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'owner account uin',
    uin VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'operator sub-account uin',
    app_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'tenant / app id',
    workspace_id VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'workspace id',
    node_type VARCHAR(16) NOT NULL DEFAULT 'file' COMMENT 'file | directory | git_folder | notebook',
    status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending | done | aborted',
    attempts INT NOT NULL DEFAULT 0 COMMENT 'recovery retry counter',
    last_error VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'last recovery error message',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT 'created time',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT 'updated time',
    PRIMARY KEY (id),
    UNIQUE KEY uk_ws_file_intent_intent_id (intent_id),
    KEY idx_ws_file_intent_status (status),
    KEY idx_ws_file_intent_app_workspace (app_id, workspace_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='write-ahead journal for file create/write operations';
