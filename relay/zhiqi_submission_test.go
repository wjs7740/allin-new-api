package relay

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZhiqiSubmissionIntentSurvivesUncertainAcceptance(t *testing.T) {
	database := setupRelayChannelDB(t)
	require.NoError(t, database.AutoMigrate(&model.Task{}, &model.ZhiqiSubmission{}))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/videos", nil)
	info := &relaycommon.RelayInfo{UserId: 51, OriginModelName: "video-alias", UsingGroup: "default", TokenId: 61,
		ChannelMeta:   &relaycommon.ChannelMeta{ChannelId: 71, UpstreamModelName: "real-video"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: "video", PublicTaskID: "task_fixture"},
		PriceData:     types.PriceData{Quota: 123}, BillingSource: "wallet"}
	require.NoError(t, prepareZhiqiSubmission(c, info, `{"model":"real-video","prompt":"fox"}`))
	var saved model.Task
	require.NoError(t, database.Where("task_id = ?", "task_fixture").First(&saved).Error)
	assert.Equal(t, 123, saved.Quota)
	assert.Empty(t, saved.PrivateData.UpstreamTaskID)
	assert.Equal(t, 61, saved.PrivateData.TokenId)
	assert.True(t, c.GetBool("zhiqi_submission_reserved"))
	result := queuedZhiqiSubmission(c, info)
	assert.Equal(t, 123, result.Quota)
	assert.Empty(t, result.UpstreamTaskID)
	assert.Contains(t, string(result.TaskData), "queued")
	var total int64
	require.NoError(t, database.Model(&model.Task{}).Count(&total).Error)
	assert.Equal(t, int64(1), total)
}
