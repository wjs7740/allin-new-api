package controller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func zhiqiActor(c *gin.Context) (int, bool) {
	id := c.GetInt("id")
	user, err := model.GetUserCache(id)
	return id, err == nil && user.Role >= common.RoleAdminUser
}

func ListZhiqiAssets(c *gin.Context) {
	userID, admin := zhiqiActor(c)
	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	page = max(1, min(page, 100000))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	pageSize = max(1, min(pageSize, 100))
	resourceType := "asset"
	if strings.HasSuffix(c.FullPath(), "/groups") {
		resourceType = "group"
	}
	items, total, err := model.ListZhiqiAssets(c.Request.Context(), userID, resourceType, c.Query("scope"), c.Query("group_id"), c.Query("keyword"), c.Query("kind"), c.Query("state"), page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for i := range items {
		items[i].CanManage = items[i].ManageableBy(userID, admin)
	}
	common.ApiSuccess(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func GetZhiqiAsset(c *gin.Context) {
	userID, admin := zhiqiActor(c)
	asset, err := model.GetZhiqiAsset(c.Request.Context(), c.Param("id"), userID)
	if err != nil || asset.State == "deleted" || (strings.Contains(c.FullPath(), "/groups/") != (asset.ResourceType == "group")) {
		zhiqiError(c, 404, "asset_not_found")
		return
	}
	preview := strings.HasSuffix(c.FullPath(), "/preview")
	if c.Query("refresh") == "true" || c.Query("refresh") == "1" || preview {
		if !preview && (asset.State == "creating" || asset.State == "updating" || asset.State == "deleting") && asset.UpdatedTime > time.Now().Add(-time.Minute).Unix() {
			zhiqiError(c, 409, "asset_operation_in_progress")
			return
		}
		if asset.UpstreamID == "" {
			zhiqiError(c, 409, "asset_requires_reconciliation")
			return
		}
		channel, channelErr := service.ZhiqiChannel(asset.ChannelID)
		if channelErr != nil {
			common.ApiError(c, channelErr)
			return
		}
		path := "/api/aigc-assets/assets/" + url.PathEscape(asset.UpstreamID)
		if asset.ResourceType == "group" {
			path = "/api/aigc-assets/groups/" + url.PathEscape(asset.UpstreamID)
		}
		if preview {
			if asset.ResourceType != "asset" {
				zhiqiError(c, 404, "asset_not_found")
				return
			}
			path += "/preview"
		} else {
			path += "?refresh=true"
		}
		response, _, requestErr := service.ZhiqiRequest(c.Request.Context(), channel, http.MethodGet, path, nil)
		if requestErr != nil {
			common.ApiError(c, requestErr)
			return
		}
		data, ok := response["data"].(map[string]any)
		if !ok {
			zhiqiError(c, 502, "invalid_asset_response")
			return
		}
		if preview {
			previewURL, valid := data["url"].(string)
			parsed, parseErr := url.Parse(previewURL)
			if !valid || parseErr != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil {
				zhiqiError(c, 502, "invalid_preview_url")
				return
			}
			common.ApiSuccess(c, gin.H{"url": previewURL})
			return
		}
		applyZhiqiAssetResult(asset, data)
		if err = model.SaveZhiqiAssetResult(c.Request.Context(), asset); err != nil {
			zhiqiError(c, 409, "asset_version_conflict")
			return
		}
	}
	asset.CanManage = asset.ManageableBy(userID, admin)
	common.ApiSuccess(c, asset)
}

type zhiqiAssetInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Scope       string `json:"scope"`
	GroupID     string `json:"group_id"`
	Kind        string `json:"kind"`
	URL         string `json:"url"`
	RequestID   string `json:"request_id"`
	MimeType    string `json:"mime_type"`
	SizeBytes   int64  `json:"size_bytes"`
	LockVersion int64  `json:"lock_version"`
	ETag        string `json:"etag"`
}

func CreateZhiqiAsset(c *gin.Context) {
	var input zhiqiAssetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		zhiqiError(c, 400, "invalid_asset_request")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 64 || utf8.RuneCountInString(input.Description) > 300 || len(input.RequestID) < 16 || len(input.RequestID) > 128 {
		zhiqiError(c, 400, "invalid_asset_name_or_request_id")
		return
	}
	userID, admin := zhiqiActor(c)
	isGroup := strings.HasSuffix(c.FullPath(), "/groups")
	isUpload := strings.HasSuffix(c.FullPath(), "/uploads")
	channel, err := service.ZhiqiAssetChannel(c.Request.Context(), userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	resourceType, prefix, path := "asset", "aia_", "/api/aigc-assets/assets"
	body := map[string]any{"name": input.Name}
	if input.Scope == "" {
		input.Scope = "private"
	}
	if input.Scope != "private" && input.Scope != "public" {
		zhiqiError(c, 400, "invalid_asset_scope")
		return
	}
	if isGroup {
		if input.Scope == "public" && !admin {
			zhiqiError(c, 403, "public_assets_require_admin")
			return
		}
		resourceType, prefix, path = "group", "aig_", "/api/aigc-assets/groups"
		body["scope"] = "private" // Remote scope is not local tenant isolation.
		body["description"] = input.Description
	} else {
		group, groupErr := model.GetZhiqiAsset(c.Request.Context(), input.GroupID, userID)
		if groupErr != nil || group.ResourceType != "group" || !group.ManageableBy(userID, admin) || group.State != "active" || group.UpstreamID == "" || group.ChannelID != channel.Id {
			zhiqiError(c, 404, "asset_group_not_available")
			return
		}
		input.Scope = group.Scope
		if input.Kind != "image" && input.Kind != "video" && input.Kind != "audio" {
			zhiqiError(c, 400, "invalid_asset_kind")
			return
		}
		body["group_id"], body["kind"] = group.UpstreamID, input.Kind
		if isUpload {
			caps := map[string]int64{"image": 30 << 20, "video": 200 << 20, "audio": 15 << 20}
			mimes := map[string]string{"image/jpeg": "image", "image/png": "image", "image/webp": "image", "image/gif": "image", "video/mp4": "video", "video/quicktime": "video", "audio/mpeg": "audio", "audio/wav": "audio", "audio/x-wav": "audio"}
			if mimes[input.MimeType] != input.Kind || input.SizeBytes < 1 || input.SizeBytes > caps[input.Kind] || (input.Kind == "image" && input.SizeBytes == caps[input.Kind]) {
				zhiqiError(c, 400, "invalid_asset_file")
				return
			}
			path = "/api/aigc-assets/uploads"
			body["mime_type"], body["size_bytes"] = input.MimeType, input.SizeBytes
		} else {
			parsed, parseErr := url.Parse(input.URL)
			if parseErr != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				zhiqiError(c, 400, "asset_url_required")
				return
			}
			body["url"] = input.URL
		}
	}
	encoded, _ := common.Marshal(input)
	requestHash := fmt.Sprintf("%x", sha256.Sum256([]byte(path+":"+input.RequestID)))
	asset := &model.ZhiqiAsset{ID: prefix + uuid.NewString(), ResourceType: resourceType, OwnerID: userID, RequestHash: requestHash,
		PayloadHash: fmt.Sprintf("%x", sha256.Sum256(encoded)), ChannelID: channel.Id, GroupID: input.GroupID, Scope: input.Scope,
		Name: input.Name, Description: input.Description, Kind: input.Kind, State: "creating"}
	created, reserveErr := model.ReserveZhiqiAsset(c.Request.Context(), asset)
	if reserveErr != nil {
		zhiqiError(c, 409, "asset_request_conflict")
		return
	}
	if !created {
		asset.CanManage = asset.ManageableBy(userID, admin)
		common.ApiSuccess(c, asset)
		return
	}
	body["request_id"] = asset.ID // Globally unique per local user and operation.
	zhiqiAssetMutation(c, asset, channel, http.MethodPost, path, body, isUpload)
}

func MutateZhiqiAsset(c *gin.Context) {
	var input zhiqiAssetInput
	if err := c.ShouldBindJSON(&input); err != nil || input.LockVersion < 1 {
		zhiqiError(c, 400, "asset_version_required")
		return
	}
	userID, admin := zhiqiActor(c)
	asset, err := model.GetZhiqiAsset(c.Request.Context(), c.Param("id"), userID)
	if err != nil || !asset.ManageableBy(userID, admin) || asset.UpstreamID == "" || (strings.Contains(c.FullPath(), "/groups/") != (asset.ResourceType == "group")) {
		zhiqiError(c, 404, "asset_not_found")
		return
	}
	complete := strings.HasSuffix(c.FullPath(), "/complete")
	path := "/api/aigc-assets/assets/" + url.PathEscape(asset.UpstreamID)
	if asset.ResourceType == "group" {
		path = "/api/aigc-assets/groups/" + url.PathEscape(asset.UpstreamID)
	}
	state := "updating"
	body := map[string]any{"lock_version": asset.UpstreamVersion}
	if c.Request.Method == http.MethodDelete {
		if c.Query("confirm") != "true" {
			zhiqiError(c, 400, "delete_confirmation_required")
			return
		}
		path += "?confirm=true"
		state = "deleting"
	} else if complete {
		if asset.ResourceType != "asset" || asset.State != "uploading" {
			zhiqiError(c, 409, "asset_not_uploading")
			return
		}
		path = "/api/aigc-assets/uploads/" + url.PathEscape(asset.UpstreamID) + "/complete"
		if len(input.ETag) > 512 {
			zhiqiError(c, 400, "invalid_upload_etag")
			return
		}
		body["etag"] = input.ETag
	} else {
		input.Name = strings.TrimSpace(input.Name)
		if input.Name == "" || utf8.RuneCountInString(input.Name) > 64 || utf8.RuneCountInString(input.Description) > 300 {
			zhiqiError(c, 400, "invalid_asset_name")
			return
		}
		body["name"] = input.Name
		if asset.ResourceType == "group" {
			body["description"] = input.Description
		}
	}
	channel, err := service.ZhiqiChannel(asset.ChannelID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err = model.ClaimZhiqiAssetMutation(c.Request.Context(), asset, input.LockVersion, state); err != nil {
		zhiqiError(c, 409, "asset_version_conflict")
		return
	}
	zhiqiAssetMutation(c, asset, channel, c.Request.Method, path, body, false)
}

func zhiqiAssetMutation(c *gin.Context, asset *model.ZhiqiAsset, channel *model.Channel, method, path string, body map[string]any, upload bool) {
	// Once persisted, a client disconnect does not cancel recording the outcome.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 55*time.Second)
	defer cancel()
	response, status, requestErr := service.ZhiqiRequest(ctx, channel, method, path, body)
	data, _ := response["data"].(map[string]any)
	resource := data
	if upload {
		resource, _ = data["asset"].(map[string]any)
	}
	asset.State = "unknown"
	if resource != nil {
		applyZhiqiAssetResult(asset, resource)
	}
	if requestErr != nil {
		asset.ErrorCode = "upstream_operation_unconfirmed"
		if status >= 400 && status < 500 && status != 408 && status != 429 {
			asset.ErrorCode = "upstream_operation_rejected"
		}
	} else if method == http.MethodDelete {
		asset.State = "deleted"
	} else if asset.UpstreamID == "" || asset.UpstreamVersion < 1 {
		asset.State = "unknown"
		asset.ErrorCode = "invalid_upstream_resource"
	}
	if err := model.SaveZhiqiAssetResult(ctx, asset); err != nil {
		zhiqiError(c, 503, "asset_result_persistence_failed")
		return
	}
	userID, admin := zhiqiActor(c)
	asset.CanManage = asset.ManageableBy(userID, admin)
	if requestErr != nil || asset.State == "unknown" {
		c.JSON(502, gin.H{"success": false, "code": asset.ErrorCode, "message": "Refresh this resource before retrying the operation", "data": asset})
		return
	}
	if upload {
		// Only object-scoped temporary upload credentials reach the client.
		common.ApiSuccess(c, gin.H{"asset": asset, "upload_url": data["upload_url"], "upload_token": data["upload_token"], "object_key": data["object_key"], "expires_at": data["expires_at"]})
		return
	}
	common.ApiSuccess(c, asset)
}

func applyZhiqiAssetResult(asset *model.ZhiqiAsset, data map[string]any) {
	if id, ok := data["id"].(string); ok && id != "" {
		asset.UpstreamID = id
	}
	if version, ok := data["lock_version"].(float64); ok && version >= 1 && version <= 9007199254740991 && version == float64(int64(version)) {
		asset.UpstreamVersion = int64(version)
	}
	if name, ok := data["name"].(string); ok {
		asset.Name = name
	}
	if description, ok := data["description"].(string); ok {
		asset.Description = description
	}
	if state, ok := data["state"].(string); ok {
		for _, valid := range []string{"creating", "uploading", "processing", "active", "failed", "unknown", "updating", "deleting", "expired", "deleted"} {
			if state == valid {
				asset.State = state
				break
			}
		}
	}
	asset.ErrorCode = ""
}
