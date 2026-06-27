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
    assert.equal(resolveCanvasSuperResolveSize(1600, 900, true), "4k:3840x2160");
});

test("resolves portrait 4k super resolution size from source ratio", () => {
    assert.equal(resolveCanvasSuperResolveSize(900, 1600, true), "4k:2160x3840");
});

test("keeps square 4k super resolution within max pixels", () => {
    assert.equal(resolveCanvasSuperResolveSize(1024, 1024, true), "4k:2880x2880");
});

test("falls back to 2k size when 4k is not supported", () => {
    assert.equal(resolveCanvasSuperResolveSize(1600, 900, false), "2720x1536");
});

test("falls back to auto when source ratio exceeds global image constraint", () => {
    assert.equal(resolveCanvasSuperResolveSize(4000, 1000, true), "auto");
});
