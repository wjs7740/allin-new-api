package service

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

// Durable claims allow multiple nodes to poll; only one sends a queued job.
// Stale in-flight claims require reconciliation and are never resubmitted.
func RunZhiqiSubmissionQueue(ctx context.Context) {
	jobs, err := model.ClaimZhiqiSubmissions(ctx, 4)
	if err != nil {
		logger.LogWarn(ctx, "Zhiqi submission queue is unavailable")
		return
	}
	var workers sync.WaitGroup
	for _, job := range jobs {
		workers.Go(func() { processZhiqiSubmission(ctx, job) })
	}
	workers.Wait()
}

func processZhiqiSubmission(ctx context.Context, job model.ZhiqiSubmission) {
	operationCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 55*time.Second)
	defer cancel()
	var task model.Task
	if err := model.DB.WithContext(operationCtx).Where("task_id = ? AND platform = ?", job.TaskID, "zhiqi").First(&task).Error; err != nil {
		return
	}
	if task.PrivateData.UpstreamTaskID != "" || task.Status == model.TaskStatusFailure || task.Status == model.TaskStatusSuccess {
		_ = model.FinishZhiqiSubmission(operationCtx, job.TaskID, "submitted")
		return
	}
	var body map[string]any
	if common.UnmarshalJsonStr(string(job.Payload), &body) != nil || body == nil {
		markZhiqiSubmissionUncertain(operationCtx, &task)
		return
	}
	channel, err := ZhiqiChannel(task.ChannelId)
	if err != nil {
		markZhiqiSubmissionUncertain(operationCtx, &task)
		return
	}
	if task.PrivateData.Key != "" {
		channel.Key = task.PrivateData.Key
		channel.Keys = nil
		channel.ChannelInfo.IsMultiKey = false
	}
	path := "/v1/videos"
	if task.Action == "image" {
		path = "/v1/images/generations/async"
	}
	response, status, err := ZhiqiRequest(operationCtx, channel, http.MethodPost, path, body)
	if err != nil {
		if status >= 400 && status < 500 && status != 408 && status != 429 {
			from := task.Status
			task.Status = model.TaskStatusFailure
			task.Progress = "100%"
			task.FinishTime = time.Now().Unix()
			task.FailReason = "Upstream rejected the generation request"
			task.SetData(map[string]any{"object": task.Action + ".task", "status": "failed", "error": map[string]any{"code": "generation_rejected", "message": task.FailReason}})
			if won, saveErr := task.UpdateWithStatus(from); saveErr == nil && won {
				RefundTaskQuota(operationCtx, &task, task.FailReason)
				_ = model.FinishZhiqiSubmission(operationCtx, job.TaskID, "rejected")
			}
			return
		}
		markZhiqiSubmissionUncertain(operationCtx, &task)
		return
	}
	id, valid := response["id"].(string)
	if !valid || id == "" || len(id) > 191 {
		markZhiqiSubmissionUncertain(operationCtx, &task)
		return
	}
	task.PrivateData.UpstreamTaskID = id
	task.Status = model.TaskStatusInProgress
	task.FailReason = ""
	task.SetData(response)
	if err = model.BindZhiqiTaskReceipt(operationCtx, &task); err != nil {
		_ = model.FinishZhiqiSubmission(operationCtx, job.TaskID, "uncertain")
		return
	}
	_ = model.FinishZhiqiSubmission(operationCtx, job.TaskID, "submitted")
}

func markZhiqiSubmissionUncertain(ctx context.Context, task *model.Task) {
	from := task.Status
	task.Status = model.TaskStatusUnknown
	task.FailReason = "Submission requires reconciliation; do not resubmit"
	task.SetData(map[string]any{"object": task.Action + ".task", "status": "in_progress", "submission_state": "reconciliation_required"})
	_, _ = task.UpdateWithStatus(from)
	_ = model.FinishZhiqiSubmission(ctx, task.TaskID, "uncertain")
}
