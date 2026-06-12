-- ws_file_node: JuiceFS inode -> tenant workspace operator mapping.
-- Distinct from JuiceFS native jfs_node; stores business owner/creator and node_type.
CREATE TABLE IF NOT EXISTS ws_file_node (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    inode_id BIGINT UNSIGNED NOT NULL COMMENT 'JuiceFS / mount inode id',
    owner_uin VARCHAR(64) NOT NULL COMMENT 'owner account uin',
    uin VARCHAR(64) NOT NULL COMMENT 'operator sub-account uin',
    app_id VARCHAR(64) NOT NULL COMMENT 'tenant / app id',
    workspace_id VARCHAR(128) NOT NULL COMMENT 'workspace id',
    node_type VARCHAR(16) NOT NULL DEFAULT 'file' COMMENT 'file | directory | git_folder | notebook',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT 'created time',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT 'updated time',
    deleted_at DATETIME(3) NULL DEFAULT NULL COMMENT 'soft-delete in business table',
    PRIMARY KEY (id),
    UNIQUE KEY uk_ws_file_node_inode_id (inode_id),
    KEY idx_ws_file_node_owner_uin (owner_uin),
    KEY idx_ws_file_node_uin (uin),
    KEY idx_ws_file_node_app_workspace (app_id, workspace_id),
    KEY idx_ws_file_node_node_type (node_type),
    KEY idx_ws_file_node_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='workspace inode operator and node type mapping';
