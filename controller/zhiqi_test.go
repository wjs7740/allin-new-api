package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZhiqiIdempotencyScopeAndPayload(t *testing.T) {
	setupTaskPluginControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.ZhiqiIdempotency{}))
	call := func(token int, body string) (*model.ZhiqiIdempotency, bool, *httptest.ResponseRecorder) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest("POST", "/v1/videos", strings.NewReader(body))
		c.Request.Header.Set("Idempotency-Key", "same-idempotency-key")
		c.Set("platform", "zhiqi")
		info := &relaycommon.RelayInfo{UserId: 91, TokenId: token, TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
		claim, handled := claimZhiqiRequest(c, info)
		return claim, handled, recorder
	}
	claim, handled, _ := call(1, `{"model":"video","prompt":"fox"}`)
	require.NotNil(t, claim)
	assert.False(t, handled)
	_, handled, processing := call(1, `{"prompt":"fox","model":"video"}`)
	assert.True(t, handled)
	assert.Equal(t, 409, processing.Code)
	task := model.Task{TaskID: claim.TaskID, UserId: 91, Platform: "zhiqi", Status: model.TaskStatusQueued}
	require.NoError(t, task.Insert())
	_, handled, replay := call(1, `{"prompt":"fox","model":"video"}`)
	assert.True(t, handled)
	assert.Equal(t, 200, replay.Code)
	assert.Contains(t, replay.Body.String(), claim.TaskID)
	_, handled, conflict := call(1, `{"model":"video","prompt":"another fox"}`)
	assert.True(t, handled)
	assert.Equal(t, 409, conflict.Code)
	other, handled, _ := call(2, `{"model":"video","prompt":"fox"}`)
	require.NotNil(t, other)
	assert.False(t, handled)
	assert.NotEqual(t, claim.TaskID, other.TaskID)
}

func TestZhiqiAssetControllerIsolationAndCatalog(t *testing.T) {
	setupTaskPluginControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.ZhiqiAsset{}))
	oldRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = oldRedis })
	for _, id := range []int{901, 902, 903} {
		role := common.RoleCommonUser
		if id == 903 {
			role = common.RoleRootUser
		}
		require.NoError(t, model.DB.Create(&model.User{Id: id, Username: fmt.Sprint(id), AffCode: fmt.Sprint(id), Group: "default", Role: role, Status: common.UserStatusEnabled}).Error)
	}
	service.InitHttpClient()
	mutations := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer merchant-fixture", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/aigc-assets/groups":
			mutations++
			var body map[string]any
			require.NoError(t, common.DecodeJson(r.Body, &body))
			assert.Equal(t, "private", body["scope"])
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"upstream-group","name":"private fixture","scope":"private","state":"active","lock_version":1}}`))
		case "/api/aigc-assets/assets":
			mutations++
			var body map[string]any
			require.NoError(t, common.DecodeJson(r.Body, &body))
			assert.Equal(t, "upstream-group", body["group_id"])
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"upstream-asset","name":"fox","kind":"image","state":"active","lock_version":2}}`))
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"real-video"},{"id":"real-image"}]}`))
		case "/api/pricing_new":
			_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
		default:
			t.Errorf("unexpected upstream request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	base, setting := upstream.URL, `{"task_plugin_key":"zhiqi"}`
	channel := model.Channel{Id: 701, Type: constant.ChannelTypeTaskPlugin, Status: common.ChannelStatusEnabled, Group: "default", Key: "merchant-fixture", BaseURL: &base, Setting: &setting}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, model.DB.Create(&model.Option{Key: "ZhiqiAssetChannelID", Value: "701"}).Error)
	engine := gin.New()
	engine.Use(func(c *gin.Context) { id, _ := strconv.Atoi(c.GetHeader("X-Fixture-User")); c.Set("id", id) })
	engine.POST("/groups", CreateZhiqiAsset)
	engine.GET("/groups", ListZhiqiAssets)
	engine.GET("/groups/:id", GetZhiqiAsset)
	engine.POST("/assets", CreateZhiqiAsset)
	engine.DELETE("/assets/:id", MutateZhiqiAsset)
	engine.POST("/catalog", PreviewZhiqiCatalog)
	call := func(method, path string, user int, body any) *httptest.ResponseRecorder {
		payload, err := common.Marshal(body)
		require.NoError(t, err)
		req := httptest.NewRequest(method, path, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Fixture-User", strconv.Itoa(user))
		out := httptest.NewRecorder()
		engine.ServeHTTP(out, req)
		return out
	}
	body := map[string]any{"name": "private fixture", "request_id": "same-fixture-request-id"}
	first := call("POST", "/groups", 901, body)
	require.Equal(t, 200, first.Code)
	var envelope struct {
		Success bool             `json:"success"`
		Data    model.ZhiqiAsset `json:"data"`
	}
	require.NoError(t, common.Unmarshal(first.Body.Bytes(), &envelope))
	require.True(t, envelope.Success)
	groupID := envelope.Data.ID
	assert.True(t, strings.HasPrefix(groupID, "aig_"))
	assert.NotContains(t, first.Body.String(), "upstream-group")
	duplicate := call("POST", "/groups", 901, body)
	assert.Equal(t, 200, duplicate.Code)
	assert.Equal(t, 1, mutations)
	assert.Equal(t, 404, call("GET", "/groups/"+groupID, 902, nil).Code)
	assert.Equal(t, 404, call("GET", "/groups/"+groupID, 903, nil).Code)
	listing := call("GET", "/groups", 902, nil)
	assert.NotContains(t, listing.Body.String(), groupID)
	assetBody := map[string]any{"group_id": groupID, "name": "fox", "kind": "image", "url": "https://cdn.example/fox.png", "request_id": "asset-fixture-request-id"}
	assert.Equal(t, 404, call("POST", "/assets", 902, assetBody).Code)
	assetResponse := call("POST", "/assets", 901, assetBody)
	require.Equal(t, 200, assetResponse.Code)
	require.NoError(t, common.Unmarshal(assetResponse.Body.Bytes(), &envelope))
	assert.NotContains(t, assetResponse.Body.String(), "upstream-asset")
	assert.Equal(t, 404, call("DELETE", "/assets/"+envelope.Data.ID+"?confirm=true", 903, map[string]any{"lock_version": 1}).Code)
	assert.Equal(t, 2, mutations)
	// Refreshing a resource during an active mutation must not release its
	// version guard and allow a second upstream mutation.
	require.NoError(t, model.DB.Model(&model.ZhiqiAsset{}).Where("id = ?", groupID).Updates(map[string]any{"state": "updating", "updated_time": common.GetTimestamp()}).Error)
	assert.Equal(t, 409, call("GET", "/groups/"+groupID+"?refresh=true", 901, nil).Code)
	catalog := call("POST", "/catalog", 903, map[string]any{"channel_id": 701, "video_models": []string{"real-video"}, "image_models": []string{"real-image"}})
	require.Equal(t, 200, catalog.Code)
	assert.Contains(t, catalog.Body.String(), "plugin_source")
	assert.NotContains(t, catalog.Body.String(), "merchant-fixture")
	assert.Equal(t, 400, call("POST", "/catalog", 903, map[string]any{"channel_id": 701, "video_models": []string{"invented-model"}}).Code)
	for _, selection := range []map[string]any{
		{"channel_id": 701, "image_models": []string{"real-image"}},
		{"channel_id": 701, "video_models": []string{"real-video"}},
	} {
		response := call("POST", "/catalog", 903, selection)
		assert.Equal(t, 200, response.Code)
		assert.Contains(t, response.Body.String(), `"success":true`)
	}
}
