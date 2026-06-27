import test from "node:test";
import assert from "node:assert/strict";

import { findPasteImageTargetNodeId } from "../src/app/(user)/canvas/utils/canvas-paste-image.ts";

const imageNode = { id: "image-1", type: "image" };
const textNode = { id: "text-1", type: "text" };

test("targets the selected image node for pasted image replacement", () => {
    assert.equal(findPasteImageTargetNodeId([imageNode, textNode], new Set(["image-1"])), "image-1");
});

test("does not target when selection is empty, non-image, or multiple nodes", () => {
    assert.equal(findPasteImageTargetNodeId([imageNode], new Set()), null);
    assert.equal(findPasteImageTargetNodeId([imageNode, textNode], new Set(["text-1"])), null);
    assert.equal(findPasteImageTargetNodeId([imageNode, textNode], new Set(["image-1", "text-1"])), null);
});
