package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	appconfig "git.woa.com/leondli/workspace-service/internal/config"
	domainsession "git.woa.com/leondli/workspace-service/internal/domain/session"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type KernelSessionStore struct {
	db *gorm.DB
}

type KernelSession struct {
	ID          uint64          `gorm:"column:id;primaryKey;autoIncrement"`
	SessionID   string          `gorm:"column:session_id;uniqueIndex:uk_ws_kernel_session_session_id"`
	Path        string          `gorm:"column:path"`
	Name        string          `gorm:"column:name"`
	Type        string          `gorm:"column:type"`
	KernelID    string          `gorm:"column:kernel_id;index:idx_ws_kernel_session_kernel_id"`
	KernelName  string          `gorm:"column:kernel_name"`
	Cluster     string          `gorm:"column:cluster"`
	CustomEnvs  json.RawMessage `gorm:"column:custom_envs"`
	LaunchPlan  json.RawMessage `gorm:"column:launch_plan"`
	State       string          `gorm:"column:state"`
	OwnerUIN    string          `gorm:"column:owner_uin"`
	UIN         string          `gorm:"column:uin"`
	AppID       string          `gorm:"column:app_id"`
	WorkspaceID string          `gorm:"column:workspace_id"`
	CreatedAt   time.Time       `gorm:"column:created_at"`
	UpdatedAt   time.Time       `gorm:"column:updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"column:deleted_at;index:idx_ws_kernel_session_deleted_at"`
}

func (KernelSession) TableName() string {
	return "ws_kernel_session"
}

func NewOptionalKernelSessionStore(cfg appconfig.MySQLConfig) (domainsession.Store, error) {
	if cfg.DSN == "" {
		return nil, nil
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return &KernelSessionStore{db: db}, nil
}

func kernelSessionFromRecord(record domainsession.Record) KernelSession {
	return KernelSession{
		SessionID:   record.SessionID,
		Path:        record.Path,
		Name:        record.Name,
		Type:        record.Type,
		KernelID:    record.KernelID,
		KernelName:  record.KernelName,
		Cluster:     record.Cluster,
		CustomEnvs:  record.CustomEnvs,
		LaunchPlan:  record.LaunchPlan,
		State:       record.State,
		OwnerUIN:    record.OwnerUIN,
		UIN:         record.UIN,
		AppID:       record.AppID,
		WorkspaceID: record.WorkspaceID,
	}
}

func (s *KernelSessionStore) Upsert(ctx context.Context, record domainsession.Record) error {
	if record.SessionID == "" {
		return errors.New("session_id is required")
	}
	row := kernelSessionFromRecord(record)
	updates := map[string]any{
		"path":         record.Path,
		"name":         record.Name,
		"type":         record.Type,
		"kernel_id":    record.KernelID,
		"kernel_name":  record.KernelName,
		"cluster":      record.Cluster,
		"custom_envs":  record.CustomEnvs,
		"launch_plan":  record.LaunchPlan,
		"state":        record.State,
		"owner_uin":    record.OwnerUIN,
		"uin":          record.UIN,
		"app_id":       record.AppID,
		"workspace_id": record.WorkspaceID,
		"deleted_at":   nil,
		"updated_at":   gorm.Expr("CURRENT_TIMESTAMP(3)"),
	}

	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "session_id"}},
			DoUpdates: clause.Assignments(updates),
		}).
		Create(&row).
		Error
}

func (s *KernelSessionStore) UpdateState(ctx context.Context, sessionID, state string) error {
	return s.db.WithContext(ctx).
		Model(&KernelSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]any{
			"state":      state,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP(3)"),
		}).
		Error
}

func (s *KernelSessionStore) MarkDeleted(ctx context.Context, sessionID string) error {
	return s.db.WithContext(ctx).
		Model(&KernelSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]any{
			"deleted_at": gorm.Expr("CURRENT_TIMESTAMP(3)"),
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP(3)"),
		}).
		Error
}

func (s *KernelSessionStore) GetBySessionID(ctx context.Context, sessionID string) (domainsession.Record, error) {
	var row KernelSession
	err := s.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Where("deleted_at IS NULL").
		First(&row).Error
	if err != nil {
		return domainsession.Record{}, err
	}
	return recordFromRow(row), nil
}

func (s *KernelSessionStore) GetByKernelID(ctx context.Context, kernelID string) (domainsession.Record, error) {
	var row KernelSession
	err := s.db.WithContext(ctx).
		Where("kernel_id = ?", kernelID).
		Where("deleted_at IS NULL").
		Order("updated_at DESC").
		First(&row).Error
	if err != nil {
		return domainsession.Record{}, err
	}
	return recordFromRow(row), nil
}

func recordFromRow(row KernelSession) domainsession.Record {
	return domainsession.Record{
		SessionID:   row.SessionID,
		Path:        row.Path,
		Name:        row.Name,
		Type:        row.Type,
		KernelID:    row.KernelID,
		KernelName:  row.KernelName,
		Cluster:     row.Cluster,
		CustomEnvs:  row.CustomEnvs,
		LaunchPlan:  row.LaunchPlan,
		State:       row.State,
		OwnerUIN:    row.OwnerUIN,
		UIN:         row.UIN,
		AppID:       row.AppID,
		WorkspaceID: row.WorkspaceID,
	}
}
