"use client";

import localforage from "localforage";

import { nanoid } from "nanoid";
import { readImageMeta } from "@/lib/image-utils";
import { useUserStore } from "@/stores/use-user-store";

export type UploadedImage = {
    url: string;
    storageKey: string;
    width: number;
    height: number;
    bytes: number;
    mimeType: string;
};

const store = localforage.createInstance({ name: "infinite-canvas", storeName: "image_files" });
const objectUrls = new Map<string, string>();

export async function uploadImage(input: string | Blob): Promise<UploadedImage> {
    const blob = typeof input === "string" ? await fetchImageBlob(input) : input;
    const remote = await uploadImageToOSS(blob);
    if (remote) {
        const meta = await readImageMeta(remote.url);
        return { ...remote, width: meta.width, height: meta.height, bytes: remote.bytes || blob.size, mimeType: remote.mimeType || blob.type || meta.mimeType };
    }
    const storageKey = `image:${nanoid()}`;
    await store.setItem(storageKey, blob);
    const url = URL.createObjectURL(blob);
    objectUrls.set(storageKey, url);
    const meta = await readImageMeta(url);
    return { url, storageKey, width: meta.width, height: meta.height, bytes: blob.size, mimeType: blob.type || meta.mimeType };
}

export async function resolveImageUrl(storageKey?: string, fallback = "") {
    if (!storageKey) return fallback;
    const cached = objectUrls.get(storageKey);
    if (cached) return cached;
    const blob = await store.getItem<Blob>(storageKey);
    if (!blob) return fallback;
    const url = URL.createObjectURL(blob);
    objectUrls.set(storageKey, url);
    return url;
}

export async function getImageBlob(storageKey: string) {
    return store.getItem<Blob>(storageKey);
}

export async function setImageBlob(storageKey: string, blob: Blob) {
    await store.setItem(storageKey, blob);
    const url = URL.createObjectURL(blob);
    objectUrls.set(storageKey, url);
    return url;
}

export async function imageToDataUrl(image: { url?: string; dataUrl?: string; storageKey?: string }) {
    let url = image.dataUrl;
    // 防御：如果 dataUrl 被错误地存成了 oss: 存储键（老画布 / 字段误用），
    // 用 image.url（公网 URL）或 resolveImageUrl(storageKey) 兑底，绝不让 oss: 进 fetch。
    if (url && url.startsWith("oss:")) {
        url = image.url || (await resolveImageUrl(image.storageKey, "")) || "";
    }
    url = url || (await resolveImageUrl(image.storageKey, image.url || ""));
    if (!url || url.startsWith("data:")) return url;
    return blobToDataUrl(await fetchImageBlob(url));
}

export async function deleteStoredImages(keys: Iterable<string>) {
    await Promise.all(
        Array.from(new Set(keys)).map(async (key) => {
            const url = objectUrls.get(key);
            if (url) URL.revokeObjectURL(url);
            objectUrls.delete(key);
            await store.removeItem(key);
        }),
    );
}

export async function cleanupUnusedImages(usedData: unknown) {
    const usedKeys = collectImageStorageKeys(usedData);
    const unused: string[] = [];
    await store.iterate((_value, key) => {
        if (!usedKeys.has(key)) unused.push(key);
    });
    await deleteStoredImages(unused);
}

export function collectImageStorageKeys(value: unknown, keys = new Set<string>()) {
    if (!value || typeof value !== "object") return keys;
    if ("storageKey" in value && typeof value.storageKey === "string" && value.storageKey.startsWith("image:")) keys.add(value.storageKey);
    Object.values(value).forEach((item) => (Array.isArray(item) ? item.forEach((child) => collectImageStorageKeys(child, keys)) : collectImageStorageKeys(item, keys)));
    return keys;
}

function blobToDataUrl(blob: Blob) {
    return new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(String(reader.result || ""));
        reader.onerror = () => reject(new Error("读取图片失败"));
        reader.readAsDataURL(blob);
    });
}

async function fetchImageBlob(url: string) {
    const response = await fetch(proxiedImageUrl(url));
    if (!response.ok) throw new Error("读取图片失败");
    return response.blob();
}

async function uploadImageToOSS(blob: Blob): Promise<Omit<UploadedImage, "width" | "height"> | null> {
    const token = useUserStore.getState().token;
    if (!token) return null;
    const formData = new FormData();
    const ext = imageFileExtension(blob.type);
    formData.set("file", new File([blob], `canvas-image.${ext}`, { type: blob.type || "image/png" }));
    const response = await fetch("/api/v1/images/uploads", {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
        body: formData,
    });
    const payload = (await response.json().catch(() => null)) as { code?: number; data?: Omit<UploadedImage, "width" | "height">; msg?: string } | null;
    if (!response.ok || !payload || payload.code !== 0 || !payload.data?.url) {
        throw new Error(payload?.msg || "图片上传 OSS 失败");
    }
    return payload.data;
}

function imageFileExtension(mimeType: string) {
    switch (mimeType.toLowerCase()) {
        case "image/jpeg":
        case "image/jpg":
            return "jpg";
        case "image/webp":
            return "webp";
        case "image/gif":
            return "gif";
        case "image/bmp":
            return "bmp";
        default:
            return "png";
    }
}

function proxiedImageUrl(url: string) {
    if (/^https?:\/\//i.test(url)) return `/image-proxy?url=${encodeURIComponent(url)}`;
    if (url.startsWith("oss:")) return `/api/v1/oss-image?key=${encodeURIComponent(url)}`;
    return url;
}
