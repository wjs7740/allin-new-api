package model

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
)

var zhiqiReceiptMu sync.Mutex

func BindZhiqiTaskReceipt(ctx context.Context, task *Task) error {
	zhiqiReceiptMu.Lock()
	defer zhiqiReceiptMu.Unlock()
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var channel Channel
		if err := lockForUpdate(tx).First(&channel, task.ChannelId).Error; err != nil {
			return err
		}
		var current Task
		if err := lockForUpdate(tx).First(&current, task.ID).Error; err != nil {
			return err
		}
		if current.PrivateData.UpstreamTaskID != "" || current.Status == TaskStatusSuccess || current.Status == TaskStatusFailure {
			return errors.New("task already reconciled")
		}
		receipt := ZhiqiReceipt{TaskID: task.TaskID, ChannelID: task.ChannelId, ReceiptHash: fmt.Sprintf("%x", sha256.Sum256([]byte(task.PrivateData.UpstreamTaskID)))}
		if err := tx.Create(&receipt).Error; err != nil {
			return errors.New("upstream receipt already belongs to another task")
		}
		return tx.Model(&current).Select("status", "fail_reason", "data", "private_data").Updates(task).Error
	})
}

// ZhiqiAsset maps one local resource to a resource in the merchant account.
// Never use upstream listing APIs to determine local ownership.
type ZhiqiAsset struct {
	ID              string `json:"id" gorm:"primaryKey;size:64"`
	ResourceType    string `json:"-" gorm:"size:16;index"`
	OwnerID         int    `json:"-" gorm:"index;uniqueIndex:uk_zhiqi_asset_request,priority:1"`
	RequestHash     string `json:"-" gorm:"size:64;uniqueIndex:uk_zhiqi_asset_request,priority:2"`
	PayloadHash     string `json:"-" gorm:"size:64"`
	ChannelID       int    `json:"-" gorm:"index"`
	UpstreamID      string `json:"-" gorm:"size:191"`
	UpstreamVersion int64  `json:"-"`
	GroupID         string `json:"group_id,omitempty" gorm:"size:64;index"`
	Scope           string `json:"scope" gorm:"size:16;index"`
	Name            string `json:"name" gorm:"size:256"`
	Description     string `json:"description,omitempty" gorm:"type:text"`
	Kind            string `json:"kind,omitempty" gorm:"size:16"`
	State           string `json:"state" gorm:"size:24;index"`
	ErrorCode       string `json:"error_code,omitempty" gorm:"size:64"`
	LockVersion     int64  `json:"lock_version"`
	CreatedTime     int64  `json:"created_time"`
	UpdatedTime     int64  `json:"updated_time"`
	CanManage       bool   `json:"can_manage" gorm:"-"`
}

func (asset ZhiqiAsset) ReadableBy(userID int) bool {
	return userID > 0 && (asset.Scope == "public" || asset.OwnerID == userID)
}

func (asset ZhiqiAsset) ManageableBy(userID int, admin bool) bool {
	if userID <= 0 {
		return false
	}
	if asset.Scope == "public" {
		return admin
	}
	return asset.OwnerID == userID
}

func GetZhiqiAsset(ctx context.Context, id string, userID int) (*ZhiqiAsset, error) {
	var asset ZhiqiAsset
	err := DB.WithContext(ctx).Where("id = ? AND (owner_id = ? OR scope = ?)", id, userID, "public").First(&asset).Error
	return &asset, err
}

func ListZhiqiAssets(ctx context.Context, userID int, resourceType, scope, groupID, keyword, kind, state string, page, pageSize int) ([]ZhiqiAsset, int64, error) {
	query := DB.WithContext(ctx).Model(&ZhiqiAsset{}).Where("resource_type = ? AND state <> ? AND (owner_id = ? OR scope = ?)", resourceType, "deleted", userID, "public")
	if scope != "" {
		query = query.Where("scope = ?", scope)
	}
	if groupID != "" {
		query = query.Where("group_id = ?", groupID)
	}
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	if state != "" {
		query = query.Where("state = ?", state)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := []ZhiqiAsset{}
	err := query.Order("created_time DESC, id ASC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&items).Error
	return items, total, err
}

// ReserveZhiqiAsset commits the local identity before any upstream mutation.
// A retry returns the same row; a different payload under the same key fails.
func ReserveZhiqiAsset(ctx context.Context, asset *ZhiqiAsset) (bool, error) {
	asset.CreatedTime = time.Now().Unix()
	asset.UpdatedTime = asset.CreatedTime
	asset.LockVersion = 1
	err := DB.WithContext(ctx).Create(asset).Error
	if err == nil {
		return true, nil
	}
	var existing ZhiqiAsset
	if lookupErr := DB.WithContext(ctx).Where("owner_id = ? AND request_hash = ?", asset.OwnerID, asset.RequestHash).First(&existing).Error; lookupErr != nil {
		return false, err
	}
	if existing.PayloadHash != asset.PayloadHash {
		return false, errors.New("request_id was already used for another operation")
	}
	*asset = existing
	return false, nil
}

// ClaimZhiqiAssetMutation is a compare-and-swap, so only one request can send
// an upstream mutation for this resource version, on every database dialect.
func ClaimZhiqiAssetMutation(ctx context.Context, asset *ZhiqiAsset, expected int64, state string) error {
	if expected != asset.LockVersion || asset.State == "creating" || asset.State == "updating" || asset.State == "deleting" || asset.State == "unknown" || asset.State == "deleted" {
		return errors.New("resource changed or requires reconciliation; refresh before retrying")
	}
	result := DB.WithContext(ctx).Model(&ZhiqiAsset{}).Where("id = ? AND lock_version = ? AND state = ?", asset.ID, expected, asset.State).
		Updates(map[string]any{"state": state, "lock_version": expected + 1, "updated_time": time.Now().Unix()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("resource version conflict")
	}
	asset.LockVersion++
	asset.State = state
	return nil
}

func SaveZhiqiAssetResult(ctx context.Context, asset *ZhiqiAsset) error {
	asset.UpdatedTime = time.Now().Unix()
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ZhiqiAsset{}).Where("id = ? AND lock_version = ?", asset.ID, asset.LockVersion).Updates(map[string]any{
			"upstream_id": asset.UpstreamID, "upstream_version": asset.UpstreamVersion, "name": asset.Name, "description": asset.Description,
			"state": asset.State, "error_code": asset.ErrorCode, "updated_time": asset.UpdatedTime, "lock_version": gorm.Expr("lock_version + 1"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("resource version conflict")
		}
		if asset.ResourceType == "group" && asset.State == "deleted" {
			return tx.Model(&ZhiqiAsset{}).Where("group_id = ?", asset.ID).Updates(map[string]any{"state": "deleted", "updated_time": asset.UpdatedTime, "lock_version": gorm.Expr("lock_version + 1")}).Error
		}
		return nil
	})
	if err == nil {
		asset.LockVersion++
	}
	return err
}
