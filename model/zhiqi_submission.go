package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Only ready records may be claimed. The controller marks ready after the
// normal billing settlement and usage log, never before the client receipt.
type ZhiqiSubmission struct {
	TaskID    string   `gorm:"primaryKey;size:191"`
	Payload   LongText `json:"-"`
	State     string   `gorm:"size:24;index"`
	CreatedAt int64
	UpdatedAt int64 `gorm:"index"`
}

type ZhiqiReceipt struct {
	TaskID      string `gorm:"primaryKey;size:191"`
	ChannelID   int    `gorm:"uniqueIndex:uk_zhiqi_channel_receipt,priority:1"`
	ReceiptHash string `gorm:"size:64;uniqueIndex:uk_zhiqi_channel_receipt,priority:2"`
}

type ZhiqiIdempotency struct {
	KeyHash     string `gorm:"primaryKey;size:64"`
	PayloadHash string `gorm:"size:64"`
	TaskID      string `gorm:"size:191;index"`
}

func ClaimZhiqiIdempotency(ctx context.Context, claim *ZhiqiIdempotency) (bool, error) {
	if err := DB.WithContext(ctx).Create(claim).Error; err == nil {
		return true, nil
	} else {
		var existing ZhiqiIdempotency
		if lookupErr := DB.WithContext(ctx).First(&existing, "key_hash = ?", claim.KeyHash).Error; lookupErr != nil {
			return false, err
		}
		if existing.PayloadHash != claim.PayloadHash {
			return false, errors.New("idempotency key already used with a different request")
		}
		*claim = existing
		return false, nil
	}
}

func ReleaseZhiqiIdempotency(ctx context.Context, claim ZhiqiIdempotency) error {
	return DB.WithContext(ctx).Where("key_hash = ? AND task_id = ?", claim.KeyHash, claim.TaskID).Delete(&ZhiqiIdempotency{}).Error
}

func InsertZhiqiSubmission(ctx context.Context, task *Task, payload string) error {
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		now := time.Now().Unix()
		return tx.Create(&ZhiqiSubmission{TaskID: task.TaskID, Payload: LongText(payload), State: "holding", CreatedAt: now, UpdatedAt: now}).Error
	})
}

func ActivateZhiqiSubmission(ctx context.Context, taskID string) error {
	result := DB.WithContext(ctx).Model(&ZhiqiSubmission{}).Where("task_id = ? AND state = ?", taskID, "holding").Updates(map[string]any{"state": "queued", "updated_at": time.Now().Unix()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("submission is not awaiting activation")
	}
	return nil
}

func ClaimZhiqiSubmissions(ctx context.Context, limit int) ([]ZhiqiSubmission, error) {
	// A worker that died while talking to the upstream must never auto-POST
	// again. Its task remains available for a verified receipt to be linked.
	var stale []ZhiqiSubmission
	if err := DB.WithContext(ctx).Select("task_id").Where("state IN ? AND updated_at < ?", []string{"submitting", "holding"}, time.Now().Add(-2*time.Minute).Unix()).Limit(100).Find(&stale).Error; err != nil {
		return nil, err
	}
	for _, job := range stale {
		if err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&ZhiqiSubmission{}).Where("task_id = ? AND state IN ? AND updated_at < ?", job.TaskID, []string{"submitting", "holding"}, time.Now().Add(-2*time.Minute).Unix()).Updates(map[string]any{"state": "uncertain", "payload": ""})
			if result.Error != nil || result.RowsAffected == 0 {
				return result.Error
			}
			return tx.Model(&Task{}).Where("task_id = ? AND platform = ? AND status NOT IN ?", job.TaskID, "zhiqi", []string{TaskStatusSuccess, TaskStatusFailure}).Updates(map[string]any{"status": TaskStatusUnknown, "fail_reason": "Submission requires reconciliation; do not resubmit"}).Error
		}); err != nil {
			return nil, err
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var candidates []ZhiqiSubmission
	if err := DB.WithContext(ctx).Where("state = ?", "queued").Order("created_at ASC").Limit(limit).Find(&candidates).Error; err != nil {
		return nil, err
	}
	claimed := make([]ZhiqiSubmission, 0, len(candidates))
	for _, candidate := range candidates {
		result := DB.WithContext(ctx).Model(&ZhiqiSubmission{}).Where("task_id = ? AND state = ?", candidate.TaskID, "queued").Updates(map[string]any{"state": "submitting", "updated_at": time.Now().Unix()})
		if result.Error != nil {
			return claimed, result.Error
		}
		if result.RowsAffected == 1 {
			claimed = append(claimed, candidate)
		}
	}
	return claimed, nil
}

func FinishZhiqiSubmission(ctx context.Context, taskID, state string) error {
	return DB.WithContext(ctx).Model(&ZhiqiSubmission{}).Where("task_id = ? AND state = ?", taskID, "submitting").Updates(map[string]any{"state": state, "payload": "", "updated_at": time.Now().Unix()}).Error
}
