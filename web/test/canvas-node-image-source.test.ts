import test from "node:test";
import assert from "node:assert/strict";

import { canvasNodeImageToDataUrlInput } from "../src/app/(user)/canvas/utils/canvas-node-image-source.ts";

test("keeps data urls exportable without fetching", () => {
    assert.deepEqual(
        canvasNodeImageToDataUrlInput({ metadata: { content: "data:image/png;base64,abc", storageKey: "image:local" } }),
        { dataUrl: "data:image/png;base64,abc" },
    );
});

test("passes remote image urls through imageToDataUrl input", () => {
    assert.deepEqual(
        canvasNodeImageToDataUrlInput({ metadata: { content: "https://cdn.example.com/a.png", storageKey: "oss:canvas/images/a.png" } }),
        { url: "https://cdn.example.com/a.png", storageKey: "oss:canvas/images/a.png" },
    );
});
