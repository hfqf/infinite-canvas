import { apiGet, apiPost } from "@/services/api/request";

export type RechargeOrder = {
    id: string;
    amountYuan: number;
    amountFen: number;
    credits: number;
    memberType: "monthly" | "annual";
    memberLevel: "standard" | "basic" | "advanced" | "premium";
    productName: string;
    status: "pending" | "paid";
    codeUrl: string;
    createdAt: string;
    paidAt: string;
};

export async function createRechargeOrder(token: string, amountYuan: number) {
    return apiPost<RechargeOrder>("/api/v1/recharge/orders", { amountYuan }, token);
}

export async function fetchRechargeOrder(token: string, id: string) {
    return apiGet<RechargeOrder>(`/api/v1/recharge/orders/${encodeURIComponent(id)}`, undefined, token);
}
