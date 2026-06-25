import { apiGet, compactApiParams } from "@/services/api/request";
import type { AdminUserQuery } from "@/services/api/admin";

export type AIImageTask = {
    id: string;
    taskId: string;
    userId: string;
    model: string;
    path: string;
    prompt: string;
    credits: number;
    size: string;
    quality: string;
    count: number;
    referenceCount: number;
    status: string;
    imageUrl: string;
    channelName: string;
    channelUrl: string;
    frozenAt: string;
    chargedAt: string;
    releasedAt: string;
    createdAt: string;
    updatedAt: string;
};

export type AIImageTaskListResponse = {
    items: AIImageTask[];
    total: number;
};

export type AIImageTaskQuery = AdminUserQuery & {
    type?: string;
};

export async function fetchMyImageTasks(token: string, query: AIImageTaskQuery = {}) {
    return apiGet<AIImageTaskListResponse>("/api/v1/image-tasks", compactApiParams(query), token);
}
