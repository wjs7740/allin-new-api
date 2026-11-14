package relay

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// Persist the reserved intent before the upstream can accept work. A process
// exit leaves a visible task for reconciliation, never an automatic re-POST.
func prepareZhiqiSubmission(c *gin.Context, info *relaycommon.RelayInfo, payload string) error {
	task := model.InitTask("zhiqi", info)
	task.Status = model.TaskStatusQueued
	task.Action = info.Action
	task.Quota = info.PriceData.Quota
	task.FailReason = ""
	task.PrivateData.BillingSource = info.BillingSource
	task.PrivateData.SubscriptionId = info.SubscriptionId
	task.PrivateData.TokenId = info.TokenId
	task.PrivateData.NodeName = common.NodeName
	task.PrivateData.Execution = service.TaskExecutionSnapshotFromContext(c)
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice: info.PriceData.ModelPrice, GroupRatio: info.PriceData.GroupRatioInfo.GroupRatio,
		ModelRatio: info.PriceData.ModelRatio, OtherRatios: info.PriceData.OtherRatios(),
		OriginModelName: info.OriginModelName, TieredSnapshot: info.TieredBillingSnapshot,
	}
	task.SetData(map[string]any{"id": task.TaskID, "object": info.Action + ".task", "model": info.OriginModelName, "status": "queued", "submission_state": "queued"})
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 10*time.Second)
	defer cancel()
	if err := model.InsertZhiqiSubmission(ctx, task, payload); err != nil {
		return err
	}
	c.Set("zhiqi_submit_intent", task)
	c.Set("zhiqi_submission_reserved", true)
	return nil
}

func queuedZhiqiSubmission(c *gin.Context, info *relaycommon.RelayInfo) *TaskSubmitResult {
	value, _ := c.Get("zhiqi_submit_intent")
	task := value.(*model.Task)
	return &TaskSubmitResult{Platform: "zhiqi", Quota: info.PriceData.Quota, TaskData: task.Data, Immediate: &relaycommon.TaskInfo{Status: model.TaskStatusQueued, Progress: "0%"}}
}
