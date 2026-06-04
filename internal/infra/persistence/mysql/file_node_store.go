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
	ID        uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	InodeID   uint64         `gorm:"column:inode_id;uniqueIndex:uk_file_node_inode_id"`
	OwnerUIN  string         `gorm:"column:owner_uin"`
	UIN       string         `gorm:"column:uin"`
	CreatedAt time.Time      `gorm:"column:created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index:idx_file_node_deleted_at"`
}

func (FileNode) TableName() string {
	return "file_node"
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

func (s *FileNodeStore) UpsertCreatedOrUpdated(ctx context.Context, mutation domainfile.NodeMutation) error {
	node := FileNode{
		InodeID:  mutation.InodeID,
		OwnerUIN: mutation.Actor.OwnerUIN,
		UIN:      mutation.Actor.UIN,
	}

	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "inode_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"owner_uin":  mutation.Actor.OwnerUIN,
				"uin":        mutation.Actor.UIN,
				"updated_at": gorm.Expr("CURRENT_TIMESTAMP(3)"),
				"deleted_at": nil,
			}),
		}).
		Create(&node).
		Error
}

func (s *FileNodeStore) MarkDeleted(ctx context.Context, mutation domainfile.NodeMutation) error {
	return s.db.WithContext(ctx).
		Model(&FileNode{}).
		Where("inode_id = ?", mutation.InodeID).
		Updates(map[string]any{
			"owner_uin":  mutation.Actor.OwnerUIN,
			"uin":        mutation.Actor.UIN,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP(3)"),
			"deleted_at": gorm.Expr("CURRENT_TIMESTAMP(3)"),
		}).
		Error
}
