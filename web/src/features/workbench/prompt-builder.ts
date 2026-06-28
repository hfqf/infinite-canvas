import type { WorkbenchGenerationMetadata, WorkbenchPromptField, WorkbenchReferenceSlot, WorkbenchSceneId } from "./types";

type BuildWorkbenchPromptInput = {
    sceneId: WorkbenchSceneId;
    sceneName: string;
    caption: string;
    subtext?: string;
    presetStyle?: string;
    presetDescription?: string;
    additionalPrompt?: string;
    negativePrompt?: string;
    sceneFields?: WorkbenchPromptField[];
    referenceSlots?: WorkbenchReferenceSlot[];
};

type BuildWorkbenchPromptResult = {
    prompt: string;
    metadata: WorkbenchGenerationMetadata;
};

const unifiedTextRequirements = [
    "文字要求：",
    "- 中文使用标准简体字，思源黑体风格",
    "- 文字清晰可读，无乱码，无无意义字符",
    "- 禁止小字密集排版",
].join("\n");

export function buildWorkbenchPrompt(input: BuildWorkbenchPromptInput): BuildWorkbenchPromptResult {
    const sceneFieldLines = [
        { label: "场景类型", value: input.sceneName },
        { label: "主标题", value: input.caption },
        { label: "副标题", value: input.subtext || "" },
        { label: "风格预设", value: input.presetStyle || "" },
        { label: "预设说明", value: input.presetDescription || "" },
        ...(input.sceneFields || []),
    ]
        .map((field) => ({ label: field.label.trim(), value: field.value.trim() }))
        .filter((field) => field.label && field.value)
        .map((field) => `- ${field.label}：${field.value}`);
    const referenceText = buildReferenceInstructions(input.referenceSlots || []);
    const prompt = [
        sceneFieldLines.length ? `场景编辑参数：\n${sceneFieldLines.join("\n")}` : "",
        referenceText,
        input.additionalPrompt?.trim() ? `补充要求：${input.additionalPrompt.trim()}` : "",
        input.negativePrompt?.trim() ? `避免：${input.negativePrompt.trim()}` : "",
        unifiedTextRequirements,
        "输出要求：高品质商业视觉效果图，文字清晰，材质真实，透视准确，可用于方案沟通。",
    ]
        .filter(Boolean)
        .join("\n");

    return {
        prompt,
        metadata: {
            source: "workbench",
            sceneId: input.sceneId,
            sceneName: input.sceneName,
            templateName: input.presetStyle?.trim() || undefined,
        },
    };
}

function buildReferenceInstructions(slots: WorkbenchReferenceSlot[]) {
    const lines = slots
        .map((slot, index) => ({ label: slot.label.trim(), role: slot.role.trim(), index }))
        .filter((slot) => slot.label && slot.role)
        .map((slot) => `图${slot.index + 1}：${slot.label}，${slot.role}。`);
    if (!lines.length) return "";
    return [
        "参考图使用说明（必须严格按上传顺序理解，不可混淆）：",
        ...lines,
        "任务：",
        "- 主体、风格、背景、光照、版式等来源必须严格遵循上方编号角色。",
        "- 禁止混淆不同图片的身份和用途，不要把参考图角色互换。",
        "- 如果某张图是主体/产品/LOGO，必须保持其核心身份、外形和可识别特征。",
        "- 如果某张图是参考图/现场照片/平面设计图，只提取其对应角色信息，不要直接复制无关内容。",
    ].join("\n");
}
