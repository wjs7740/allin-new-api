package model

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestZhiqiAssetDatabaseContracts(t *testing.T) {
	cases := []struct {
		name      string
		dialector gorm.Dialector
	}{{"sqlite", sqlite.Open(filepath.Join(t.TempDir(), "assets.db"))}}
	if dsn := os.Getenv("ALLIN_TEST_MYSQL_DSN"); dsn != "" {
		cases = append(cases, struct {
			name      string
			dialector gorm.Dialector
		}{"mysql", mysql.Open(dsn)})
	}
	if dsn := os.Getenv("ALLIN_TEST_POSTGRES_DSN"); dsn != "" {
		cases = append(cases, struct {
			name      string
			dialector gorm.Dialector
		}{"postgres", postgres.Open(dsn)})
	}
	originalDB := DB
	originalType := common.MainDatabaseType()
	t.Cleanup(func() { DB = originalDB; common.SetMainDatabaseType(originalType) })
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			DB, err = gorm.Open(tc.dialector, &gorm.Config{})
			require.NoError(t, err)
			common.SetMainDatabaseType(common.DatabaseType(tc.name))
			sqlDB, err := DB.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			// A database with pre-existing upstream tables represents an upgrade.
			require.NoError(t, DB.AutoMigrate(&TaskPlugin{}, &Channel{}, &Task{}))
			legacy := TaskPlugin{Key: "legacy-" + uuid.NewString(), Version: "1.0.0", APIVersion: 1, Source: "fixture", SourceHash: "fixture"}
			require.NoError(t, DB.Create(&legacy).Error)
			for range 2 {
				require.NoError(t, DB.AutoMigrate(&ZhiqiAsset{}, &ZhiqiSubmission{}, &ZhiqiReceipt{}, &ZhiqiIdempotency{}))
			}
			var preserved TaskPlugin
			require.NoError(t, DB.First(&preserved, legacy.Id).Error)
			assert.Equal(t, legacy.Source, preserved.Source)
			request := uuid.NewString()
			group := &ZhiqiAsset{ID: "aig_" + uuid.NewString(), ResourceType: "group", OwnerID: 11, RequestHash: request, PayloadHash: "same", ChannelID: 7, Scope: "private", Name: "我的素材", State: "creating"}
			created, err := ReserveZhiqiAsset(t.Context(), group)
			require.NoError(t, err)
			require.True(t, created)
			duplicate := &ZhiqiAsset{ID: "aig_" + uuid.NewString(), OwnerID: 11, RequestHash: request, PayloadHash: "same"}
			created, err = ReserveZhiqiAsset(t.Context(), duplicate)
			require.NoError(t, err)
			assert.False(t, created)
			assert.Equal(t, group.ID, duplicate.ID)
			conflict := &ZhiqiAsset{ID: "aig_" + uuid.NewString(), OwnerID: 11, RequestHash: request, PayloadHash: "different"}
			_, err = ReserveZhiqiAsset(t.Context(), conflict)
			require.Error(t, err)
			_, err = GetZhiqiAsset(t.Context(), group.ID, 12)
			require.ErrorIs(t, err, gorm.ErrRecordNotFound)
			assert.False(t, group.ManageableBy(12, true), "admin cannot manage another user's private asset")
			group.State, group.UpstreamID, group.UpstreamVersion = "active", "upstream-group", 2
			require.NoError(t, SaveZhiqiAssetResult(t.Context(), group))
			require.NoError(t, ClaimZhiqiAssetMutation(t.Context(), group, group.LockVersion, "updating"))
			require.Error(t, ClaimZhiqiAssetMutation(t.Context(), duplicate, 1, "updating"))
			group.State = "active"
			require.NoError(t, SaveZhiqiAssetResult(t.Context(), group))
			for i, scope := range []string{"private", "public"} {
				asset := &ZhiqiAsset{ID: "aia_" + uuid.NewString(), ResourceType: "asset", OwnerID: 11, RequestHash: uuid.NewString(), PayloadHash: fmt.Sprint(i), ChannelID: 7, GroupID: group.ID, Scope: scope, Name: "asset", Kind: "image", State: "active"}
				_, err = ReserveZhiqiAsset(t.Context(), asset)
				require.NoError(t, err)
				assert.Equal(t, scope == "public", asset.ReadableBy(12))
				assert.Equal(t, scope == "public", asset.ManageableBy(12, true))
			}
			items, total, err := ListZhiqiAssets(t.Context(), 12, "asset", "", group.ID, "", "", "", 1, 20)
			require.NoError(t, err)
			require.Equal(t, int64(1), total)
			require.Len(t, items, 1)
			assert.Equal(t, "public", items[0].Scope)
			require.NoError(t, ClaimZhiqiAssetMutation(t.Context(), group, group.LockVersion, "deleting"))
			group.State = "deleted"
			require.NoError(t, SaveZhiqiAssetResult(t.Context(), group))
			_, total, err = ListZhiqiAssets(t.Context(), 11, "asset", "", group.ID, "", "", "", 1, 20)
			require.NoError(t, err)
			assert.Zero(t, total)
			for range 2 {
				require.NoError(t, DB.AutoMigrate(&ZhiqiAsset{}, &ZhiqiSubmission{}, &ZhiqiReceipt{}, &ZhiqiIdempotency{}))
			}
			var count int64
			require.NoError(t, DB.Model(&ZhiqiAsset{}).Where("id = ?", group.ID).Count(&count).Error)
			assert.Equal(t, int64(1), count)
			channel := Channel{Name: "receipt-" + uuid.NewString()}
			require.NoError(t, DB.Create(&channel).Error)
			pending := Task{TaskID: "task_" + uuid.NewString(), ChannelId: channel.Id, Platform: "zhiqi", Status: TaskStatusUnknown, Quota: 123}
			require.NoError(t, pending.Insert())
			pending.PrivateData.UpstreamTaskID = "receipt-" + uuid.NewString()
			pending.Status = TaskStatusQueued
			require.NoError(t, BindZhiqiTaskReceipt(t.Context(), &pending))
			require.Error(t, BindZhiqiTaskReceipt(t.Context(), &pending))
			other := Task{TaskID: "task_" + uuid.NewString(), ChannelId: channel.Id, Platform: "zhiqi", Status: TaskStatusUnknown, Quota: 321}
			require.NoError(t, other.Insert())
			other.PrivateData.UpstreamTaskID = pending.PrivateData.UpstreamTaskID
			require.Error(t, BindZhiqiTaskReceipt(t.Context(), &other))
			queued := Task{TaskID: "task_queue_" + uuid.NewString(), ChannelId: channel.Id, Platform: "zhiqi", Status: TaskStatusQueued, Quota: 50}
			require.NoError(t, InsertZhiqiSubmission(t.Context(), &queued, `{"model":"fixture","prompt":"fox"}`))
			jobs, err := ClaimZhiqiSubmissions(t.Context(), 10)
			require.NoError(t, err)
			assert.Empty(t, jobs)
			require.NoError(t, ActivateZhiqiSubmission(t.Context(), queued.TaskID))
			jobs, err = ClaimZhiqiSubmissions(t.Context(), 10)
			require.NoError(t, err)
			require.Len(t, jobs, 1)
			assert.Equal(t, queued.TaskID, jobs[0].TaskID)
			jobs, err = ClaimZhiqiSubmissions(t.Context(), 10)
			require.NoError(t, err)
			assert.Empty(t, jobs)
			require.NoError(t, DB.Model(&ZhiqiSubmission{}).Where("task_id = ?", queued.TaskID).Update("updated_at", 1).Error)
			jobs, err = ClaimZhiqiSubmissions(t.Context(), 10)
			require.NoError(t, err)
			assert.Empty(t, jobs)
			var stale ZhiqiSubmission
			require.NoError(t, DB.First(&stale, "task_id = ?", queued.TaskID).Error)
			assert.Equal(t, "uncertain", stale.State)
			assert.Empty(t, stale.Payload)
			claim := ZhiqiIdempotency{KeyHash: uuid.NewString(), PayloadHash: "payload-1", TaskID: queued.TaskID}
			claimed, err := ClaimZhiqiIdempotency(t.Context(), &claim)
			require.NoError(t, err)
			assert.True(t, claimed)
			replay := ZhiqiIdempotency{KeyHash: claim.KeyHash, PayloadHash: "payload-1", TaskID: "a-different-task"}
			claimed, err = ClaimZhiqiIdempotency(t.Context(), &replay)
			require.NoError(t, err)
			assert.False(t, claimed)
			assert.Equal(t, queued.TaskID, replay.TaskID)
			conflicting := ZhiqiIdempotency{KeyHash: claim.KeyHash, PayloadHash: "payload-2"}
			_, err = ClaimZhiqiIdempotency(t.Context(), &conflicting)
			require.Error(t, err)
		})
	}
}
