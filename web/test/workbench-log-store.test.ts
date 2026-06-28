import test from "node:test";
import assert from "node:assert/strict";

import { createWorkbenchSnapshotStore, serializeWorkbenchSnapshot } from "../src/features/workbench/workbench-log-store.ts";
import { WorkbenchSceneId } from "../src/features/workbench/types.ts";

function createMemoryStorage() {
    const values = new Map<string, unknown>();
    return {
        async getItem<T>(key: string) {
            return (values.get(key) as T | undefined) || null;
        },
        async setItem<T>(key: string, value: T) {
            values.set(key, value);
            return value;
        },
        async removeItem(key: string) {
            values.delete(key);
        },
        async iterate<T, R>(iterator: (value: T, key: string) => R) {
            let result: R | undefined;
            for (const [key, value] of values) result = iterator(value as T, key);
            return result;
        },
    };
}

const snapshot = {
    id: "task_1",
    taskId: "task_1",
    createdAt: 1000,
    updatedAt: 2000,
    sceneId: WorkbenchSceneId.Menu,
    sceneName: "菜单设计",
    templateName: "黑底高奢西餐",
    prompt: "生成菜单",
    fields: { shopName: "好味餐厅" },
    references: [
        { id: "ref_1", name: "logo.png", type: "image/png", dataUrl: "data:image/png;base64,abc", storageKey: "image:logo" },
        { id: "ref_2", name: "style.png", type: "image/png", dataUrl: "data:image/png;base64,def" },
    ],
    model: "gpt-image-2",
    quality: "high",
    size: "3:4",
    count: "1",
    resultImageUrls: ["https://cdn.example.com/menu.png"],
    metadata: { source: "workbench" as const, sceneId: WorkbenchSceneId.Menu, sceneName: "菜单设计", templateName: "黑底高奢西餐" },
};

test("workbench snapshots are saved and restored by task id", async () => {
    const store = createWorkbenchSnapshotStore(createMemoryStorage());

    await store.save(snapshot);
    const restored = await store.get("task_1");

    assert.equal(restored?.taskId, "task_1");
    assert.equal(restored?.sceneName, "菜单设计");
    assert.deepEqual(restored?.fields, { shopName: "好味餐厅" });
});

test("workbench snapshots are listed newest first and removed by task id", async () => {
    const store = createWorkbenchSnapshotStore(createMemoryStorage());

    await store.save({ ...snapshot, id: "task_1", taskId: "task_1", updatedAt: 1000 });
    await store.save({ ...snapshot, id: "task_2", taskId: "task_2", updatedAt: 3000 });

    assert.deepEqual(
        (await store.list()).map((item) => item.taskId),
        ["task_2", "task_1"],
    );

    await store.remove("task_2");
    assert.equal(await store.get("task_2"), null);
});

test("workbench snapshots omit inline data urls when storage keys exist", () => {
    const serialized = serializeWorkbenchSnapshot(snapshot);

    assert.equal(serialized.references[0].dataUrl, "");
    assert.equal(serialized.references[0].storageKey, "image:logo");
    assert.equal(serialized.references[1].dataUrl, "data:image/png;base64,def");
});
