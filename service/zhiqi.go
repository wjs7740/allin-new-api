package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func ZhiqiChannel(id int) (*model.Channel, error) {
	channel, err := model.GetChannelById(id, true)
	if err != nil {
		return nil, err
	}
	if channel.Status != common.ChannelStatusEnabled || channel.Type != constant.ChannelTypeTaskPlugin || channel.GetSetting().TaskPluginKey != "zhiqi" {
		return nil, errors.New("select an enabled Zhiqi plugin channel")
	}
	return channel, nil
}

// ZhiqiRequest only targets the administrator's channel base URL. Never
// follows redirects with merchant credentials or includes response bodies in errors.
func ZhiqiRequest(ctx context.Context, channel *model.Channel, method, path string, body any) (map[string]any, int, error) {
	base, err := url.Parse(strings.TrimRight(channel.GetBaseURL(), "/"))
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, 0, errors.New("invalid Zhiqi channel address")
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, 0, errors.New("invalid Zhiqi path")
	}
	keys := channel.GetKeys()
	if len(keys) != 1 {
		return nil, 0, errors.New("use one merchant API key per Zhiqi channel")
	}
	var payload []byte
	if body != nil {
		payload, err = common.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
	}
	requestCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, method, base.String()+path, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, errors.New("invalid Zhiqi request")
	}
	request.Header.Set("Authorization", "Bearer "+keys[0])
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	client, err := GetHttpClientWithProxySettings(channel.GetSetting().Proxy, channel.GetSetting())
	if err != nil {
		return nil, 0, errors.New("Zhiqi channel transport is unavailable")
	}
	copyClient := *client
	copyClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := copyClient.Do(request)
	if err != nil {
		return nil, 0, errors.New("Zhiqi request interrupted; reconcile mutations before retrying")
	}
	defer response.Body.Close()
	const limit = 8 << 20
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || len(raw) > limit {
		return nil, response.StatusCode, errors.New("Zhiqi response exceeds the response limit or is incomplete")
	}
	var result map[string]any
	if err = common.Unmarshal(raw, &result); err != nil || result == nil {
		return nil, response.StatusCode, errors.New("Zhiqi returned an invalid JSON response")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, response.StatusCode, fmt.Errorf("Zhiqi returned HTTP %d", response.StatusCode)
	}
	if success, ok := result["success"].(bool); ok && !success {
		return result, response.StatusCode, errors.New("Zhiqi reported an unsuccessful operation")
	}
	return result, response.StatusCode, nil
}

func ZhiqiAssetChannel(ctx context.Context, userID int) (*model.Channel, error) {
	var option model.Option
	if err := model.DB.WithContext(ctx).Where(&model.Option{Key: "ZhiqiAssetChannelID"}).First(&option).Error; err != nil {
		return nil, errors.New("AIGC asset library is not configured")
	}
	var channelID int
	if _, err := fmt.Sscan(option.Value, &channelID); err != nil {
		return nil, errors.New("AIGC asset library is not configured")
	}
	channel, err := ZhiqiChannel(channelID)
	if err != nil {
		return nil, err
	}
	user, err := model.GetUserCache(userID)
	if err != nil {
		return nil, err
	}
	groups := GetUserUsableGroups(user.Group)
	for _, group := range channel.GetGroups() {
		if _, ok := groups[group]; ok {
			return channel, nil
		}
	}
	return nil, errors.New("asset channel is not available to this user group")
}

func ResolveZhiqiLibraryAssets(ctx context.Context, userID, channelID int, value any) (map[string]any, error) {
	input, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("library_assets must be an object")
	}
	output := make(map[string]any, len(input))
	types := map[string]string{"images": "image", "videos": "video", "audios": "audio", "first_frame": "image", "last_frame": "image"}
	count := 0
	for key, raw := range input {
		kind, exists := types[key]
		if !exists {
			return nil, errors.New("unsupported library_assets field")
		}
		var ids []any
		single := key == "first_frame" || key == "last_frame"
		if single {
			ids = []any{raw}
		} else {
			ids, ok = raw.([]any)
			if !ok {
				return nil, errors.New("library asset references must be arrays")
			}
		}
		resolved := make([]any, 0, len(ids))
		for _, rawID := range ids {
			count++
			if count > 63 {
				return nil, errors.New("at most 63 library assets may be referenced")
			}
			id, valid := rawID.(string)
			if !valid {
				return nil, errors.New("invalid library asset reference")
			}
			asset, err := model.GetZhiqiAsset(ctx, id, userID)
			if err != nil || asset.ResourceType != "asset" || asset.Kind != kind || asset.State != "active" || asset.ChannelID != channelID || asset.UpstreamID == "" {
				return nil, errors.New("library asset is not accessible, ready, or on this channel")
			}
			if asset.GroupID != "" {
				group, err := model.GetZhiqiAsset(ctx, asset.GroupID, userID)
				if err != nil || group.ResourceType != "group" || group.State != "active" || group.ChannelID != channelID {
					return nil, errors.New("library asset group is not available")
				}
			}
			resolved = append(resolved, asset.UpstreamID)
		}
		if single {
			output[key] = resolved[0]
		} else {
			output[key] = resolved
		}
	}
	return output, nil
}
