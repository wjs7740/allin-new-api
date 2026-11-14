package service

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZhiqiLibraryReferenceIsolation(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.ZhiqiAsset{}))
	t.Cleanup(func() { model.DB.Where("owner_id IN ?", []int{831, 832}).Delete(&model.ZhiqiAsset{}) })
	fixtures := []model.ZhiqiAsset{
		{ID: "aia_owned", ResourceType: "asset", OwnerID: 831, RequestHash: "owned", Scope: "private", ChannelID: 71, Kind: "image", State: "active", UpstreamID: "upstream-owned"},
		{ID: "aia_other", ResourceType: "asset", OwnerID: 832, RequestHash: "other", Scope: "private", ChannelID: 71, Kind: "image", State: "active", UpstreamID: "upstream-other"},
		{ID: "aia_public", ResourceType: "asset", OwnerID: 832, RequestHash: "public", Scope: "public", ChannelID: 71, Kind: "video", State: "active", UpstreamID: "upstream-public"},
	}
	require.NoError(t, model.DB.Create(&fixtures).Error)
	resolved, err := ResolveZhiqiLibraryAssets(t.Context(), 831, 71, map[string]any{"images": []any{"aia_owned"}, "videos": []any{"aia_public"}})
	require.NoError(t, err)
	assert.Equal(t, []any{"upstream-owned"}, resolved["images"])
	assert.Equal(t, []any{"upstream-public"}, resolved["videos"])
	for _, input := range []map[string]any{
		{"images": []any{"aia_other"}}, {"images": []any{"upstream-owned"}}, {"images": []any{"aia_public"}}, {"unknown": []any{}},
	} {
		_, err := ResolveZhiqiLibraryAssets(t.Context(), 831, 71, input)
		require.Error(t, err)
	}
	_, err = ResolveZhiqiLibraryAssets(t.Context(), 831, 72, map[string]any{"images": []any{"aia_owned"}})
	require.Error(t, err)
}

func TestZhiqiRequestCredentialBoundary(t *testing.T) {
	InitHttpClient()
	hits := 0
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; t.Error("credentials must not follow redirects") }))
	defer other.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer fixture-merchant-key", r.Header.Get("Authorization"))
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", other.URL)
			w.WriteHeader(307)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
	}))
	defer upstream.Close()
	base := upstream.URL
	channel := &model.Channel{Key: "fixture-merchant-key", BaseURL: &base}
	response, status, err := ZhiqiRequest(t.Context(), channel, http.MethodGet, "/v1/models", nil)
	require.NoError(t, err)
	assert.Equal(t, 200, status)
	assert.Equal(t, true, response["success"])
	_, _, err = ZhiqiRequest(t.Context(), channel, http.MethodGet, "/redirect", nil)
	require.Error(t, err)
	assert.Zero(t, hits)
	assert.NotContains(t, err.Error(), channel.Key)
}

func TestZhiqiPollingAndDeliveryKeepConsumedCharge(t *testing.T) {
	truncate(t)
	seedUser(t, 833, 900)
	seedChannel(t, 733)
	seedChargedAccounting(t, 833, 733, 0, 100, 1)
	task := makeTask(833, 733, 100, 0, BillingSourceWallet, 0)
	task.Platform = "zhiqi"
	task.SubmitTime = time.Now().Add(-100 * time.Hour).Unix()
	task.PrivateData.UpstreamTaskID = "remote-task"
	require.NoError(t, task.Insert())
	oldLimit := constant.TaskPollMaxFailures
	constant.TaskPollMaxFailures = 1
	t.Cleanup(func() { constant.TaskPollMaxFailures = oldLimit })
	require.NoError(t, recordPollFailure(t.Context(), nil, task, task.Status, pollClassTransient, 503, ""))
	sweepTimedOutTasks(t.Context())
	var stored model.Task
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.TaskStatus(model.TaskStatusInProgress), stored.Status)
	assert.Equal(t, 100, stored.Quota)
	stored.SetData(map[string]any{"error": map[string]any{"code": "media_storage_failed"}})
	assert.True(t, RefundTaskQuota(t.Context(), &stored, "delivery failed"))
	var user model.User
	require.NoError(t, model.DB.First(&user, 833).Error)
	assert.Equal(t, 900, user.Quota)
	assert.Equal(t, 100, user.UsedQuota)
	assert.Equal(t, 100, stored.Quota)
	assert.Equal(t, common.UserStatusEnabled, user.Status)
}

