import { apiPost } from "@/services/api/request";
import type { AuthUser } from "@/services/api/auth";

export async function consumeCanvasToolCredits(token: string, tool: string) {
    return apiPost<AuthUser>("/api/v1/canvas/tool-credits/consume", { tool }, token);
}
