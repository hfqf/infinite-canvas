const IMAGE_SIZE_STEP = 16;
const IMAGE_4K_LONG_EDGE = 3840;
const IMAGE_2K_PIXELS = 2048 * 2048;
const IMAGE_MAX_RATIO = 3;

export function buildCanvasSuperResolvePrompt() {
    return "请对参考图片进行 AI 超分和高清重建，提升分辨率、边缘锐度、纹理细节和压缩瑕疵修复。保持原始构图、主体身份、颜色、文字位置、图案结构和整体风格不变，不要改变画面内容，不要新增元素、水印或文字。";
}

export function resolveCanvasSuperResolveSize(width: number, height: number, supports4K: boolean) {
    const sourceWidth = Math.max(1, Math.round(width));
    const sourceHeight = Math.max(1, Math.round(height));
    const ratio = Math.max(sourceWidth, sourceHeight) / Math.min(sourceWidth, sourceHeight);
    if (!Number.isFinite(ratio) || ratio > IMAGE_MAX_RATIO) return "auto";
    const scale = supports4K ? IMAGE_4K_LONG_EDGE / Math.max(sourceWidth, sourceHeight) : Math.sqrt(IMAGE_2K_PIXELS / (sourceWidth * sourceHeight));
    const targetWidth = roundToStep(sourceWidth * scale);
    const targetHeight = roundToStep(sourceHeight * scale);
    return `${targetWidth}x${targetHeight}`;
}

function roundToStep(value: number) {
    return Math.max(IMAGE_SIZE_STEP, Math.floor(value / IMAGE_SIZE_STEP) * IMAGE_SIZE_STEP);
}
