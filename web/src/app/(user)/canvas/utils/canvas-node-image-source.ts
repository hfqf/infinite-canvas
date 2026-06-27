import type { CanvasNodeData } from "../types";

type ImageToDataUrlInput = {
    url?: string;
    dataUrl?: string;
    storageKey?: string;
};

export function canvasNodeImageToDataUrlInput(node: Pick<CanvasNodeData, "metadata">): ImageToDataUrlInput {
    const content = node.metadata?.content || "";
    if (content.startsWith("data:")) return { dataUrl: content };
    return { url: content, storageKey: node.metadata?.storageKey };
}
