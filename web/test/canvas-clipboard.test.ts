import test from "node:test";
import assert from "node:assert/strict";

import { getClipboardImageFiles } from "../src/app/(user)/canvas/utils/canvas-clipboard.ts";

test("returns pasted image files from clipboard files", () => {
    const image = new File(["image"], "demo.png", { type: "image/png" });
    const text = new File(["name"], "demo.txt", { type: "text/plain" });

    assert.deepEqual(getClipboardImageFiles({ files: [text, image], items: [] }), [image]);
});

test("returns pasted image files from clipboard items", () => {
    const image = new File(["image"], "demo.png", { type: "image/png" });

    assert.deepEqual(
        getClipboardImageFiles({
            files: [],
            items: [
                {
                    kind: "file",
                    type: "image/png",
                    getAsFile: () => image,
                },
            ],
        }),
        [image],
    );
});

test("ignores plain text clipboard content", () => {
    const text = new File(["demo.png"], "demo.png", { type: "text/plain" });

    assert.deepEqual(getClipboardImageFiles({ files: [text], items: [] }), []);
});

test("returns image files with empty mime type by extension", () => {
    const image = new File(["image"], "demo.webp");

    assert.deepEqual(getClipboardImageFiles({ files: [image], items: [] }), [image]);
});
