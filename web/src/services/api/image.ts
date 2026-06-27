import axios from "axios";

import { buildApiUrl, useConfigStore, type AiConfig } from "@/stores/use-config-store";
import { useUserStore } from "@/stores/use-user-store";
import { nanoid } from "nanoid";
import { dataUrlToJpegFile } from "@/lib/image-utils";
import { buildImageReferencePromptText } from "@/lib/image-reference-prompt";
import { apiPost } from "@/services/api/request";
import { imageToDataUrl } from "@/services/image-storage";
import type { ReferenceImage } from "@/types/image";
import { IMAGE_MAX_RATIO, IMAGE_SIZE_STEP, mark4KImageSize, parseImageDimensions, stripImageSizeMarker, validateImageGenerationSize } from "@/constant/image-generation-constraints";

export type ChatCompletionMessage = {
    role: "system" | "user" | "assistant";
    content: string | Array<{ type: "text"; text: string } | { type: "image_url"; image_url: { url: string } }>;
};

type ImageApiResponse = {
    id?: string;
    task_id?: string;
    data?: Array<Record<string, unknown>>;
    result?: { data?: Array<Record<string, unknown>> };
    error?: { message?: string } | string;
    status?: string;
    retry_after?: number;
    code?: number;
    msg?: string;
};

export type VectorizeImageResult = {
    content: string;
    dataUrl: string;
    width: number;
    height: number;
    bytes: number;
    mimeType: string;
    engine?: string;
};

const QUALITY_BASE: Record<string, number> = {
    low: 1024,
    medium: 2048,
    high: 2880,
    standard: 1024,
    hd: 2048,
};
const QUALITY_ALIASES: Record<string, string> = {
    "1k": "low",
    "2k": "medium",
    "4k": "high",
};
const DEFAULT_IMAGE_SHORT_SIDE = 1024;
const IMAGE_RESPONSE_FORMAT = "url";
const IMAGE_OUTPUT_FORMAT = "png";
const IMAGE_TASK_MAX_POLLS = 90;
const IMAGE_TASK_DEFAULT_DELAY = 2000;
const IMAGE_TASK_MAX_DELAY = 10000;
const DEFAULT_REFERENCE_COMPRESSION_QUALITY = 0.8;

function normalizeQuality(quality: string) {
    const value = quality.trim().toLowerCase();
    const normalized = QUALITY_ALIASES[value] || value;
    return QUALITY_BASE[normalized] ? normalized : undefined;
}

/** Map "quality + ratio" to an explicit pixel dimension like "3840x2160". */
function resolveSize(quality: string | undefined, ratio: string): string {
    const parsedRatio = parseImageRatio(ratio);
    const basePixels = quality ? QUALITY_BASE[quality] : undefined;
    const isLandscape = parsedRatio.width >= parsedRatio.height;
    const longRatio = isLandscape ? parsedRatio.width / parsedRatio.height : parsedRatio.height / parsedRatio.width;
    let longSide: number;
    let shortSide: number;

    if (basePixels) {
        const targetPixels = basePixels * basePixels;
        const longSideRaw = Math.sqrt(targetPixels * longRatio);
        longSide = Math.floor(longSideRaw / IMAGE_SIZE_STEP) * IMAGE_SIZE_STEP;
        shortSide = Math.round(longSide / longRatio / IMAGE_SIZE_STEP) * IMAGE_SIZE_STEP;
    } else {
        shortSide = DEFAULT_IMAGE_SHORT_SIDE;
        longSide = Math.round((shortSide * longRatio) / IMAGE_SIZE_STEP) * IMAGE_SIZE_STEP;
    }

    const width = isLandscape ? longSide : shortSide;
    const height = isLandscape ? shortSide : longSide;
    validateImageSize(width, height);
    return `${width}x${height}`;
}

function parseImageRatio(value: string) {
    const parts = value.split(":");
    if (parts.length !== 2) throw new Error("图像尺寸格式不支持，请使用 auto、9:16 或 1024x1024");
    const w = Number(parts[0]);
    const h = Number(parts[1]);
    if (!Number.isFinite(w) || !Number.isFinite(h) || w <= 0 || h <= 0) throw new Error("图像比例必须是正数，例如 9:16");
    if (Math.max(w, h) / Math.min(w, h) > IMAGE_MAX_RATIO) throw new Error("图像宽高比不能超过 3:1，请调整尺寸");
    return { width: w, height: h };
}

function validateImageSize(width: number, height: number) {
    validateImageGenerationSize(width, height);
}

