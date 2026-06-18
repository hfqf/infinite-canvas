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

export type CanvasImagePresetEditId = "vectorize" | "logoVectorize" | "decompose" | "clarify";

export const IMAGE_PRESET_EDIT_CONFIG = {
    vectorize: {
        title: "Vectorized Image",
        label: "转矢量",
        panelLabel: "转矢量",
        tooltip: "生成产品设计矢量化还原图",
        error: "转矢量失败",
        prompt: "请基于参考图片，重新绘制为适合后续转 SVG / AI / EPS 的高清纯色矢量友好设计稿。\n\n核心目标：尽量保持原图的视觉识别效果和设计结构，但把画面整理成更适合自动描摹的纯色、清晰、闭合形状图。不要生成真实摄影图，不要重新设计。\n\n适用对象包括工业产品、消费电子、家具家电、包装、产品 LOGO、品牌标识、门头招牌、店招灯箱、展陈导视和商业空间外立面。\n\n要求：\n1. 严格保留原图的整体比例、视角、构图、外轮廓、结构分件、接缝、倒角、圆角、按钮、接口、孔位、螺丝、图案、文字排布、LOGO/标识位置、门头字牌比例、招牌边框和灯箱结构。\n2. 保持原有文字和标识的相对位置、大小、字形轮廓和排版关系，不新增文字，不改字，不替换品牌元素。\n3. 将复杂照片质感简化为干净的平面色块、闭合轮廓和少量必要线条；边缘锐利，线条连续，线宽尽量统一。\n4. 使用有限色阶表达材质差异，例如金属、塑料、玻璃、亚克力、发光字、喷绘、木纹、橡胶、织物等只保留主要颜色关系和结构层次，不保留细碎纹理。\n5. 背景为纯白或透明风格，主体居中，轮廓完整，四周留白均匀，适合后续 Illustrator / Figma / Inkscape 自动描摹。\n6. 如果原图是产品照片，保持原视角或轻微整理为稳定产品设计视角；如果原图是门头、LOGO、包装或平面图，必须保持原版式和透视关系。\n\n避免：照片级渲染、真实反光、复杂渐变、强投影、纹理噪点、脏背景、模糊边缘、锯齿、手绘抖动、卡通化、贴纸化、图标化、重新设计、改变比例、遗漏接口/按钮/分割线、修改文字、添加水印。",
    },
    logoVectorize: {
        title: "Logo SVG",
        label: "Logo矢量",
        panelLabel: "Logo矢量",
        tooltip: "生成可导入 CDR 的 Logo/文字 SVG 初稿",
        error: "Logo 矢量失败",
        prompt: "",
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
