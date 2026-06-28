"use client";

import localforage from "localforage";

import type { WorkbenchGenerationMetadata, WorkbenchSceneId } from "./types";

export type WorkbenchSnapshotReference = {
    id: string;
    name: string;
    type?: string;
    dataUrl?: string;
    storageKey?: string;
    slotFieldName?: string;
    role?: string;
};

export type WorkbenchGenerationSnapshot = {
    id: string;
    taskId: string;
    createdAt: number;
    updatedAt: number;
    sceneId: WorkbenchSceneId;
    sceneName: string;
    templateName?: string;
    prompt: string;
    fields: Record<string, string>;
    references: WorkbenchSnapshotReference[];
    model: string;
    quality: string;
    size: string;
    count: string;
    resultImageUrls?: string[];
    metadata: WorkbenchGenerationMetadata;
};

type WorkbenchSnapshotStorage = {
    getItem<T>(key: string): Promise<T | null>;
    setItem<T>(key: string, value: T): Promise<T>;
    removeItem(key: string): Promise<void>;
    iterate<T, R>(iterator: (value: T, key: string) => R): Promise<R | undefined>;
};

const workbenchSnapshotStore = localforage.createInstance({ name: "infinite-canvas", storeName: "workbench_generation_snapshots" });

export function createWorkbenchSnapshotStore(storage: WorkbenchSnapshotStorage) {
    return {
        async save(snapshot: WorkbenchGenerationSnapshot) {
            await storage.setItem(snapshot.taskId, serializeWorkbenchSnapshot(snapshot));
        },
        async get(taskId: string) {
            return storage.getItem<WorkbenchGenerationSnapshot>(taskId);
        },
        async list() {
            const snapshots: WorkbenchGenerationSnapshot[] = [];
            await storage.iterate<WorkbenchGenerationSnapshot, void>((value) => {
                if (value && value.taskId) snapshots.push(value);
            });
            return snapshots.sort((a, b) => b.updatedAt - a.updatedAt);
        },
        async remove(taskId: string) {
            await storage.removeItem(taskId);
        },
    };
}

export const workbenchSnapshots = createWorkbenchSnapshotStore(workbenchSnapshotStore);

export async function saveWorkbenchGenerationSnapshot(snapshot: WorkbenchGenerationSnapshot) {
    await workbenchSnapshots.save(snapshot);
}

export async function getWorkbenchGenerationSnapshot(taskId: string) {
    return workbenchSnapshots.get(taskId);
}

export async function listWorkbenchGenerationSnapshots() {
    return workbenchSnapshots.list();
}

export async function deleteWorkbenchGenerationSnapshot(taskId: string) {
    await workbenchSnapshots.remove(taskId);
}

export function serializeWorkbenchSnapshot(snapshot: WorkbenchGenerationSnapshot): WorkbenchGenerationSnapshot {
    return {
        ...snapshot,
        references: snapshot.references.map((item) => ({ ...item, dataUrl: item.storageKey ? "" : item.dataUrl })),
    };
}