function resolveRequestSize(quality: string | undefined, size: string, keepBusinessMarker = false) {
    const rawValue = size.trim();
    const value = stripImageSizeMarker(rawValue);
    if (!value || value.toLowerCase() === "auto") return undefined;
    const dimensions = parseImageDimensions(value);
    if (dimensions) {
        validateImageSize(dimensions.width, dimensions.height);
        const normalized = `${dimensions.width}x${dimensions.height}`;
        return keepBusinessMarker && rawValue.toLowerCase().startsWith("4k:") ? mark4KImageSize(normalized) : normalized;
    }
    if (value.includes(":")) return resolveSize(quality, value);
    throw new Error("图像尺寸格式不支持，请使用 auto、9:16 或 1024x1024");
}

function resolveImageDataUrl(item: Record<string, unknown>) {
    if (typeof item.b64_json === "string" && item.b64_json) {
        return item.b64_json.startsWith("data:") ? item.b64_json : `data:image/png;base64,${item.b64_json}`;
    }
    if (typeof item.url === "string" && item.url) {
        return normalizeImageUrl(item.url);
    }
    return null;
}

function parseImagePayload(payload: ImageApiResponse) {
    if (typeof payload.code === "number" && payload.code !== 0) {
        throw new Error(payload.msg || "请求失败");
    }
    const status = payload.status?.toLowerCase();
    if (status === "failed" || status === "error" || status === "canceled") {
        throw new Error(readApiError(payload) || "图片生成失败");
    }
    if (isPendingImageStatus(status)) {
        throw new Error("图片任务仍在处理中，请稍后重试");
    }
    const items = Array.isArray(payload.data) ? payload.data : Array.isArray(payload.result?.data) ? payload.result.data : [];
    const images =
        items
            .map(resolveImageDataUrl)
            .filter((value): value is string => Boolean(value))
            .map((dataUrl) => ({ id: nanoid(), dataUrl }));

    if (images.length === 0) {
        throw new Error("接口没有返回图片");
    }

    return images;
}

async function resolveImagePayload(config: AiConfig, payload: ImageApiResponse, retryAfter?: string | number) {
    if (!isPendingImagePayload(payload)) return parseImagePayload(payload);
    const taskId = imageTaskId(payload);
    if (!taskId) return parseImagePayload(payload);
    return pollImageTask(config, taskId, retryAfter || payload.retry_after);
}

async function pollImageTask(config: AiConfig, taskId: string, retryAfter?: string | number) {
    let delay = imageTaskDelay(retryAfter);
    for (let attempt = 0; attempt < IMAGE_TASK_MAX_POLLS; attempt += 1) {
        await sleep(delay);
        const response = await axios.get<ImageApiResponse>(imageTaskApiUrl(config, taskId), { headers: aiHeaders(config) });
        if (!isPendingImagePayload(response.data)) return parseImagePayload(response.data);
        delay = imageTaskDelay(retryAfterHeader(response.headers["retry-after"]) || response.data.retry_after);
    }
    throw new Error("图片任务处理超时，请稍后重试");
}

function imageTaskApiUrl(config: AiConfig, taskId: string) {
    const url = aiApiUrl(config, `/image-tasks/${encodeURIComponent(taskId)}`);
    return config.channelMode === "remote" ? `${url}?model=${encodeURIComponent(config.model)}` : url;
}

function isPendingImagePayload(payload: ImageApiResponse) {
    return isPendingImageStatus(payload.status?.toLowerCase());
}

function isPendingImageStatus(status: string | undefined) {
    return status === "queued" || status === "running" || status === "pending" || status === "processing" || status === "in_progress";
}

function imageTaskId(payload: ImageApiResponse) {
    return payload.task_id || payload.id || "";
}

function imageTaskDelay(value: string | number | undefined) {
    const seconds = typeof value === "number" ? value : Number(value);
    const delay = Number.isFinite(seconds) && seconds > 0 ? seconds * 1000 : IMAGE_TASK_DEFAULT_DELAY;
    return Math.min(IMAGE_TASK_MAX_DELAY, Math.max(IMAGE_TASK_DEFAULT_DELAY, delay));
}

function retryAfterHeader(value: unknown) {
    if (typeof value === "string" || typeof value === "number") return value;
    if (Array.isArray(value) && (typeof value[0] === "string" || typeof value[0] === "number")) return value[0];
    return undefined;
}

function sleep(ms: number) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

function normalizeImageUrl(value: string) {
    const markdownLink = value.match(/^\[[^\]]+\]\((https?:\/\/[^)]+)\)$/);
    return markdownLink?.[1] || value;
}