func TestZhiqiPublicViewPreservesLocalModelAndIdentity(t *testing.T) {
	task := &model.Task{TaskID: "task_local", Platform: "zhiqi", Action: "video", Properties: model.Properties{OriginModelName: "public-alias"}, PrivateData: model.TaskPrivateData{UpstreamTaskID: "remote"}}
	task.SetData(map[string]any{"id": "remote", "model": "private-model", "status": "succeeded"})
	view, err := BuildTaskPluginView(task)
	require.NoError(t, err)
	data := view.Data.(map[string]any)
	assert.Equal(t, "task_local", data["id"])
	assert.Equal(t, "public-alias", data["model"])
	assert.Equal(t, "video.task", data["object"])
	assert.Contains(t, string(task.Data), "private-model", "persisted upstream snapshot remains intact")
}

func TestZhiqiDurableQueueSendsOnceAndRetainsUncertainCharges(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.ZhiqiSubmission{}, &model.ZhiqiReceipt{}))
	t.Cleanup(func() { model.DB.Where("channel_id = ?", 734).Delete(&model.ZhiqiReceipt{}) })
	t.Cleanup(func() { model.DB.Where("task_id LIKE ?", "task_queue_%").Delete(&model.ZhiqiSubmission{}) })
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		assert.Equal(t, "Bearer fixture-queue-key", r.Header.Get("Authorization"))
		var body map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &body))
		w.Header().Set("Content-Type", "application/json")
		if body["prompt"] == "uncertain" {
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"error":{"code":"busy"}}`))
			return
		}
		if body["prompt"] == "rejected" {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":{"code":"invalid"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"upstream-queue-task","object":"video.task","status":"queued","model":"queue-video"}`))
	}))
	defer upstream.Close()
	InitHttpClient()
	seedUser(t, 834, 700)
	base, setting := upstream.URL, `{"task_plugin_key":"zhiqi"}`
	channel := model.Channel{Id: 734, Type: constant.ChannelTypeTaskPlugin, Status: common.ChannelStatusEnabled, Key: "fixture-queue-key", BaseURL: &base, Setting: &setting}
	require.NoError(t, model.DB.Create(&channel).Error)
	for _, prompt := range []string{"success", "uncertain", "rejected"} {
		task := &model.Task{TaskID: "task_queue_" + prompt, Platform: "zhiqi", Action: "video", Status: model.TaskStatusQueued, ChannelId: 734, UserId: 834, Quota: 100,
			PrivateData: model.TaskPrivateData{Key: "fixture-queue-key", BillingSource: BillingSourceWallet}}
		payload, err := common.Marshal(map[string]any{"model": "queue-video", "prompt": prompt, "duration": 6})
		require.NoError(t, err)
		require.NoError(t, model.InsertZhiqiSubmission(t.Context(), task, string(payload)))
		// A persisted intent alone is never sent before billing activates it.
		if prompt == "success" {
			RunZhiqiSubmissionQueue(t.Context())
			assert.Zero(t, hits.Load())
		}
		require.NoError(t, model.ActivateZhiqiSubmission(t.Context(), task.TaskID))
	}
	seedChargedAccounting(t, 834, 734, 0, 300, 3)
	RunZhiqiSubmissionQueue(t.Context())
	assert.Equal(t, int32(3), hits.Load())
	RunZhiqiSubmissionQueue(t.Context())
	assert.Equal(t, int32(3), hits.Load(), "neither rejection nor uncertain acceptance is submitted again")
	var success, uncertain, rejected model.Task
	require.NoError(t, model.DB.Where("task_id = ?", "task_queue_success").First(&success).Error)
	require.NoError(t, model.DB.Where("task_id = ?", "task_queue_uncertain").First(&uncertain).Error)
	require.NoError(t, model.DB.Where("task_id = ?", "task_queue_rejected").First(&rejected).Error)
	assert.Equal(t, "upstream-queue-task", success.PrivateData.UpstreamTaskID)
	assert.Equal(t, 100, uncertain.Quota)
	assert.Equal(t, model.TaskStatus(model.TaskStatusUnknown), uncertain.Status)
	assert.Zero(t, rejected.Quota)
	var user model.User
	require.NoError(t, model.DB.First(&user, 834).Error)
	assert.Equal(t, 800, user.Quota)
	assert.Equal(t, 200, user.UsedQuota)
}
