// The administrator may replace this catalog with models returned by the
// upstream. Generic aliases can be mapped to a specific upstream model.
const ZHIQI_CATALOG = { video: ["zhiqi-video", "seedance-2.0-fast-token", "seedance-2.0-mini-token"], image: ["zhiqi-image"] };

const RESOLUTIONS = ["default", "480p", "512p", "720p", "768p", "1080p", "1k", "2k", "4k"];
export const meta = {
  apiVersion: 1,
  key: "zhiqi",
  name: "Zhiqi",
  version: "1.0.0",
  author: { name: "Allin New API" },
  icon: "text:ZQ",
  baseUrl: "https://zhiqiapi.com",
  description: { en: "Zhiqi text, image and video APIs", zh: "志奇文本、图片与视频接口" },
  keyHint: { en: "Zhiqi API key", zh: "志奇 API Key" },
  models: ZHIQI_CATALOG.video.concat(ZHIQI_CATALOG.image),
  fetchMode: "per_task",
  channelConfig: { type: "object", properties: {
    defaultDuration: { type: "integer", title: { en: "Default video duration", zh: "默认视频时长" }, minimum: 1, maximum: 3600, default: 6 },
  } },
  usageSchema: {
    requests: { type: "number", unit: "count", description: { en: "Generation request unit price", zh: "生成请求单价" } },
    seconds: { type: "number", unit: "second", description: { en: "Video generation unit price", zh: "视频生成单价" } },
    image_count: { type: "number", unit: "count", description: { en: "Image generation unit price", zh: "图片生成单价" } },
    resolution: { enum: RESOLUTIONS, description: { en: "Output resolution", zh: "输出清晰度" } },
  },
  relay: [
    { incoming_path: "/v1/models", upstream_path: "/v1/models" },
    { incoming_path: "/v1/chat/completions", upstream_path: "/v1/chat/completions" },
    { incoming_path: "/v1/responses", upstream_path: "/v1/responses" },
    { incoming_path: "/v1/responses/compact", upstream_path: "/v1/responses/compact" },
    { incoming_path: "/v1/messages", upstream_path: "/v1/messages" },
    { incoming_path: "/v1beta/models/{model}:generateContent", upstream_path: "/v1beta/models/{model}:generateContent" },
    { incoming_path: "/v1/embeddings", upstream_path: "/v1/embeddings" },
    { incoming_path: "/v1/audio/speech", upstream_path: "/v1/audio/speech" },
    { incoming_path: "/v1/images/generations", upstream_path: "/v1/images/generations" },
    { incoming_path: "/v1/images/edits", upstream_path: "/v1/images/edits" },
  ],
  routes: [
    ...(ZHIQI_CATALOG.video.length ? [
    { method: "POST", path: "/zhiqi/v1/videos", type: "submit", decode: "createVideo", render: "videoCreated", models: ZHIQI_CATALOG.video },
    { method: "GET", path: "/zhiqi/v1/videos/:task_id", type: "query", render: "videoStatus" },
    { method: "GET", path: "/zhiqi/v1/videos/:task_id/content", type: "query", render: "videoContent" },
    ] : []),
    ...(ZHIQI_CATALOG.image.length ? [
    { method: "POST", path: "/v1/images/generations/async", type: "submit", decode: "createImage", render: "created", models: ZHIQI_CATALOG.image },
    { method: "GET", path: "/v1/images/generations/:task_id", type: "query", render: "imageStatus" },
    ] : []),
  ],
  protocols: ZHIQI_CATALOG.video.length ? [{ name: "openai_video", models: ZHIQI_CATALOG.video }] : [],
};