function readApiError(data: { error?: { message?: string } | string; msg?: string; detail?: { error?: { message?: string } | string } | string } | undefined) {
    if (!data) return undefined;
    if (data.msg) return data.msg;
    if (typeof data.error === "string") return data.error;
    if (data.error?.message) return data.error.message;
    if (typeof data.detail === "string") return data.detail;
    if (typeof data.detail?.error === "string") return data.detail.error;
    return data.detail?.error?.message;
}

function readAxiosError(error: unknown, fallback: string) {
    if (axios.isAxiosError<{ error?: { message?: string } | string; msg?: string; code?: number; detail?: { error?: { message?: string } | string } | string }>(error)) {
        const responseData = error.response?.data;
        return readApiError(responseData) || readStatusError(error.response?.status, fallback);
    }
    return error instanceof Error ? error.message : fallback;
}

function readStatusError(status: number | undefined, fallback: string) {
    if (status === 401 || status === 403) return "鉴权失败，请检查 API Key、套餐权限或模型权限";
    if (status === 429) return "请求被限流或额度不足，请稍后重试";
    return status ? `${fallback}：${status}` : fallback;
}

function parseStreamChunk(chunk: string, onDelta: (value: string) => void) {
    let deltaText = "";
    for (const eventBlock of chunk.split("\n\n")) {
        const data = eventBlock
            .split("\n")
            .find((line) => line.startsWith("data: "))
            ?.slice(6);
        if (!data || data === "[DONE]") continue;
        const delta = (JSON.parse(data) as { choices?: Array<{ delta?: { content?: string } }> }).choices?.[0]?.delta?.content || "";
        deltaText += delta;
    }
    if (deltaText) onDelta(deltaText);
}

function withSystemPrompt(config: AiConfig, prompt: string) {
    const systemPrompt = config.systemPrompt.trim();
    return systemPrompt ? `${systemPrompt}\n\n${prompt}` : prompt;
}

function aiApiUrl(config: AiConfig, path: string) {
    return config.channelMode === "remote" ? `/api/v1${path}` : buildApiUrl(config.baseUrl, path);
}

function aiHeaders(config: AiConfig, contentType?: string) {
    const token = useUserStore.getState().token;
    return config.channelMode === "remote"
        ? {
              ...(token ? { Authorization: `Bearer ${token}` } : {}),
              ...(contentType ? { "Content-Type": contentType } : {}),
          }
        : {
              Authorization: `Bearer ${config.apiKey}`,
              ...(contentType ? { "Content-Type": contentType } : {}),
          };
}

function refreshRemoteUser(config: AiConfig) {
    if (config.channelMode === "remote") void useUserStore.getState().hydrateUser();
}

export async function requestVectorizeImage(image: string, mode: "general" | "logo" | "colorMask" = "colorMask") {
    const token = useUserStore.getState().token;
    const value = image.trim();
    const payload = /^https?:\/\//i.test(value) ? { imageUrl: value, mode } : { dataUrl: value, mode };
    const result = await apiPost<Omit<VectorizeImageResult, "dataUrl">>("/api/v1/images/vectorize", payload, token || undefined);
    if (!result.content.slice(0, 512).toLowerCase().includes("<svg")) throw new Error("后端没有返回有效 SVG");
    return {
        ...result,
        dataUrl: `data:${result.mimeType || "image/svg+xml"};charset=utf-8,${encodeURIComponent(result.content)}`,
    };
}

function withSystemMessage(config: AiConfig, messages: ChatCompletionMessage[]) {
    const systemPrompt = config.systemPrompt.trim();
    return systemPrompt ? [{ role: "system" as const, content: systemPrompt }, ...messages] : messages;
}

export async function requestGeneration(config: AiConfig, prompt: string) {
    const n = Math.max(1, Math.min(15, Math.floor(Math.abs(Number(config.count)) || 1)));
    const quality = normalizeQuality(config.quality);
    const requestSize = resolveRequestSize(quality, config.size, config.channelMode === "remote");
    try {
        const response = await axios.post<ImageApiResponse>(
            aiApiUrl(config, "/images/generations"),
            {
                model: config.model,
                prompt: withSystemPrompt(config, prompt),
                n,
                ...(quality ? { quality } : {}),
                ...(requestSize ? { size: requestSize } : {}),
                response_format: IMAGE_RESPONSE_FORMAT,
                output_format: IMAGE_OUTPUT_FORMAT,
                async: true,
            },
            {
                headers: aiHeaders(config, "application/json"),
            },
        );
        const images = await resolveImagePayload(config, response.data, retryAfterHeader(response.headers["retry-after"]));
        refreshRemoteUser(config);
        return images;
    } catch (error) {
        throw new Error(readAxiosError(error, "请求失败"));
    }
}

