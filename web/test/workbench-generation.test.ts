import test from "node:test";
import assert from "node:assert/strict";

import { workbenchRemoteImageConfig } from "../src/features/workbench/remote-config.ts";
import { generateWorkbenchImage } from "../src/features/workbench/workbench-generation.ts";

const baseConfig = {
    channelMode: "local",
    baseUrl: "https://local.example.com",
    apiKey: "local-key",
    model: "text-model",
    imageModel: "gpt-image-2",
    videoModel: "",
    textModel: "",
    audioModel: "",
    audioVoice: "",
    audioFormat: "",
    audioSpeed: "",
    audioInstructions: "",
    videoSeconds: "",
    vquality: "",
    videoGenerateAudio: "",
    videoWatermark: "",
    systemPrompt: "",
    models: [],
    imageModels: ["gpt-image-2"],
    videoModels: [],
    textModels: [],
    audioModels: [],
    quality: "auto",
    size: "1:1",
    count: "1",
    canvasImageCount: "1",
} as const;

test("forces workbench image config to remote channel and image model", () => {
    const config = workbenchRemoteImageConfig(baseConfig);

    assert.equal(config.channelMode, "remote");
    assert.equal(config.model, "gpt-image-2");
});

test("uses selected workbench model when provided", () => {
    const config = workbenchRemoteImageConfig(baseConfig, "seedream-image");

    assert.equal(config.channelMode, "remote");
    assert.equal(config.model, "seedream-image");
});

test("workbench generation calls text-to-image without references", async () => {
    const calls: string[] = [];
    const result = await generateWorkbenchImage({
        config: baseConfig,
        prompt: "poster",
        metadata: { source: "workbench", sceneId: "POSTER", sceneName: "海报设计", templateName: "酸性设计" },
        generate: async (config, prompt, metadata) => {
            calls.push(`${config.channelMode}:${config.model}:${prompt}:${metadata?.sceneId}`);
            return { taskId: "task_generate", images: [{ id: "image_1", dataUrl: "https://cdn.example.com/a.png" }] };
        },
    });

    assert.deepEqual(calls, ["remote:gpt-image-2:poster:POSTER"]);
    assert.equal(result.taskId, "task_generate");
});

test("workbench generation calls image edit when references exist", async () => {
    const calls: string[] = [];
    const result = await generateWorkbenchImage({
        config: baseConfig,
        prompt: "menu",
        references: [{ id: "ref_1", name: "menu.png", type: "image/png", dataUrl: "data:image/png;base64,abc" }],
        metadata: { source: "workbench", sceneId: "MENU", sceneName: "菜单设计", templateName: "黑底高奢西餐" },
        edit: async (config, prompt, references, mask, metadata) => {
            calls.push(`${config.channelMode}:${config.model}:${prompt}:${references.length}:${mask ? "mask" : "none"}:${metadata?.sceneId}`);
            return { taskId: "task_edit", images: [{ id: "image_1", dataUrl: "https://cdn.example.com/b.png" }] };
        },
    });

    assert.deepEqual(calls, ["remote:gpt-image-2:menu:1:none:MENU"]);
    assert.equal(result.taskId, "task_edit");
});
