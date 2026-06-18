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
    [CanvasNodeType.Svg]: { width: 340, height: 240, title: "SVG" },
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
    [CanvasNodeType.Svg]: {
        ...NODE_DEFAULT_SIZE[CanvasNodeType.Svg],
        metadata: { content: "", status: "idle", mimeType: "image/svg+xml" },
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
        tooltip: "生成产品设计矢量化还原图",
        error: "转矢量失败",
        prompt: "把参考图高保真转换为产品设计师可用的矢量化视觉稿，核心目标是尽量保持原来的视觉效果，而不是重新设计、风格化重绘或创作新图。适用对象包括工业产品、消费电子、家具家电、包装、产品 LOGO、品牌标识、门头招牌、店招灯箱、展陈导视和商业空间外立面。必须严格临摹原图的整体比例、透视角度、外轮廓、结构分件、接缝、倒角、圆角、按钮、孔位、接口、螺丝、纹理、材质、图案、文字排布、LOGO/标识位置、门头字牌比例、招牌边框、灯箱结构和所有可见关键细节。不要擅自重设计、不要替换品牌元素、不要改字、不要改布局、不要改颜色关系、不要改变原图的光影气质、不要把复杂产品简化成普通图标或卡通贴纸。保留原有材质特征和明暗关系，例如金属、塑料、玻璃、亚克力、发光字、喷绘、木纹、橡胶、织物、磨砂、抛光、高光和阴影；表现方式使用干净清晰的矢量边缘、平整色块、精细线稿和少量柔和渐变，但整体观感要贴近原图。背景保持简洁，主体清晰，输出应像产品说明书、专利图、CMF 提案、品牌视觉稿、门头施工效果图或工业设计展示板中的精致矢量化还原稿。",
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
