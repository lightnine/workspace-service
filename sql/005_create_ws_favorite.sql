-- ws_favorite: per-user starred workspace objects (P1).
CREATE TABLE IF NOT EXISTS ws_favorite (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    owner_uin VARCHAR(64) NOT NULL COMMENT 'owner account uin',
    uin VARCHAR(64) NOT NULL COMMENT 'user who favorited',
    app_id VARCHAR(64) NOT NULL COMMENT 'tenant / app id',
    workspace_id VARCHAR(128) NOT NULL COMMENT 'workspace id',
    object_path VARCHAR(1024) NOT NULL COMMENT 'workspace-relative path',
    node_type VARCHAR(16) NOT NULL DEFAULT 'file' COMMENT 'file | directory | git_folder | notebook',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    deleted_at DATETIME(3) NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_ws_favorite_user_path (uin, app_id, workspace_id, object_path(191)),
    KEY idx_ws_favorite_app_workspace (app_id, workspace_id),
    KEY idx_ws_favorite_owner_uin (owner_uin)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='user favorites in workspace browser';
