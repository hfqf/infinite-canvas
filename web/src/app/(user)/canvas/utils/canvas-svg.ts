"use client";

export type CanvasSvgMeta = {
    content: string;
    dataUrl: string;
    width: number;
    height: number;
    bytes: number;
    mimeType: string;
};

const SVG_MIME_TYPE = "image/svg+xml";
const blockedTags = new Set(["script", "foreignObject", "iframe", "object", "embed", "audio", "video", "image"]);

export function buildSvgMeta(text: string): CanvasSvgMeta {
    const content = sanitizeSvg(extractSvg(text));
    const size = readSvgSize(content);
    return {
        content,
        dataUrl: svgToDataUrl(content),
        width: size.width,
        height: size.height,
        bytes: new Blob([content], { type: SVG_MIME_TYPE }).size,
        mimeType: SVG_MIME_TYPE,
    };
}

export function svgToDataUrl(svg: string) {
    return `data:${SVG_MIME_TYPE};charset=utf-8,${encodeURIComponent(svg)}`;
}

export function svgBlob(svg: string) {
    return new Blob([svg], { type: SVG_MIME_TYPE });
}

export async function traceImageToSvg(imageUrl: string): Promise<CanvasSvgMeta> {
    const image = await loadImage(imageUrl);
    const maxEdge = 640;
    const scale = Math.min(1, maxEdge / Math.max(image.naturalWidth || image.width, image.naturalHeight || image.height));
    const width = Math.max(1, Math.round((image.naturalWidth || image.width) * scale));
    const height = Math.max(1, Math.round((image.naturalHeight || image.height) * scale));
    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    const context = canvas.getContext("2d", { willReadFrequently: true });
    if (!context) throw new Error("浏览器不支持图片转 SVG");
    context.imageSmoothingEnabled = true;
    context.imageSmoothingQuality = "high";
    context.drawImage(image, 0, 0, width, height);

    const pixels = context.getImageData(0, 0, width, height).data;
    const rects = mergeColorRects(pixels, width, height);
    const body = rects
        .map((rect) => `<rect x="${rect.x}" y="${rect.y}" width="${rect.width}" height="${rect.height}" fill="${rect.color}"/>`)
        .join("");
    const content = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${width} ${height}" width="${width}" height="${height}" shape-rendering="crispEdges"><rect width="${width}" height="${height}" fill="#fff"/>${body}</svg>`;
    return {
        content,
        dataUrl: svgToDataUrl(content),
        width,
        height,
        bytes: new Blob([content], { type: SVG_MIME_TYPE }).size,
        mimeType: SVG_MIME_TYPE,
    };
}

function extractSvg(text: string) {
    const match = text.match(/<svg[\s\S]*?<\/svg>/i);
    if (!match) throw new Error("模型没有返回有效 SVG 源码");
    return match[0];
}

function mergeColorRects(pixels: Uint8ClampedArray, width: number, height: number) {
    const active = new Map<string, { x: number; y: number; width: number; height: number; color: string }>();
    const rects: { x: number; y: number; width: number; height: number; color: string }[] = [];
    for (let y = 0; y < height; y += 1) {
        const nextActive = new Map<string, { x: number; y: number; width: number; height: number; color: string }>();
        let x = 0;
        while (x < width) {
            const color = quantizedPixelColor(pixels, (y * width + x) * 4);
            let runWidth = 1;
            while (x + runWidth < width && quantizedPixelColor(pixels, (y * width + x + runWidth) * 4) === color) runWidth += 1;
            if (color !== "#fff") {
                const key = `${x}:${runWidth}:${color}`;
                const previous = active.get(key);
                if (previous) {
                    previous.height += 1;
                    nextActive.set(key, previous);
                } else {
                    nextActive.set(key, { x, y, width: runWidth, height: 1, color });
                }
            }
            x += runWidth;
        }
        active.forEach((rect, key) => {
            if (!nextActive.has(key)) rects.push(rect);
        });
        active.clear();
        nextActive.forEach((rect, key) => active.set(key, rect));
    }
    active.forEach((rect) => rects.push(rect));
    return rects;
}

function quantizedPixelColor(pixels: Uint8ClampedArray, index: number) {
    const alpha = pixels[index + 3];
    if (alpha < 16) return "#fff";
    const r = quantizeChannel(blendOnWhite(pixels[index], alpha));
    const g = quantizeChannel(blendOnWhite(pixels[index + 1], alpha));
    const b = quantizeChannel(blendOnWhite(pixels[index + 2], alpha));
    if (r >= 248 && g >= 248 && b >= 248) return "#fff";
    return `#${hex(r)}${hex(g)}${hex(b)}`;
}

function blendOnWhite(value: number, alpha: number) {
    const a = alpha / 255;
    return Math.round(value * a + 255 * (1 - a));
}

function quantizeChannel(value: number) {
    return Math.max(0, Math.min(255, Math.round(value / 24) * 24));
}

function hex(value: number) {
    return value.toString(16).padStart(2, "0");
}

function loadImage(url: string) {
    return new Promise<HTMLImageElement>((resolve, reject) => {
        const image = new Image();
        image.onload = () => resolve(image);
        image.onerror = () => reject(new Error("读取图片失败，无法转 SVG"));
        image.src = url;
    });
}

function sanitizeSvg(svg: string) {
    const parser = new DOMParser();
    const doc = parser.parseFromString(svg, SVG_MIME_TYPE);
    if (doc.querySelector("parsererror") || doc.documentElement.tagName.toLowerCase() !== "svg") throw new Error("SVG 源码解析失败");

    doc.querySelectorAll(Array.from(blockedTags).join(",")).forEach((element) => element.remove());
    doc.querySelectorAll("*").forEach((element) => {
        Array.from(element.attributes).forEach((attribute) => {
            const name = attribute.name.toLowerCase();
            const value = attribute.value.trim().toLowerCase();
            const href = name === "href" || name === "xlink:href" || name.endsWith(":href");
            if (name.startsWith("on") || value.startsWith("javascript:") || (href && value && !value.startsWith("#")) || hasUnsafeUrl(attribute.value)) element.removeAttribute(attribute.name);
        });
    });

    const root = doc.documentElement;
    if (!root.getAttribute("xmlns")) root.setAttribute("xmlns", "http://www.w3.org/2000/svg");
    return new XMLSerializer().serializeToString(root);
}

function hasUnsafeUrl(value: string) {
    if (/@import/i.test(value)) return true;
    return Array.from(value.matchAll(/url\s*\(\s*(['"]?)(.*?)\1\s*\)/gi)).some((match) => !match[2].trim().startsWith("#"));
}

function readSvgSize(svg: string) {
    const doc = new DOMParser().parseFromString(svg, SVG_MIME_TYPE);
    const root = doc.documentElement;
    const width = parseSvgLength(root.getAttribute("width"));
    const height = parseSvgLength(root.getAttribute("height"));
    if (width && height) return { width, height };

    const viewBox = root.getAttribute("viewBox")?.trim().split(/\s+/).map(Number);
    if (viewBox?.length === 4 && viewBox.every(Number.isFinite) && viewBox[2] > 0 && viewBox[3] > 0) return { width: Math.round(viewBox[2]), height: Math.round(viewBox[3]) };

    return { width: 1024, height: 768 };
}

function parseSvgLength(value: string | null) {
    if (!value) return 0;
    const match = value.match(/^([\d.]+)/);
    const number = match ? Number(match[1]) : 0;
    return Number.isFinite(number) && number > 0 ? Math.round(number) : 0;
}
