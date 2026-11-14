package controller

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

// Returns a claimed request, or ends the response for a replay/conflict.
// Scope includes the API key and exact operation, so no other token receives
// a task through this shortcut.
func claimZhiqiRequest(c *gin.Context, info *relaycommon.RelayInfo) (*model.ZhiqiIdempotency, bool) {
	key := c.GetHeader("Idempotency-Key")
	if c.GetString("platform") != "zhiqi" || key == "" {
		return nil, false
	}
	if len(key) > 128 || strings.TrimSpace(key) != key || strings.ContainsAny(key, "\r\n\t") {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_idempotency_key", "message": "Idempotency-Key must contain 1 to 128 non-whitespace bytes"}})
		return nil, true
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		zhiqiError(c, 400, "invalid_request_body")
		return nil, true
	}
	raw, err := storage.Bytes()
	if err != nil || len(raw) > 1<<20 {
		zhiqiError(c, 400, "invalid_request_body")
		return nil, true
	}
	var body map[string]any
	if common.Unmarshal(raw, &body) != nil || body == nil {
		zhiqiError(c, 400, "invalid_request_body")
		return nil, true
	}
	canonical, err := common.Marshal(body)
	if err != nil {
		zhiqiError(c, 400, "invalid_request_body")
		return nil, true
	}
	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	if info.PublicTaskID == "" {
		info.PublicTaskID = model.GenerateTaskID()
	}
	claim := &model.ZhiqiIdempotency{KeyHash: fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s:%s", info.UserId, info.TokenId, c.Request.URL.Path, key)))), PayloadHash: fmt.Sprintf("%x", sha256.Sum256(canonical)), TaskID: info.PublicTaskID}
	created, err := model.ClaimZhiqiIdempotency(c.Request.Context(), claim)
	if err != nil {
		zhiqiError(c, 409, "idempotency_conflict")
		return nil, true
	}
	if created {
		return claim, false
	}
	var task model.Task
	if err = model.DB.WithContext(c.Request.Context()).Where("task_id = ? AND user_id = ? AND platform = ?", claim.TaskID, info.UserId, "zhiqi").First(&task).Error; err != nil {
		c.Header("Retry-After", "2")
		zhiqiError(c, 409, "idempotency_request_in_progress")
		return nil, true
	}
	presentTaskSubmission(c, &taskSubmissionOutcome{Task: &task, RelayInfo: info, Result: &relay.TaskSubmitResult{Platform: "zhiqi", Quota: task.Quota}})
	return nil, true
}
