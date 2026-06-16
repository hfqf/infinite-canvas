import { CanvasNodeType } from "./types";
import type { CanvasNodeMetadata } from "./types";

type CanvasNodeSpec = {
    width: number;
    height: number;
    title: string;
    metadata?: CanvasNodeMetadata;
};

export const NODE_DEFAULT_SIZE = {
    [CanvasNodeType.Image]: { width: 340, height: 240, title: "New Generation" },
    [CanvasNodeType.Text]: { width: 340, height: 240, title: "Note" },
    [CanvasNodeType.Config]: { width: 340, height: 240, title: "生成配置" },
    [CanvasNodeType.Video]: { width: 420, height: 236, title: "Video" },
    [CanvasNodeType.Audio]: { width: 340, height: 120, title: "Audio" },
} satisfies Record<CanvasNodeType, { width: number; height: number; title: string }>;

export const NODE_SPECS = {
    [CanvasNodeType.Image]: {
        ...NODE_DEFAULT_SIZE[CanvasNodeType.Image],
        metadata: { content: "", status: "idle" },
    },
    [CanvasNodeType.Text]: {
        ...NODE_DEFAULT_SIZE[CanvasNodeType.Text],
        metadata: { content: "", status: "idle", fontSize: 14 },
    },
    [CanvasNodeType.Config]: {
        ...NODE_DEFAULT_SIZE[CanvasNodeType.Config],
        metadata: { content: "", status: "idle", generationMode: "image" },
    },
    [CanvasNodeType.Video]: {
        ...NODE_DEFAULT_SIZE[CanvasNodeType.Video],
        metadata: { content: "", status: "idle" },
    },
    [CanvasNodeType.Audio]: {
        ...NODE_DEFAULT_SIZE[CanvasNodeType.Audio],
        metadata: { content: "", status: "idle" },
    },
} satisfies Record<CanvasNodeType, CanvasNodeSpec>;

export type CanvasImagePresetEditId = "vectorize" | "decompose" | "clarify";

export const IMAGE_PRESET_EDIT_CONFIG = {
    vectorize: {
        title: "Vectorized Image",
        label: "转矢量",
        panelLabel: "转矢量",
        tooltip: "生成矢量插画风格图片",
        error: "转矢量失败",
        prompt: "将参考图转换为干净的矢量插画风格，保留主体轮廓和关键细节，使用清晰边缘、纯色块、少量渐变，整体像可用于图标、贴纸或平面设计的矢量稿，背景保持简洁。",
    },
    decompose: {
        title: "Decomposed Image",
        label: "平面拆解",
        panelLabel: "平面拆解",
        tooltip: "生成平面元素拆解图",
        error: "平面拆解失败",
        prompt: "将参考图拆解为平面设计元素展示，分离主体、背景、装饰元素、文字区域和关键形状，用整洁排版展示这些元素，保持原图的主要视觉信息和色彩关系，生成一张清晰的元素拆解图。",
    },
    clarify: {
        title: "Clear Image",
        label: "变高清",
        panelLabel: "模糊变高清",
        tooltip: "修复模糊并提升清晰度",
        error: "变高清失败",
        prompt: "修复参考图的模糊、噪点和压缩痕迹，提升清晰度、边缘质量和细节表现，保持原始构图、主体身份、颜色和风格不变，不要改变画面内容。",
    },
} satisfies Record<
    CanvasImagePresetEditId,
    {
        title: string;
        label: string;
        panelLabel: string;
        tooltip: string;
        error: string;
        prompt: string;
    }
>;

export function getNodeSpec(type: CanvasNodeType) {
    return NODE_SPECS[type];
}
