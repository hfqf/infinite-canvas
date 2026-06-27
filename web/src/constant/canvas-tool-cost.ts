export type CanvasToolCost = {
    tool: string;
    credits: number;
};

export const canvasToolbarToolOptions = [
    { id: "info", label: "信息" },
    { id: "delete", label: "删除" },
    { id: "retry", label: "重试" },
    { id: "saveAsset", label: "存素材" },
    { id: "download", label: "下载" },
    { id: "edit", label: "编辑" },
    { id: "editText", label: "编辑文字" },
    { id: "generateImage", label: "文本生图" },
    { id: "config", label: "生成配置" },
    { id: "decreaseFont", label: "缩小字号" },
    { id: "increaseFont", label: "增大字号" },
    { id: "uploadImage", label: "上传图片" },
    { id: "uploadVideo", label: "上传视频" },
    { id: "uploadAudio", label: "上传音频" },
    { id: "copyPrompt", label: "复制提示词" },
    { id: "reversePrompt", label: "反推提示词" },
    { id: "replace", label: "替换图片" },
    { id: "resize", label: "锁比例" },
    { id: "maskEdit", label: "局部编辑" },
    { id: "crop", label: "裁剪" },
    { id: "split", label: "切图" },
    { id: "upscale", label: "放大" },
    { id: "superResolve", label: "AI 超分" },
    { id: "vectorize", label: "转矢量" },
    { id: "decompose", label: "平面拆解" },
    { id: "clarify", label: "模糊变高清" },
    { id: "angle", label: "多角度" },
    { id: "view", label: "查看大图" },
] as const;

export function canvasToolCostCredits(toolCosts: CanvasToolCost[] | undefined, tool: string) {
    return toolCosts?.find((item) => item.tool === tool)?.credits || 0;
}

export function canvasToolTooltipTitle(title: string, toolCredits: number, modelCredits = 0) {
    const tool = Math.max(0, toolCredits);
    const model = Math.max(0, modelCredits);
    if (model > 0) return `${title} · 工具服务费 ${tool} 积分 · 模型消耗 ${model} 积分 · 合计 ${tool + model} 积分`;
    return `${title} · 消耗 ${tool} 积分`;
}
