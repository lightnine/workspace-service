package mysql

import (
	"context"
	"errors"
	"time"

	appconfig "git.woa.com/leondli/workspace-service/internal/config"
	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FileIntentStore persists write-ahead file intents in ws_file_intent.
type FileIntentStore struct {
	db *gorm.DB
}

type FileIntent struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	IntentID    string    `gorm:"column:intent_id;uniqueIndex:uk_ws_file_intent_intent_id"`
	Op          string    `gorm:"column:op"`
	AbsPath     string    `gorm:"column:abs_path"`
	OwnerUIN    string    `gorm:"column:owner_uin"`
	UIN         string    `gorm:"column:uin"`
	AppID       string    `gorm:"column:app_id"`
	WorkspaceID string    `gorm:"column:workspace_id"`
	NodeType    string    `gorm:"column:node_type"`
	Status      string    `gorm:"column:status;index:idx_ws_file_intent_status"`
	Attempts    int       `gorm:"column:attempts"`
	LastError   string    `gorm:"column:last_error"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (FileIntent) TableName() string {
	return "ws_file_intent"
}

// NewOptionalFileIntentStore returns nil (no error) when the MySQL DSN is empty,
// matching the other optional stores so the service degrades gracefully.
func NewOptionalFileIntentStore(cfg appconfig.MySQLConfig) (domainfile.IntentStore, error) {
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

	return &FileIntentStore{db: db}, nil
}

func (s *FileIntentStore) Enqueue(ctx context.Context, intent domainfile.Intent) error {
	if intent.ID == "" {
		return errors.New("intent id is required")
	}
	nodeType := domainfile.NormalizeNodeType(intent.NodeType)
	if nodeType == "" {
		nodeType = domainfile.NodeTypeFile
	}
	row := FileIntent{
		IntentID:    intent.ID,
		Op:          string(intent.Op),
		AbsPath:     intent.AbsPath,
		OwnerUIN:    intent.Actor.OwnerUIN,
		UIN:         intent.Actor.UIN,
		AppID:       intent.Actor.AppID,
		WorkspaceID: intent.Actor.WorkspaceID,
		NodeType:    nodeType,
		Status:      string(domainfile.IntentPending),
	}
	// On retry with the same intent_id, reset it back to pending.
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "intent_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"op":         row.Op,
				"abs_path":   row.AbsPath,
				"node_type":  row.NodeType,
				"status":     string(domainfile.IntentPending),
				"last_error": "",
				"updated_at": gorm.Expr("CURRENT_TIMESTAMP(3)"),
			}),
		}).
		Create(&row).
		Error
}

func (s *FileIntentStore) MarkDone(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).
		Model(&FileIntent{}).
		Where("intent_id = ?", id).
		Updates(map[string]any{
			"status":     string(domainfile.IntentDone),
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP(3)"),
		}).
		Error
}

func (s *FileIntentStore) MarkAborted(ctx context.Context, id, reason string) error {
	return s.db.WithContext(ctx).
		Model(&FileIntent{}).
		Where("intent_id = ?", id).
		Updates(map[string]any{
			"status":     string(domainfile.IntentAborted),
			"last_error": truncateError(reason),
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP(3)"),
		}).
		Error
}

func (s *FileIntentStore) IncrementAttempt(ctx context.Context, id, lastErr string) error {
	return s.db.WithContext(ctx).
		Model(&FileIntent{}).
		Where("intent_id = ?", id).
		Updates(map[string]any{
			"attempts":   gorm.Expr("attempts + 1"),
			"last_error": truncateError(lastErr),
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP(3)"),
		}).
		Error
}

func (s *FileIntentStore) ListPending(ctx context.Context, limit int) ([]domainfile.Intent, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []FileIntent
	if err := s.db.WithContext(ctx).
		Where("status = ?", string(domainfile.IntentPending)).
		Order("id ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domainfile.Intent, 0, len(rows))
	for _, row := range rows {
		out = append(out, intentFromRow(row))
	}
	return out, nil
}

func intentFromRow(row FileIntent) domainfile.Intent {
	return domainfile.Intent{
		ID:      row.IntentID,
		Op:      domainfile.IntentOp(row.Op),
		AbsPath: row.AbsPath,
		Actor: domainfile.NewNodeActor(
			row.OwnerUIN, row.UIN, row.AppID, row.WorkspaceID,
		),
		NodeType:  row.NodeType,
		Status:    domainfile.IntentStatus(row.Status),
		Attempts:  row.Attempts,
		LastError: row.LastError,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func truncateError(msg string) string {
	const max = 512
	if len(msg) > max {
		return msg[:max]
	}
	return msg
}
