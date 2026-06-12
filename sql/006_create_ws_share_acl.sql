-- ws_share_acl: folder/object share permissions (P1).
CREATE TABLE IF NOT EXISTS ws_share_acl (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    acl_id VARCHAR(64) NOT NULL COMMENT 'acl entry uuid',
    resource_path VARCHAR(1024) NOT NULL COMMENT 'shared folder or file path',
    resource_type VARCHAR(16) NOT NULL DEFAULT 'folder' COMMENT 'folder | file | notebook',
    grantee_type VARCHAR(16) NOT NULL DEFAULT 'user' COMMENT 'user | group | workspace',
    grantee_id VARCHAR(128) NOT NULL COMMENT 'uin or group id',
    permission VARCHAR(32) NOT NULL DEFAULT 'read' COMMENT 'read | write | manage',
    owner_uin VARCHAR(64) NOT NULL COMMENT 'resource owner',
    uin VARCHAR(64) NOT NULL COMMENT 'who granted',
    app_id VARCHAR(64) NOT NULL COMMENT 'tenant / app id',
    workspace_id VARCHAR(128) NOT NULL COMMENT 'workspace id',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    deleted_at DATETIME(3) NULL DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_ws_share_acl_acl_id (acl_id),
    KEY idx_ws_share_acl_resource (app_id, workspace_id, resource_path(191)),
    KEY idx_ws_share_acl_grantee (grantee_type, grantee_id),
    KEY idx_ws_share_acl_owner_uin (owner_uin)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='workspace object share ACL';
