import type { ComponentProps } from "react";
import { Zap } from "lucide-react";

export function CreditSymbol({ className, ...props }: ComponentProps<"span">) {
    return (
        <span {...props} className={`inline-flex items-center justify-center ${className || ""}`}>
            <Zap className="size-[1em] fill-current" strokeWidth={2.4} />
        </span>
    );
}

export type ModelCreditCost = {
    model: string;
    credits: number;
};

export function modelCreditCost(modelCosts: ModelCreditCost[] | undefined, model: string) {
    return modelCosts?.find((item) => item.model === model)?.credits || 0;
}

export function requestCreditCost(options: { channelMode: string; modelCosts?: ModelCreditCost[]; model: string; count?: string | number; size?: string; quality?: string }) {
    if (options.channelMode !== "remote") return 0;
    const count = Math.max(1, Math.floor(Math.abs(Number(options.count)) || 1));
    const credits = is4KImageRequest(options.size, options.quality) ? 6 : modelCreditCost(options.modelCosts, options.model);
    return credits * count;
}

function is4KImageRequest(size = "", quality = "") {
    if (quality.trim().toLowerCase() === "4k") return true;
    const value = size.trim().toLowerCase();
    if (value.includes("4k")) return true;
    const match = value.match(/^(\d+)\s*[x×*]\s*(\d+)$/);
    if (!match) return false;
    return Number(match[1]) >= 3840 || Number(match[2]) >= 3840;
}
