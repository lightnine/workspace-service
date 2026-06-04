-- kernel_session: Jupyter Session / Kernel registry for workspace-service (replaces
-- wedata-jupyter-server SQLite `session` table in the target architecture).
-- WebSocket connections are NOT stored here; only session metadata for ACL and lifecycle.
CREATE TABLE IF NOT EXISTS kernel_session (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT 'primary key',
    session_id VARCHAR(64) NOT NULL COMMENT 'Jupyter session uuid',
    path VARCHAR(1024) NOT NULL DEFAULT '' COMMENT 'notebook or script path in workspace',
    name VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'display name, often filename',
    type VARCHAR(64) NOT NULL DEFAULT 'notebook' COMMENT 'notebook | file | ...',
    kernel_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'kernel id on DLC EG / gateway',
    kernel_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'kernelspec name',
    cluster TEXT NULL COMMENT 'DLC cluster descriptor (JSON text)',
    custom_envs JSON NULL COMMENT 'KERNEL_* / mount env snapshot from create request',
    launch_plan JSON NULL COMMENT 'optional KernelLaunchPlan snapshot (future)',
    state VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'starting | busy | idle | dead | ...',
    owner_uin VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'owner account uin',
    uin VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'operator sub-account uin',
    app_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'tenant / app id',
    workspace_id VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'workspace id',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT 'created time',
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT 'updated time',
    deleted_at DATETIME(3) NULL DEFAULT NULL COMMENT 'soft-delete when session removed',
    PRIMARY KEY (id),
    UNIQUE KEY uk_kernel_session_session_id (session_id),
    KEY idx_kernel_session_kernel_id (kernel_id),
    KEY idx_kernel_session_app_workspace (app_id, workspace_id),
    KEY idx_kernel_session_owner_uin (owner_uin),
    KEY idx_kernel_session_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='Jupyter session and kernel mapping per workspace tenant';
