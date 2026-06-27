export type ModelCreditCost = {
    model: string;
    credits: number;
    supports4K?: boolean;
};

const EXTRA_REFERENCE_CREDITS = 1;

export function modelCreditCost(modelCosts: ModelCreditCost[] | undefined, model: string) {
    return modelCosts?.find((item) => item.model === model)?.credits || 0;
}

export function modelSupports4K(modelCosts: ModelCreditCost[] | undefined, model: string) {
    return modelCosts?.find((item) => item.model === model)?.supports4K !== false;
}

export function requestCreditCost(options: { channelMode: string; modelCosts?: ModelCreditCost[]; model: string; count?: string | number; size?: string; quality?: string; referenceCount?: number }) {
    if (options.channelMode !== "remote") return 0;
    const count = Math.max(1, Math.floor(Math.abs(Number(options.count)) || 1));
    const is4K = is4KImageRequest(options.size, options.quality) && modelSupports4K(options.modelCosts, options.model);
    const baseCredits = modelCreditCost(options.modelCosts, options.model) + (is4K ? 3 : 0);
    const extraReferenceCredits = Math.max(0, Math.floor(Number(options.referenceCount) || 0) - 1) * EXTRA_REFERENCE_CREDITS;
    return (baseCredits + extraReferenceCredits) * count;
}

export function canvasGenerationCredits(options: { channelMode: string; modelCosts?: ModelCreditCost[]; model: string; mode: "image" | "text" | "video" | "audio"; count?: string | number; size?: string; quality?: string; imageReferenceCount?: number }) {
    const count = Math.max(1, Math.min(15, Math.floor(Math.abs(Number(options.count)) || 1)));
    return requestCreditCost({
        channelMode: options.channelMode,
        modelCosts: options.modelCosts,
        model: options.model,
        count: options.mode === "image" ? count : 1,
        size: options.mode === "image" ? options.size || "" : "",
        quality: options.mode === "image" ? options.quality || "" : "",
        referenceCount: options.mode === "image" ? options.imageReferenceCount || 0 : 0,
    });
}

export function is4KImageRequest(size = "", quality = "") {
    return quality.trim().toLowerCase() === "4k" || size.trim().toLowerCase().startsWith("4k:");
}
