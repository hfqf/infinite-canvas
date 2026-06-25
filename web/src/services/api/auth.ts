import { apiGet, apiPost } from "@/services/api/request";

export const AUTH_TOKEN_KEY = "infinite-canvas-auth-token-v1";

export type UserRole = "guest" | "user" | "admin";
export type MemberType = "" | "monthly" | "annual" | "test";
export type MemberLevel = "" | "standard" | "basic" | "advanced" | "premium" | "test";

export type AuthUser = {
    id: string;
    username: string;
    email: string;
    displayName: string;
    avatarUrl: string;
    role: UserRole;
    credits: number;
    frozenCredits: number;
    memberType: MemberType;
    memberLevel: MemberLevel;
    lastRechargeAmountYuan: number;
    lastRechargedAt: string;
    affCode: string;
    affCount: number;
    createdAt: string;
    updatedAt: string;
};

export type AuthSession = {
    token: string;
    user: AuthUser;
};

export type AuthPayload = {
    username: string;
    password: string;
    email?: string;
    verificationCode?: string;
    inviteCode?: string;
};

export type VerificationCodeResult = {
    expiresInSeconds: number;
    debugCode?: string;
};

export async function login(payload: AuthPayload) {
    return apiPost<AuthSession>("/api/auth/login", payload);
}

export async function register(payload: AuthPayload) {
    return apiPost<AuthSession>("/api/auth/register", payload);
}

export async function requestVerificationCode(email: string, purpose = "register") {
    return apiPost<VerificationCodeResult>("/api/auth/verification-code", { email, purpose });
}

export async function fetchCurrentUser(token?: string) {
    return apiGet<AuthUser>("/api/auth/me", undefined, token);
}
