export type CanvasImageRemoteResult = {
    url: string;
    storageKey?: string;
    bytes?: number;
    mimeType?: string;
    width?: number;
    height?: number;
};

export type CanvasImageDownloadStrategyResult = {
    shareUrl: string;
    uploaded?: CanvasImageRemoteResult;
    uploadError?: unknown;
};

type CanvasImageDownloadStrategyInput = {
    blob: Blob;
    fileName: string;
    currentUrl?: string;
    isMobile: boolean;
    saveBlob: (blob: Blob, fileName: string) => void;
    ensureRemoteImage?: () => Promise<CanvasImageRemoteResult>;
};

export async function runCanvasImageDownloadStrategy(input: CanvasImageDownloadStrategyInput): Promise<CanvasImageDownloadStrategyResult> {
    input.saveBlob(input.blob, input.fileName);
    if (!input.isMobile) return { shareUrl: "" };

    const existingUrl = shareableImageUrl(input.currentUrl);
    if (existingUrl) return { shareUrl: existingUrl };
    if (!input.ensureRemoteImage) return { shareUrl: "" };

    try {
        const uploaded = await input.ensureRemoteImage();
        return { shareUrl: shareableImageUrl(uploaded.url), uploaded };
    } catch (error) {
        return { shareUrl: "", uploadError: error };
    }
}

export function isMobileBrowser(userAgent = typeof navigator === "undefined" ? "" : navigator.userAgent) {
    return /Android|iPhone|iPad|iPod|Mobile|Windows Phone/i.test(userAgent);
}

export function shareableImageUrl(url?: string) {
    const value = url?.trim() || "";
    return /^https?:\/\//i.test(value) ? value : "";
}
