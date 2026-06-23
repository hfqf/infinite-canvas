import { apiGet, apiPost } from "@/services/api/request";

export type RechargeOrder = {
    id: string;
    amountYuan: number;
    amountFen: number;
    credits: number;
    memberType: "monthly" | "annual" | "test";
    memberLevel: "standard" | "basic" | "advanced" | "premium" | "test";
    productName: string;
    status: "pending" | "paid";
    codeUrl: string;
    createdAt: string;
    paidAt: string;
};

export async function createRechargeOrder(token: string, amount: number | { amountYuan?: number; amountFen?: number }) {
    return apiPost<RechargeOrder>("/api/v1/recharge/orders", typeof amount === "number" ? { amountYuan: amount } : amount, token);
}

export async function fetchRechargeOrder(token: string, id: string) {
    return apiGet<RechargeOrder>(`/api/v1/recharge/orders/${encodeURIComponent(id)}`, undefined, token);
}
