import { apiDelete, apiGet, apiPost, compactApiParams } from "@/services/api/request";

export type Prompt = {
    id: string;
    title: string;
    coverUrl: string;
    prompt: string;
    tags: string[];
    category: string;
    githubUrl: string;
    preview: string;
    createdAt: string;
    updatedAt: string;
};

export const ALL_PROMPTS_OPTION = "全部";

export type PromptListResponse = {
    items: Prompt[];
    tags: string[];
    categories: string[];
    total: number;
};

export type UserPrompt = {
    id: string;
    userId: string;
    title: string;
    prompt: string;
    tags: string[];
    category: string;
    sortOrder: number;
    source: string;
    createdAt: string;
    updatedAt: string;
};

export type UserPromptListResponse = {
    items: UserPrompt[];
    tags: string[];
    categories: string[];
    total: number;
};

export type UserPromptPayload = Partial<Pick<UserPrompt, "id" | "title" | "category" | "sortOrder" | "source">> & {
    prompt: string;
    tags?: string[];
};

export async function fetchPrompts({ keyword = "", tag = [], category = ALL_PROMPTS_OPTION, page, pageSize }: { keyword?: string; tag?: string[]; category?: string; page?: number; pageSize?: number } = {}) {
    return apiGet<PromptListResponse>(
        "/api/prompts",
        compactApiParams({
            ...(keyword ? { keyword } : {}),
            ...(tag.length ? { tag } : {}),
            ...(category !== ALL_PROMPTS_OPTION ? { category } : {}),
            ...(page ? { page } : {}),
            ...(pageSize ? { pageSize } : {}),
        }),
    );
}

export async function fetchUserPrompts(
    token: string,
    { keyword = "", tag = [], category = ALL_PROMPTS_OPTION, page, pageSize }: { keyword?: string; tag?: string[]; category?: string; page?: number; pageSize?: number } = {},
) {
    return apiGet<UserPromptListResponse>(
        "/api/v1/user-prompts",
        compactApiParams({
            ...(keyword ? { keyword } : {}),
            ...(tag.length ? { tag } : {}),
            ...(category !== ALL_PROMPTS_OPTION ? { category } : {}),
            ...(page ? { page } : {}),
            ...(pageSize ? { pageSize } : {}),
        }),
        token,
    );
}

export async function saveUserPrompt(token: string, payload: UserPromptPayload) {
    return apiPost<UserPrompt>("/api/v1/user-prompts", payload, token);
}

export async function deleteUserPrompt(token: string, id: string) {
    return apiDelete<{ ok: boolean }>(`/api/v1/user-prompts/${encodeURIComponent(id)}`, token);
}

export async function reorderUserPrompts(token: string, items: Array<{ id: string; sortOrder: number }>) {
    return apiPost<{ ok: boolean }>("/api/v1/user-prompts/reorder", { items }, token);
}

export function userPromptToPrompt(item: UserPrompt): Prompt {
    return {
        id: item.id,
        title: item.title,
        coverUrl: "",
        prompt: item.prompt,
        tags: item.tags,
        category: item.category,
        githubUrl: "",
        preview: "",
        createdAt: item.createdAt,
        updatedAt: item.updatedAt,
    };
}

export function formatPromptDate(value: string) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? "" : new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" }).format(date);
}
