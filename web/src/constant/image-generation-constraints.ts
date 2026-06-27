export const IMAGE_SIZE_STEP = 16;
export const IMAGE_MIN_PIXELS = 655360;
export const IMAGE_MAX_PIXELS = 8294400;
export const IMAGE_MAX_LONG_EDGE = 3840;
export const IMAGE_MAX_RATIO = 3;
export const IMAGE_4K_SIZE_PREFIX = "4k:";

export const imageGenerationQualityOptions = ["low", "medium", "high", "auto"] as const;
export const commonImageGenerationSizes = ["1024x1024", "1536x1024", "1024x1536", "1088x1920", "2048x2048", "2048x1152", "3840x2160", "2160x3840", "auto"] as const;

export function mark4KImageSize(size: string) {
    const value = stripImageSizeMarker(size);
    return value === "auto" ? value : `${IMAGE_4K_SIZE_PREFIX}${value}`;
}

export function stripImageSizeMarker(size = "") {
    return size.trim().toLowerCase().startsWith(IMAGE_4K_SIZE_PREFIX) ? size.trim().slice(IMAGE_4K_SIZE_PREFIX.length) : size.trim();
}

export function isBusiness4KImageRequest(size = "", quality = "") {
    return quality.trim().toLowerCase() === "4k" || size.trim().toLowerCase().startsWith(IMAGE_4K_SIZE_PREFIX);
}

export function parseImageDimensions(size: string) {
    const match = stripImageSizeMarker(size).match(/^(\d+)x(\d+)$/i);
    if (!match) return null;
    return { width: Number(match[1]), height: Number(match[2]) };
}

export function validateImageGenerationSize(width: number, height: number) {
    if (!Number.isInteger(width) || !Number.isInteger(height) || width <= 0 || height <= 0) throw new Error("图像尺寸必须是正整数，例如 1024x1024");
    if (width % IMAGE_SIZE_STEP !== 0 || height % IMAGE_SIZE_STEP !== 0) throw new Error("图像尺寸的宽高必须是 16 的倍数，请调整尺寸");
    if (Math.max(width, height) > IMAGE_MAX_LONG_EDGE) throw new Error("图像尺寸最长边不能超过 3840px，请调整尺寸");
    if (Math.max(width, height) / Math.min(width, height) > IMAGE_MAX_RATIO) throw new Error("图像宽高比不能超过 3:1，请调整尺寸");
    const pixels = width * height;
    if (pixels < IMAGE_MIN_PIXELS || pixels > IMAGE_MAX_PIXELS) throw new Error("图像总像素需在 655360 到 8294400 之间，请调整尺寸");
}

export function resolveConstrainedImageSize(width: number, height: number, options: { supports4K: boolean; mark4K?: boolean }) {
    const sourceWidth = Math.max(1, Math.round(width));
    const sourceHeight = Math.max(1, Math.round(height));
    const longSide = Math.max(sourceWidth, sourceHeight);
    const shortSide = Math.min(sourceWidth, sourceHeight);
    const ratio = longSide / shortSide;
    if (!Number.isFinite(ratio) || ratio > IMAGE_MAX_RATIO) return "auto";

    const targetPixels = options.supports4K ? IMAGE_MAX_PIXELS : 2048 * 2048;
    let scale = options.supports4K ? IMAGE_MAX_LONG_EDGE / longSide : Math.sqrt(targetPixels / (sourceWidth * sourceHeight));
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
    return options.supports4K && options.mark4K ? mark4KImageSize(size) : size;
}

function floorToStep(value: number) {
    return Math.max(IMAGE_SIZE_STEP, Math.floor(value / IMAGE_SIZE_STEP) * IMAGE_SIZE_STEP);
}