function object(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function positiveInteger(value, maximum, name) {
  if ((typeof value !== "number" && typeof value !== "string") || String(value).trim() === "") throw new Error(name + " must be an integer");
  const n = Number(value);
  if (!Number.isInteger(n) || n < 1 || n > maximum) throw new Error(name + " is outside the supported range");
  return n;
}

function publicURL(value, name) {
  if (typeof value !== "string" || !/^https?:\/\/[^\s/@]+(?::\d+)?(?:[/?#][^\s]*)?$/i.test(value)) throw new Error(name + " requires an HTTP(S) URL");
  return value;
}

function normalize(body, action, config) {
  if (action !== "video" && action !== "image") throw new Error("unsupported media action");
  if (!object(body)) throw new Error("JSON object required");
  if (typeof body.model !== "string" || !body.model.trim()) throw new Error("model is required");
  if (typeof body.prompt !== "string" || !body.prompt.trim()) throw new Error("prompt is required");
  const req = Object.assign({}, body);
  if (req.metadata !== undefined) {
    if (!object(req.metadata)) throw new Error("metadata must be an object");
    for (const key of Object.keys(req.metadata)) {
      if (key === "model" || key === "prompt" || req[key] !== undefined) throw new Error("metadata conflicts with " + key);
      req[key] = req.metadata[key];
    }
    delete req.metadata;
  }
  const aliases = action === "video" ? {
    duration: ["seconds"], aspect_ratio: ["ratio"], input_reference: ["image", "image_url"], reference_image_urls: ["images", "image_urls"],
  } : {};
  for (const canonical of Object.keys(aliases)) {
    for (const alias of aliases[canonical]) {
      if (req[alias] === undefined || req[alias] === null || req[alias] === "") { delete req[alias]; continue; }
      if (req[canonical] !== undefined && req[canonical] !== null && req[canonical] !== "") {
        if (canonical !== "duration" || Number(req[canonical]) !== Number(req[alias])) throw new Error("conflicting aliases for " + canonical);
      } else req[canonical] = req[alias];
      delete req[alias];
    }
  }
  const shared = ["model", "prompt", "n", "size", "resolution", "aspect_ratio", "reference_image_urls"];
  const fields = action === "video" ? shared.concat([
    "duration", "input_reference", "reference_video_urls", "reference_audio_urls", "first_frame_image_url", "last_frame_image_url",
    "negative_prompt", "generate_audio", "return_last_frame", "seed", "bypass_face_check", "grid_strength", "strength", "auto_face_mask", "library_assets",
  ]) : shared.concat(["quality", "response_format", "output_format", "output_compression", "background", "moderation", "mask"]);
  for (const key of Object.keys(req)) if (!fields.includes(key)) throw new Error("unsupported field: " + key);
  req.n = req.n === undefined ? 1 : positiveInteger(req.n, 128, "n");
  if (action === "video") {
    req.duration = positiveInteger(req.duration === undefined ? (config && config.defaultDuration) || 6 : req.duration, 3600, "duration");
    // Count-adjusted seconds also stay within the host's second-unit ceiling.
    if (req.duration * req.n > 3600) throw new Error("total video duration exceeds 3600 seconds");
  }
  if (req.resolution !== undefined) {
    if (typeof req.resolution !== "string" || !RESOLUTIONS.slice(1).includes(req.resolution.toLowerCase())) throw new Error("unsupported resolution");
    req.resolution = req.resolution.toLowerCase();
  }
  for (const key of ["input_reference", "first_frame_image_url", "last_frame_image_url", "mask"]) if (req[key] !== undefined) publicURL(req[key], key);
  for (const key of ["reference_image_urls", "reference_video_urls", "reference_audio_urls"]) {
    if (req[key] === undefined) continue;
    if (!Array.isArray(req[key]) || req[key].length > 128) throw new Error(key + " must be an array of at most 128 URLs");
    for (const value of req[key]) publicURL(value, key);
    if (req[key].length === 0) delete req[key];
  }
  for (const key of ["generate_audio", "return_last_frame", "bypass_face_check", "auto_face_mask"]) {
    if (req[key] !== undefined && typeof req[key] !== "boolean") throw new Error(key + " must be boolean");
  }
  if (req.seed !== undefined && !Number.isSafeInteger(req.seed)) throw new Error("seed must be an integer");
  if (req.grid_strength !== undefined && (typeof req.grid_strength !== "number" || !Number.isFinite(req.grid_strength) || req.grid_strength < 0 || req.grid_strength > 1)) throw new Error("grid_strength must be between 0 and 1");
  if (req.strength !== undefined) req.strength = positiveInteger(req.strength, 10, "strength");
  // Local asset IDs are resolved by the host after the channel is selected.
  if (req.library_assets !== undefined && !object(req.library_assets)) throw new Error("library_assets must be an object");
  return req;
}

function decode(ctx, action) {
  if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
  const body = normalize(ctx.body.value, action);
  // Decoder contexts have no channel config. Leave an omitted duration absent
  // so billing and the request builder use the selected channel's default.
  if (action === "video" && ctx.body.value.duration === undefined && ctx.body.value.seconds === undefined &&
      (!object(ctx.body.value.metadata) || (ctx.body.value.metadata.duration === undefined && ctx.body.value.metadata.seconds === undefined))) delete body.duration;
  return { kind: "submit", model: ctx.model || body.model, action, requestBody: body };
}

export function buildSubmitRequest(ctx) {
  const body = normalize(ctx.requestBody, ctx.action, ctx.config);
  body.model = ctx.upstreamModel || ctx.model;
  return { url: ctx.baseUrl.replace(/\/+$/, "") + (ctx.action === "image" ? "/v1/images/generations/async" : "/v1/videos"),
    method: "POST", headers: { Authorization: "Bearer " + ctx.apiKey, "Content-Type": "application/json" }, body };
}

export function parseSubmitResponse(_ctx, response) {
  const body = response.body;
  if (!object(body) || typeof body.id !== "string" || !body.id.trim()) throw new Error("upstream submission has no task ID; reconcile before resubmitting");
  return { taskId: body.id, taskData: body };
}

export function extractUsage(ctx) {
  const body = normalize(ctx.requestBody, ctx.action, ctx.config);
  return { requests: 1, seconds: ctx.action === "video" ? body.duration * body.n : 0,
    image_count: ctx.action === "image" ? body.n : 0, resolution: body.resolution || "default" };
}

export function buildQueryRequest(ctx) {
  return { url: ctx.baseUrl.replace(/\/+$/, "") + (ctx.action === "image" ? "/v1/images/generations/" : "/v1/videos/") + encodeURIComponent(ctx.taskId),
    method: "GET", headers: { Authorization: "Bearer " + ctx.apiKey } };
}

export function parseTaskResult(_ctx, body) {
  const statuses = { queued: "QUEUED", in_progress: "IN_PROGRESS", succeeded: "SUCCESS", completed: "SUCCESS", failed: "FAILURE", expired: "FAILURE" };
  const status = object(body) ? statuses[body.status] : undefined;
  if (!status) return { status: "UNKNOWN", reason: "unrecognized upstream task status" };
  const result = { status };
  const progress = Number(String(body.progress || "0").replace(/%$/, ""));
  if (Number.isFinite(progress) && progress >= 0 && progress <= 100) result.progress = Math.min(progress, status === "SUCCESS" ? 100 : 99) + "%";
  if (status === "FAILURE") result.reason = object(body.error) && typeof body.error.message === "string" ? body.error.message : "upstream task failed";
  return result;
}

export function extractUsageOnComplete(task, _result, body) {
  const facts = {};
  if (!object(body)) return facts;
  if (task.action === "image") {
    const data = Array.isArray(body.data) ? body.data : object(body.data) ? [body.data] : [];
    const count = Math.max(data.filter(x => object(x) && typeof x.url === "string" && x.url.trim()).length,
      data.filter(x => object(x) && typeof x.b64_json === "string" && x.b64_json.trim()).length);
    if (count > 0 && count <= 128) facts.image_count = count;
  } else {
    const duration = body.duration === undefined ? body.seconds : body.duration;
    const n = body.n === undefined ? 1 : Number(body.n);
    const seconds = Number(duration) * n;
    if (Number.isInteger(n) && n >= 1 && n <= 128 && Number.isFinite(seconds) && seconds > 0 && seconds <= 3600) facts.seconds = seconds;
  }
  if (typeof body.resolution === "string" && RESOLUTIONS.includes(body.resolution.toLowerCase())) facts.resolution = body.resolution.toLowerCase();
  return facts;
}

function publicTask(task, kind) {
  const statuses = { NOT_START: "queued", SUBMITTED: "queued", QUEUED: "queued", IN_PROGRESS: "in_progress", SUCCESS: "succeeded", FAILURE: "failed" };
  const output = { id: task.task_id, object: kind + ".task", model: object(task.data) ? task.data.model || "" : "",
    status: statuses[task.status] || "in_progress", created: task.created_at, progress: task.progress || "0%" };
  if (task.finished_at) output.completed_at = task.finished_at;
  if (task.status === "SUCCESS" && object(task.data)) {
    output.data = task.data.data || [];
    if (task.data.usage !== undefined) output.usage = task.data.usage;
  }
  if (object(task.data) && task.data.submission_state) output.submission_state = task.data.submission_state;
  if (task.status === "FAILURE") output.error = { code: object(task.data) && object(task.data.error) ? task.data.error.code || "generation_failed" : "generation_failed", message: task.fail_reason || "upstream task failed" };
  return output;
}

export const native = {
  createVideo(ctx) { return decode(ctx, "video"); },
  videoCreated(_ctx, task) { return publicTask(task, "video"); },
  videoStatus(_ctx, task) { if (!object(task.data) || task.data.object !== "video.task") throw new Error("artifact_not_found"); return publicTask(task, "video"); },
  videoContent(ctx, task) {
    if (!object(task.data) || task.data.object !== "video.task") throw new Error("artifact_not_found");
    if (!ctx.artifacts || !ctx.artifacts.video) return { __status: 409, __body: { error: { code: "video_not_ready", message: "Video is not ready" } } };
    return { __status: 302, __headers: { Location: ctx.artifacts.video.url } };
  },
  createImage(ctx) { return decode(ctx, "image"); },
  created(_ctx, task) { return { __status: 202, __body: publicTask(task, "image") }; },
  imageStatus(_ctx, task) { if (!object(task.data) || task.data.object !== "image.task") throw new Error("artifact_not_found"); return publicTask(task, "image"); },
};

export const protocols = ZHIQI_CATALOG.video.length ? { openai_video: {
  decodeRequest(ctx) { return decode(ctx, "video"); },
  render(_ctx, task) { if (!object(task.data) || task.data.object !== "video.task") throw new Error("artifact_not_found"); return publicTask(task, "video"); },
} } : {};

export function listArtifacts(task) {
  if (task.status !== "SUCCESS") return [];
  if (object(task.data) && task.data.object === "video.task") return [{ key: "video", type: "video", mimeType: "video/mp4" }];
  const data = object(task.data) && Array.isArray(task.data.data) ? task.data.data : [];
  return data.flatMap((entry, i) => object(entry) && typeof entry.url === "string" && entry.url ? [{ key: "image-" + i, type: "image" }] : []);
}

export function buildContentRequest(ctx) {
  if (ctx.action === "video" && ctx.artifactKey === "video") return {
    url: ctx.baseUrl.replace(/\/+$/, "") + "/v1/videos/" + encodeURIComponent(ctx.upstreamTaskId) + "/content",
    method: "GET", headers: { Authorization: "Bearer " + ctx.apiKey },
  };
  const match = /^image-(\d+)$/.exec(ctx.artifactKey || "");
  const data = object(ctx.data) && Array.isArray(ctx.data.data) ? ctx.data.data : [];
  const entry = match ? data[Number(match[1])] : null;
  if (!object(entry) || !entry.url) throw new Error("artifact_not_found");
  return { url: publicURL(entry.url, "image result"), method: "GET", headers: {}, credentialless: true };
}
