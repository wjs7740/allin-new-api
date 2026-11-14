package controller

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/plugins"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// GetZhiqiPricing exposes only local retail entries the current API key can
// route to. Upstream catalog costs are restricted to the root preview.
func GetZhiqiPricing(c *gin.Context) {
	groups, err := getModelListGroups(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enabled := service.GetGroupsEnabledModels(groups.ownerGroups)
	limits, _ := c.Get("token_model_limit")
	allowed, _ := limits.(map[string]bool)
	prices := []model.Pricing{}
	for _, price := range model.GetPricing() {
		if !slices.Contains(enabled, price.ModelName) {
			continue
		}
		if common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled) && !allowed[price.ModelName] && !allowed[ratio_setting.RoutingMatchModelName(price.ModelName)] {
			continue
		}
		price.EnableGroup = slices.DeleteFunc(slices.Clone(price.EnableGroup), func(group string) bool { return !slices.Contains(groups.ownerGroups, group) })
		prices = append(prices, price)
	}
	groupRatios := map[string]float64{}
	for _, group := range groups.ownerGroups {
		ratio := ratio_setting.GetGroupRatio(group)
		if special, ok := ratio_setting.GetGroupGroupRatio(groups.userGroup, group); ok {
			ratio = special
		}
		groupRatios[group] = ratio
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": prices, "vendors": model.GetVendors(), "group_ratio": groupRatios, "supported_endpoint": model.GetSupportedEndpointMap(), "pricing_version": "allin-zhiqi-v1"})
}

func GetZhiqiConfiguration(c *gin.Context) {
	var option model.Option
	channelID := 0
	if model.DB.WithContext(c.Request.Context()).Where(&model.Option{Key: "ZhiqiAssetChannelID"}).First(&option).Error == nil {
		channelID, _ = strconv.Atoi(option.Value)
	}
	common.ApiSuccess(c, gin.H{"asset_channel_id": channelID})
}

func SetZhiqiConfiguration(c *gin.Context) {
	var request struct {
		AssetChannelID int `json:"asset_channel_id"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.AssetChannelID < 0 {
		zhiqiError(c, 400, "invalid_configuration")
		return
	}
	if request.AssetChannelID > 0 {
		channel, err := service.ZhiqiChannel(request.AssetChannelID)
		if err != nil || len(channel.GetKeys()) != 1 {
			zhiqiError(c, 400, "select_a_single_key_zhiqi_channel")
			return
		}
	}
	if err := model.UpdateOption("ZhiqiAssetChannelID", strconv.Itoa(request.AssetChannelID)); err != nil {
		common.ApiError(c, err)
		return
	}
	GetZhiqiConfiguration(c)
}

// PreviewZhiqiCatalog returns a reviewable plugin source. Installation and
// activation deliberately use the existing plugin administration endpoints.
func PreviewZhiqiCatalog(c *gin.Context) {
	var request struct {
		ChannelID   int      `json:"channel_id"`
		VideoModels []string `json:"video_models"`
		ImageModels []string `json:"image_models"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		zhiqiError(c, 400, "invalid_catalog_request")
		return
	}
	channel, err := service.ZhiqiChannel(request.ChannelID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	models, _, err := service.ZhiqiRequest(c.Request.Context(), channel, http.MethodGet, "/v1/models", nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	entries, ok := models["data"].([]any)
	if !ok {
		zhiqiError(c, 502, "invalid_upstream_model_catalog")
		return
	}
	available := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if item, valid := entry.(map[string]any); valid {
			if id, valid := item["id"].(string); valid && id != "" {
				available[id] = true
			}
		}
	}
	if len(request.VideoModels)+len(request.ImageModels) > 2000 {
		zhiqiError(c, 400, "too_many_catalog_models")
		return
	}
	seen := map[string]bool{}
	for _, name := range append(slices.Clone(request.VideoModels), request.ImageModels...) {
		if !available[name] || seen[strings.ToLower(name)] {
			zhiqiError(c, 400, "selected_model_missing_or_duplicated")
			return
		}
		seen[strings.ToLower(name)] = true
	}
	pricing, pricingStatus, pricingErr := service.ZhiqiRequest(c.Request.Context(), channel, http.MethodGet, "/api/pricing_new", nil)
	result := gin.H{"models": entries, "pricing": pricing, "pricing_available": pricingErr == nil, "pricing_status": pricingStatus}
	if len(seen) > 0 {
		if request.VideoModels == nil {
			request.VideoModels = []string{}
		}
		if request.ImageModels == nil {
			request.ImageModels = []string{}
		}
		catalog, marshalErr := common.Marshal(map[string][]string{"video": request.VideoModels, "image": request.ImageModels})
		if marshalErr != nil {
			common.ApiError(c, marshalErr)
			return
		}
		source, sourceErr := plugins.Source("zhiqi")
		if sourceErr != nil {
			common.ApiError(c, sourceErr)
			return
		}
		start := strings.Index(source, "const ZHIQI_CATALOG = ")
		end := strings.Index(source[start:], ";\n") + start
		source = source[:start] + "const ZHIQI_CATALOG = " + string(catalog) + source[end:]
		digest := sha256.Sum256(catalog)
		version := fmt.Sprintf("1.0.0+catalog.%x", digest[:12])
		source = strings.Replace(source, `version: "1.0.0"`, `version: "`+version+`"`, 1)
		registry := jsplugin.NewRegistry()
		if _, err = registry.Register(source, jsplugin.Options{}); err != nil {
			common.ApiError(c, err)
			return
		}
		result["plugin_source"] = source
		result["plugin_version"] = version
	}
	common.ApiSuccess(c, result)
}

func zhiqiError(c *gin.Context, status int, code string) {
	c.JSON(status, gin.H{"success": false, "code": code, "message": code, "data": nil})
}

// Bind a verified upstream receipt to a persisted uncertain submission.
// This never creates another upstream job or changes the customer's charge.
func ReconcileZhiqiTask(c *gin.Context) {
	var input struct {
		TaskID         string `json:"task_id"`
		UpstreamTaskID string `json:"upstream_task_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.TaskID == "" || input.UpstreamTaskID == "" || len(input.UpstreamTaskID) > 191 {
		zhiqiError(c, 400, "invalid_task_reference")
		return
	}
	var task model.Task
	if err := model.DB.Where("task_id = ? AND platform = ?", input.TaskID, "zhiqi").First(&task).Error; err != nil {
		zhiqiError(c, 404, "task_not_found")
		return
	}
	if task.PrivateData.UpstreamTaskID != "" || task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure {
		zhiqiError(c, 409, "task_already_reconciled")
		return
	}
	var submission model.ZhiqiSubmission
	if err := model.DB.WithContext(c.Request.Context()).First(&submission, "task_id = ?", task.TaskID).Error; err != nil || submission.State != "uncertain" {
		zhiqiError(c, 409, "task_submission_is_still_active")
		return
	}
	channel, err := service.ZhiqiChannel(task.ChannelId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	path := "/v1/videos/"
	if task.Action == "image" {
		path = "/v1/images/generations/"
	}
	response, _, err := service.ZhiqiRequest(c.Request.Context(), channel, http.MethodGet, path+url.PathEscape(input.UpstreamTaskID), nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if response["id"] != input.UpstreamTaskID || response["model"] != task.Properties.UpstreamModelName {
		zhiqiError(c, 409, "upstream_task_does_not_match_model")
		return
	}
	task.PrivateData.UpstreamTaskID = input.UpstreamTaskID
	task.Status = model.TaskStatusQueued
	task.FailReason = ""
	task.SetData(response)
	if err = model.BindZhiqiTaskReceipt(c.Request.Context(), &task); err != nil {
		zhiqiError(c, 409, "task_version_conflict")
		return
	}
	recordManageAudit(c, "zhiqi.task.reconcile", map[string]any{"task_id": task.TaskID, "channel_id": task.ChannelId})
	common.ApiSuccess(c, gin.H{"task_id": task.TaskID, "status": "queued"})
}