export async function requestEdit(config: AiConfig, prompt: string, references: ReferenceImage[], mask?: ReferenceImage) {
    const quality = normalizeQuality(config.quality);
    const requestSize = resolveRequestSize(quality, config.size, config.channelMode === "remote");
    const requestPrompt = buildImageReferencePromptText(prompt, references);
    const referenceCompressionQuality = resolveReferenceCompressionQuality();
    const formData = new FormData();
    formData.set("model", config.model);
    formData.set("prompt", withSystemPrompt(config, requestPrompt));
    formData.set("response_format", IMAGE_RESPONSE_FORMAT);
    formData.set("output_format", IMAGE_OUTPUT_FORMAT);
    formData.set("async", "true");
    if (quality) {
        formData.set("quality", quality);
    }
    if (requestSize) {
        formData.set("size", requestSize);
    }
    const files = await Promise.all(references.map(async (image) => dataUrlToJpegFile({ ...image, dataUrl: await imageToDataUrl(image) }, referenceCompressionQuality)));
    files.forEach((file) => formData.append("image", file));
    if (mask) formData.set("mask", await dataUrlToJpegFile({ ...mask, dataUrl: await imageToDataUrl(mask) }, referenceCompressionQuality));

    try {
        const response = await axios.post<ImageApiResponse>(aiApiUrl(config, "/images/edits"), formData, { headers: aiHeaders(config) });
        const images = await resolveImagePayload(config, response.data, retryAfterHeader(response.headers["retry-after"]));
        refreshRemoteUser(config);
        return images;
    } catch (error) {
        throw new Error(readAxiosError(error, "请求失败"));
    }
}

function resolveReferenceCompressionQuality() {
    const quality = Number(useConfigStore.getState().publicSettings?.image?.referenceCompressionQuality ?? DEFAULT_REFERENCE_COMPRESSION_QUALITY);
    if (!Number.isFinite(quality)) return DEFAULT_REFERENCE_COMPRESSION_QUALITY;
    return Math.min(1, Math.max(0.1, quality));
}

export async function requestImageQuestion(config: AiConfig, messages: ChatCompletionMessage[], onDelta: (text: string) => void) {
    let buffer = "";
    let answer = "";
    let processedLength = 0;

    try {
        const response = await axios.post(
            aiApiUrl(config, "/chat/completions"),
            {
                model: config.model,
                messages: withSystemMessage(config, messages),
                stream: true,
            },
            {
                headers: {
                    ...aiHeaders(config, "application/json"),
                } as Record<string, string>,
                responseType: "text",
                onDownloadProgress: (event) => {
                    const responseText = String(event.event?.target?.responseText || "");
                    const nextText = responseText.slice(processedLength);
                    processedLength = responseText.length;
                    buffer += nextText;
                    const chunks = buffer.split("\n\n");
                    buffer = chunks.pop() || "";
                    for (const chunk of chunks) {
                        parseStreamChunk(chunk, (delta) => {
                            answer += delta;
                            onDelta(answer);
                        });
                    }
                },
            },
        );
        if (typeof response.data === "object" && response.data && "code" in response.data && (response.data as { code?: number; msg?: string }).code !== 0) {
            throw new Error((response.data as { msg?: string }).msg || "请求失败");
        }
        if (typeof response.data === "string") {
            let apiError = "";
            try {
                const payload = JSON.parse(response.data) as { code?: number; msg?: string };
                if (typeof payload.code === "number" && payload.code !== 0) {
                    apiError = payload.msg || "请求失败";
                }
            } catch {
                // ignore plain text stream content
            }
            if (apiError) throw new Error(apiError);
        }
        if (buffer) {
            parseStreamChunk(buffer, (delta) => {
                answer += delta;
                onDelta(answer);
            });
        }
    } catch (error) {
        throw new Error(readAxiosError(error, "请求失败"));
    }
    refreshRemoteUser(config);
    return answer || "没有返回内容";
}

export async function fetchImageModels(config: AiConfig) {
    if (config.channelMode === "remote") return config.models;
    try {
        const response = await axios.get<{ data?: Array<{ id?: string }>; error?: { message?: string } }>(buildApiUrl(config.baseUrl, "/models"), {
            headers: {
                Authorization: `Bearer ${config.apiKey}`,
            },
        });
        return (response.data.data || [])
            .map((model) => model.id)
            .filter((id): id is string => Boolean(id))
            .sort((a, b) => a.localeCompare(b));
    } catch (error) {
        throw new Error(readAxiosError(error, "读取模型失败"));
    }
}
