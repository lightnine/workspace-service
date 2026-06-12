package mysql

import (
	"context"
	"time"

	appconfig "git.woa.com/leondli/workspace-service/internal/config"
	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FileNodeStore struct {
	db *gorm.DB
}

type FileNode struct {
	ID          uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	InodeID     uint64         `gorm:"column:inode_id;uniqueIndex:uk_ws_file_node_inode_id"`
	OwnerUIN    string         `gorm:"column:owner_uin"`
	UIN         string         `gorm:"column:uin"`
	AppID       string         `gorm:"column:app_id"`
	WorkspaceID string         `gorm:"column:workspace_id"`
	NodeType    string         `gorm:"column:node_type"`
	CreatedAt   time.Time      `gorm:"column:created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index:idx_ws_file_node_deleted_at"`
}

func (FileNode) TableName() string {
	return "ws_file_node"
}

func NewOptionalFileNodeStore(cfg appconfig.MySQLConfig) (domainfile.NodeStore, error) {
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

	return &FileNodeStore{db: db}, nil
}

func fileNodeFromMutation(mutation domainfile.NodeMutation) FileNode {
	nodeType := domainfile.NormalizeNodeType(mutation.NodeType)
	if nodeType == "" {
		nodeType = domainfile.NodeTypeFile
	}
	return FileNode{
		InodeID:     mutation.InodeID,
		OwnerUIN:    mutation.Actor.OwnerUIN,
		UIN:         mutation.Actor.UIN,
		AppID:       mutation.Actor.AppID,
		WorkspaceID: mutation.Actor.WorkspaceID,
		NodeType:    nodeType,
	}
}

func scopeAssignments(actor domainfile.NodeActor, nodeType string) map[string]any {
	updates := map[string]any{
		"owner_uin":    actor.OwnerUIN,
		"uin":          actor.UIN,
		"app_id":       actor.AppID,
		"workspace_id": actor.WorkspaceID,
		"updated_at":   gorm.Expr("CURRENT_TIMESTAMP(3)"),
	}
	if t := domainfile.NormalizeNodeType(nodeType); t != "" {
		updates["node_type"] = t
	}
	return updates
}

func (s *FileNodeStore) UpsertCreatedOrUpdated(ctx context.Context, mutation domainfile.NodeMutation) error {
	node := fileNodeFromMutation(mutation)
	updates := scopeAssignments(mutation.Actor, mutation.NodeType)
	updates["deleted_at"] = nil

	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "inode_id"}},
			DoUpdates: clause.Assignments(updates),
		}).
		Create(&node).
		Error
}

func (s *FileNodeStore) MarkDeleted(ctx context.Context, mutation domainfile.NodeMutation) error {
	updates := scopeAssignments(mutation.Actor, "")
	updates["deleted_at"] = gorm.Expr("CURRENT_TIMESTAMP(3)")

	return s.db.WithContext(ctx).
		Model(&FileNode{}).
		Where("inode_id = ?", mutation.InodeID).
		Updates(updates).
		Error
}

func (s *FileNodeStore) LookupByInodeIDs(ctx context.Context, inodeIDs []uint64) (map[uint64]domainfile.NodeRecord, error) {
	out := make(map[uint64]domainfile.NodeRecord)
	if len(inodeIDs) == 0 {
		return out, nil
	}

	var rows []FileNode
	if err := s.db.WithContext(ctx).
		Where("inode_id IN ?", inodeIDs).
		Where("deleted_at IS NULL").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.InodeID] = domainfile.NodeRecord{
			InodeID:     row.InodeID,
			OwnerUIN:    row.OwnerUIN,
			UIN:         row.UIN,
			AppID:       row.AppID,
			WorkspaceID: row.WorkspaceID,
			NodeType:    row.NodeType,
		}
	}
	return out, nil
}
