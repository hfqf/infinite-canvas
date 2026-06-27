const IMAGE_SIZE_STEP = 16;
const IMAGE_MIN_PIXELS = 655360;
const IMAGE_MAX_PIXELS = 8294400;
const IMAGE_MAX_LONG_EDGE = 3840;
const IMAGE_MAX_RATIO = 3;

export function buildCanvasSuperResolvePrompt() {
    return "请对参考图片进行 AI 超分和高清重建，提升分辨率、边缘锐度、纹理细节和压缩瑕疵修复。保持原始构图、主体身份、颜色、文字位置、图案结构和整体风格不变，不要改变画面内容，不要新增元素、水印或文字。";
}

export function resolveCanvasSuperResolveSize(width: number, height: number, supports4K: boolean) {
    const sourceWidth = Math.max(1, Math.round(width));
    const sourceHeight = Math.max(1, Math.round(height));
    const longSide = Math.max(sourceWidth, sourceHeight);
    const shortSide = Math.min(sourceWidth, sourceHeight);
    const ratio = longSide / shortSide;
    if (!Number.isFinite(ratio) || ratio > IMAGE_MAX_RATIO) return "auto";

    const targetPixels = supports4K ? IMAGE_MAX_PIXELS : 2048 * 2048;
    let scale = supports4K ? IMAGE_MAX_LONG_EDGE / longSide : Math.sqrt(targetPixels / (sourceWidth * sourceHeight));
    scale = Math.min(scale, IMAGE_MAX_LONG_EDGE / longSide);
    scale = Math.min(scale, Math.sqrt(IMAGE_MAX_PIXELS / (sourceWidth * sourceHeight)));
    scale = Math.max(scale, Math.sqrt(IMAGE_MIN_PIXELS / (sourceWidth * sourceHeight)));

    let targetWidth = floorToStep(sourceWidth * scale);
    let targetHeight = floorToStep(sourceHeight * scale);
    while (targetWidth * targetHeight > IMAGE_MAX_PIXELS || Math.max(targetWidth, targetHeight) > IMAGE_MAX_LONG_EDGE) {
        targetWidth = Math.max(IMAGE_SIZE_STEP, targetWidth - IMAGE_SIZE_STEP);
        targetHeight = Math.max(IMAGE_SIZE_STEP, floorToStep(targetWidth / (sourceWidth / sourceHeight)));
    }
    const size = `${targetWidth}x${targetHeight}`;
    return supports4K ? `4k:${size}` : size;
}

function floorToStep(value: number) {
    return Math.max(IMAGE_SIZE_STEP, Math.floor(value / IMAGE_SIZE_STEP) * IMAGE_SIZE_STEP);
}
