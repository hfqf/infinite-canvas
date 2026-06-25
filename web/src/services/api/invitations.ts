import { apiGet, compactApiParams, type ApiParams } from "@/services/api/request";

export type InvitationRecord = {
    inviterId: string;
    inviterUsername: string;
    inviterDisplayName: string;
    inviteeId: string;
    inviteeUsername: string;
    inviteeDisplayName: string;
    inviteeEmail: string;
    createdAt: string;
};

export type InvitationRecordListResponse = {
    items: InvitationRecord[];
    total: number;
};

export type InvitationRecordQuery = {
    keyword?: string;
    page?: number;
    pageSize?: number;
};

export async function fetchMyInvitations(token: string, query: InvitationRecordQuery = {}) {
    return apiGet<InvitationRecordListResponse>("/api/v1/invitations", compactApiParams(query as ApiParams), token);
}

export async function fetchAdminInvitations(token: string, query: InvitationRecordQuery = {}) {
    return apiGet<InvitationRecordListResponse>("/api/admin/invitations", compactApiParams(query as ApiParams), token);
}
