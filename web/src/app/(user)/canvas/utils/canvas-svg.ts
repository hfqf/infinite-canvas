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

function extractSvg(text: string) {
    const match = text.match(/<svg[\s\S]*?<\/svg>/i);
    if (!match) throw new Error("模型没有返回有效 SVG 源码");
    return match[0];
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
