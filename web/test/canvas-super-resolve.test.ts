import test from "node:test";
import assert from "node:assert/strict";

import { buildCanvasSuperResolvePrompt, resolveCanvasSuperResolveSize } from "../src/app/(user)/canvas/utils/canvas-super-resolve.ts";

test("builds an ai super resolution prompt that preserves source content", () => {
    const prompt = buildCanvasSuperResolvePrompt();

    assert.match(prompt, /超分/);
    assert.match(prompt, /保持原始构图/);
    assert.match(prompt, /不要改变画面内容/);
});

test("resolves 4k super resolution size from source ratio", () => {
    assert.equal(resolveCanvasSuperResolveSize(1600, 900, true), "3840x2160");
});

test("falls back to 2k size when 4k is not supported", () => {
    assert.equal(resolveCanvasSuperResolveSize(1600, 900, false), "2720x1536");
});
