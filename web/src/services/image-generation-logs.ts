"use client";

import localforage from "localforage";
import { nanoid } from "nanoid";

import type { AiConfig } from "@/stores/use-config-store";
import type { UploadedImage } from "@/services/image-storage";
import type { ReferenceImage } from "@/types/image";

type GeneratedImageLogItem = {
    id: string;
    dataUrl: string;
    storageKey?: string;
    durationMs: number;
    width: number;
    height: number;
    bytes: number;
    mimeType?: string;
};

type ImageGenerationLogConfig = Pick<AiConfig, "model" | "imageModel" | "quality" | "size" | "count">;

type ImageGenerationLog = {
    id: string;
    createdAt: number;
    title: string;
    prompt: string;
    time: string;
    model: string;
    config: ImageGenerationLogConfig;
    references: ReferenceImage[];
    durationMs: number;
    successCount: number;
    failCount: number;
    imageCount: number;
    size: string;
    quality: string;
    status: "成功" | "失败";
    images: GeneratedImageLogItem[];
    thumbnails: string[];
};

type SaveImageGenerationLogInput = {
    prompt: string;
    model: string;
    config: ImageGenerationLogConfig;
    references?: ReferenceImage[];
    durationMs: number;
    images: UploadedImage[];
    failCount?: number;
};

const imageGenerationLogStore = localforage.createInstance({ name: "infinite-canvas", storeName: "image_generation_logs" });

export async function saveImageGenerationLog(input: SaveImageGenerationLogInput) {
    if (typeof window === "undefined" || input.images.length === 0) return;
    const log = buildImageGenerationLog(input);
    await imageGenerationLogStore.setItem(log.id, serializeImageGenerationLog(log));
}

function buildImageGenerationLog(input: SaveImageGenerationLogInput): ImageGenerationLog {
    const now = Date.now();
    const images = input.images.map((image) => ({
        id: nanoid(),
        dataUrl: image.url,
        storageKey: image.storageKey,
        durationMs: input.durationMs,
        width: image.width,
        height: image.height,
        bytes: image.bytes,
        mimeType: image.mimeType,
    }));
    const successCount = images.length;
    const failCount = Math.max(0, input.failCount || 0);
    return {
        id: nanoid(),
        createdAt: now,
        title: input.prompt.slice(0, 12) || "未命名",
        prompt: input.prompt,
        time: new Date(now).toLocaleString("zh-CN", { hour12: false }),
        model: input.model,
        config: input.config,
        references: input.references || [],
        durationMs: input.durationMs,
        successCount,
        failCount,
        imageCount: Number(input.config.count) || successCount,
        size: input.config.size,
        quality: input.config.quality,
        status: successCount ? "成功" : "失败",
        images,
        thumbnails: images.map((image) => image.dataUrl).filter(Boolean),
    };
}

function serializeImageGenerationLog(log: ImageGenerationLog): ImageGenerationLog {
    return {
        ...log,
        references: log.references.map((item) => ({ ...item, dataUrl: item.storageKey ? "" : item.dataUrl })),
        images: log.images.map((image) => ({ ...image, dataUrl: image.storageKey ? "" : image.dataUrl })),
        thumbnails: [],
    };
}
