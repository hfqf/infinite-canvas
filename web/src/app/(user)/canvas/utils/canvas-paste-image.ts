type PasteTargetNode = {
    id: string;
    type: string;
};

export function findPasteImageTargetNodeId(nodes: PasteTargetNode[], selectedNodeIds: Set<string>) {
    if (selectedNodeIds.size !== 1) return null;
    const nodeId = Array.from(selectedNodeIds)[0];
    return nodes.find((node) => node.id === nodeId)?.type === "image" ? nodeId : null;
}
