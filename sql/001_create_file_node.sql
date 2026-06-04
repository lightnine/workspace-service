CREATE TABLE IF NOT EXISTS file_node (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    inode_id BIGINT UNSIGNED NOT NULL COMMENT 'JuiceFS inode id',
    owner_uin VARCHAR(64) NOT NULL COMMENT 'owner account uin',
    uin VARCHAR(64) NOT NULL COMMENT 'operator sub-account uin',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT 'created time',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT 'updated time',
    deleted_at DATETIME(3) NULL DEFAULT NULL COMMENT 'deleted time',
    PRIMARY KEY (id),
    UNIQUE KEY uk_file_node_inode_id (inode_id),
    KEY idx_file_node_owner_uin (owner_uin),
    KEY idx_file_node_uin (uin),
    KEY idx_file_node_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='workspace file inode to operator mapping';
