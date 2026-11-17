package plugins_test

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZhiqiContracts(t *testing.T) {
	source, err := builtinplugins.Source("zhiqi")
	require.NoError(t, err)
	registry := jsplugin.NewRegistry()
	plugin, err := registry.RegisterFactory(source, jsplugin.Options{Key: "zhiqi"})
	require.NoError(t, err)
	require.NoError(t, jsplugin.ValidateV1Meta(plugin.Meta))
	call := func(name string, args ...any) map[string]any {
		t.Helper()
		value, callErr := plugin.Engine.Call(t.Context(), name, args...)
		require.NoError(t, callErr)
		data, marshalErr := common.Marshal(value)
		require.NoError(t, marshalErr)
		var result map[string]any
		require.NoError(t, common.Unmarshal(data, &result))
		return result
	}
	ctx := func(body map[string]any, action string) map[string]any {
		return map[string]any{"model": "zhiqi-video", "upstreamModel": "upstream-model", "baseUrl": "https://upstream.example/",
			"apiKey": "fixture-key", "action": action, "requestBody": body, "config": map[string]any{"defaultDuration": 8}}
	}
	t.Run("routes video and async images independently of synchronous provider routes", func(t *testing.T) {
		binding, ok := registry.Generation().LookupEndpoint("POST", "/v1/videos", "zhiqi-video")
		require.True(t, ok)
		assert.Equal(t, "zhiqi", binding.Plugin.Meta.Key)
		_, ok = registry.Generation().LookupEndpoint("POST", "/v1/videos", "gpt-image-2")
		assert.False(t, ok)
		assert.True(t, plugin.Meta.HasProviderPart())
	})
	t.Run("normalizes aliases and preserves explicit zero and false", func(t *testing.T) {
		body := map[string]any{"model": "zhiqi-video", "prompt": "a fox", "seconds": "8", "duration": 8,
			"ratio": "16:9", "image_url": "https://cdn.example/fox.png", "seed": 0, "generate_audio": false, "grid_strength": 0}
		descriptor := call("buildSubmitRequest", ctx(body, "video"))
		assert.Equal(t, "https://upstream.example/v1/videos", descriptor["url"])
		req := descriptor["body"].(map[string]any)
		assert.Equal(t, "upstream-model", req["model"])
		assert.Equal(t, float64(8), req["duration"])
		assert.Equal(t, false, req["generate_audio"])
		assert.Equal(t, float64(0), req["seed"])
		assert.Equal(t, float64(0), req["grid_strength"])
		assert.Equal(t, "16:9", req["aspect_ratio"])
		assert.NotContains(t, req, "seconds")
		facts := call("extractUsage", ctx(body, "video"))
		assert.Equal(t, float64(8), facts["seconds"])
	})
	for _, tc := range []struct{ size, resolution, ratio string }{
		{"1280x720", "720p", "16:9"}, {"720x1280", "720p", "9:16"},
		{"1920x1080", "1080p", "16:9"}, {"854x480", "480p", "16:9"},
		{"720p", "720p", ""}, {"16:9", "", "16:9"},
	} {
		t.Run("video size "+tc.size+" normalizes submission and billing together", func(t *testing.T) {
			body := map[string]any{"model": "seedance-2.0-fast", "prompt": "cup", "seconds": "5", "size": tc.size}
			driver := ctx(body, "video")
			req := call("buildSubmitRequest", driver)["body"].(map[string]any)
			assert.NotContains(t, req, "size")
			if tc.resolution != "" {
				assert.Equal(t, tc.resolution, req["resolution"])
				assert.Equal(t, tc.resolution, call("extractUsage", driver)["resolution"])
			}
			if tc.ratio != "" {
				assert.Equal(t, tc.ratio, req["aspect_ratio"])
			}
		})
	}
	t.Run("channel default determines both sent duration and reservation", func(t *testing.T) {
		body := map[string]any{"model": "zhiqi-video", "prompt": "a fox"}
		intent, decodeErr := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "decodeRequest"},
			map[string]any{"model": "zhiqi-video", "body": map[string]any{"kind": "json", "value": body}})
		require.NoError(t, decodeErr)
		encoded, marshalErr := common.Marshal(intent)
		require.NoError(t, marshalErr)
		var decoded struct {
			RequestBody map[string]any `json:"requestBody"`
		}
		require.NoError(t, common.Unmarshal(encoded, &decoded))
		assert.NotContains(t, decoded.RequestBody, "duration")
		descriptor := call("buildSubmitRequest", ctx(decoded.RequestBody, "video"))
		assert.Equal(t, float64(8), descriptor["body"].(map[string]any)["duration"])
		assert.Equal(t, float64(8), call("extractUsage", ctx(decoded.RequestBody, "video"))["seconds"])
	})
	t.Run("Seedance token models reserve bounded tokens and settle reported usage", func(t *testing.T) {
		driver := ctx(map[string]any{"model": "alias", "prompt": "fox", "duration": 6, "reference_video_urls": []any{"https://cdn.example/v.mp4"}}, "video")
		driver["upstreamModel"] = "seedance-2.0-fast-token"
		facts := call("extractUsage", driver)
		assert.Equal(t, float64(120000), facts["tokens"])
		assert.Equal(t, true, facts["has_video"])
		for _, usage := range []any{-1, 1.5, "1200", 1e30, nil} {
			facts = call("extractUsageOnComplete", driver, map[string]any{}, map[string]any{"usage": map[string]any{"total_tokens": usage}})
			assert.NotContains(t, facts, "tokens")
		}
		facts = call("extractUsageOnComplete", driver, map[string]any{}, map[string]any{"usage": map[string]any{"total_tokens": 98765}})
		assert.Equal(t, float64(98765), facts["tokens"])
		driver["requestBody"] = map[string]any{"model": "alias", "prompt": "fox", "duration": 1e30}
		_, err := plugin.Engine.Call(t.Context(), "extractUsage", driver)
		require.Error(t, err)
	})
	for _, tc := range []struct {
		name  string
		extra map[string]any
	}{
		{"conflicting duration", map[string]any{"seconds": 4, "duration": 8}},
		{"conflicting image aliases", map[string]any{"image": "https://a.example/1", "image_url": "https://a.example/2"}},
		{"negative duration", map[string]any{"duration": -1}},
		{"huge duration", map[string]any{"duration": 1e30}},
		{"fractional count", map[string]any{"n": 1.5}},
		{"oversized count", map[string]any{"n": 129}},
		{"total duration overflow", map[string]any{"duration": 3600, "n": 2}},
		{"metadata quantity bypass", map[string]any{"metadata": map[string]any{"duration": 1e30}}},
		{"unrecognized field", map[string]any{"callback_url": "https://example.com/callback"}},
		{"inline input", map[string]any{"input_reference": "data:image/png;base64,AAAA"}},
		{"boolean quantity", map[string]any{"duration": true}},
		{"unknown pixel size", map[string]any{"size": "1234x567"}},
		{"object property as size", map[string]any{"size": "__proto__"}},
		{"size resolution conflict", map[string]any{"size": "1280x720", "resolution": "480p"}},
		{"size ratio conflict", map[string]any{"size": "1280x720", "aspect_ratio": "9:16"}},
		{"metadata size resolution conflict", map[string]any{"size": "1280x720", "metadata": map[string]any{"resolution": "480p"}}},
	} {
		t.Run("rejects "+tc.name+" before reservation", func(t *testing.T) {
			body := map[string]any{"model": "zhiqi-video", "prompt": "fox"}
			for key, value := range tc.extra {
				body[key] = value
			}
			_, callErr := plugin.Engine.Call(t.Context(), "extractUsage", ctx(body, "video"))
			require.Error(t, callErr)
		})
	}
	t.Run("accepts 202 and binds the upstream task without exposing credentials", func(t *testing.T) {
		result := call("parseSubmitResponse", ctx(nil, "image"), map[string]any{"statusCode": 202, "body": map[string]any{"id": "remote-1", "object": "image.task", "status": "queued"}})
		assert.Equal(t, "remote-1", result["taskId"])
		assert.NotContains(t, result, "apiKey")
		_, callErr := plugin.Engine.Call(t.Context(), "parseSubmitResponse", ctx(nil, "image"), map[string]any{"body": map[string]any{"status": "queued"}})
		require.ErrorContains(t, callErr, "no task ID")
	})
	for status, expected := range map[string]string{"queued": "QUEUED", "in_progress": "IN_PROGRESS", "succeeded": "SUCCESS", "failed": "FAILURE", "expired": "FAILURE", "other": "UNKNOWN"} {
		t.Run("upstream status "+status, func(t *testing.T) {
			result := call("parseTaskResult", map[string]any{}, map[string]any{"status": status, "progress": "99%"})
			assert.Equal(t, expected, result["status"])
		})
	}
	t.Run("image billing counts payloads instead of response entries", func(t *testing.T) {
		for _, tc := range []struct {
			body  any
			count any
		}{
			{map[string]any{"url": "https://cdn.example/1.png"}, float64(1)},
			{[]any{map[string]any{"url": "https://cdn.example/1.png"}, map[string]any{"b64_json": "AAAA"}, map[string]any{"revised_prompt": "fox"}}, float64(1)},
			{[]any{map[string]any{"revised_prompt": "fox"}}, nil},
		} {
			facts := call("extractUsageOnComplete", map[string]any{"action": "image"}, map[string]any{}, map[string]any{"data": tc.body})
			assert.Equal(t, tc.count, facts["image_count"])
		}
	})
	t.Run("video download uses private task id only on the upstream request", func(t *testing.T) {
		descriptor := call("buildContentRequest", map[string]any{"action": "video", "artifactKey": "video", "baseUrl": "https://upstream.example", "upstreamTaskId": "remote/id", "apiKey": "fixture-key"})
		assert.Equal(t, "https://upstream.example/v1/videos/remote%2Fid/content", descriptor["url"])
		assert.Equal(t, "Bearer fixture-key", descriptor["headers"].(map[string]any)["Authorization"])
	})
}
