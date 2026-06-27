import { isBusiness4KImageRequest } from "@/constant/image-generation-constraints";

export type ImageWaitInfo = {
    seconds: number;
    baseSeconds: number;
    referenceCount: number;
    is4K: boolean;
};

export function imageGenerationWaitInfo(options: { size?: string; quality?: string; referenceCount?: number }): ImageWaitInfo {
    const referenceCount = Math.max(0, Math.floor(Number(options.referenceCount) || 0));
    const is4K = is4KImageRequest(options.size || "", options.quality || "");
    const baseSeconds = is4K ? 180 : 120;
    return {
        seconds: baseSeconds + referenceCount * 60,
        baseSeconds,
        referenceCount,
        is4K,
    };
}

export function imageWaitDetailText(info: ImageWaitInfo) {
    if (!info.referenceCount) return `基础 ${info.baseSeconds} 秒`;
    return `基础 ${info.baseSeconds} 秒 + ${info.referenceCount} 张参考图 x 60 秒`;
}

function is4KImageRequest(size: string, quality: string) {
    return isBusiness4KImageRequest(size, quality);
}
