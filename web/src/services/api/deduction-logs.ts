import { apiGet, compactApiParams } from "@/services/api/request";
import type { AdminCreditLog, AdminCreditLogListResponse, AdminUserQuery } from "@/services/api/admin";

export type CreditLog = AdminCreditLog;
export type CreditLogListResponse = AdminCreditLogListResponse;
export type CreditLogQuery = AdminUserQuery;

export async function fetchMyDeductionLogs(token: string, query: CreditLogQuery = {}) {
    return apiGet<CreditLogListResponse>("/api/v1/deduction-logs", compactApiParams(query), token);
}
