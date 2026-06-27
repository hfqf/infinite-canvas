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

export function is4KImageRequest(size = "", quality = "") {
    if (quality.trim().toLowerCase() === "4k") return true;
    const value = size.trim().toLowerCase();
    if (value.includes("4k")) return true;
    const match = value.match(/^(\d+)\s*[x×*]\s*(\d+)$/);
    if (!match) return false;
    return Number(match[1]) >= 3840 || Number(match[2]) >= 3840;
}
