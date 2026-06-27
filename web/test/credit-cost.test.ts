import test from "node:test";
import assert from "node:assert/strict";

import { modelSupports4K, requestCreditCost } from "../src/constant/credit-cost.ts";

const modelCosts = [
    { model: "gpt-image-2", credits: 5, supports4K: true },
    { model: "image-lite", credits: 2, supports4K: false },
];

test("uses configured non-4k image model credits", () => {
    assert.equal(requestCreditCost({ channelMode: "remote", modelCosts, model: "gpt-image-2", count: 2, size: "1024x1024", referenceCount: 3 }), 14);
});

test("adds three credits for supported 4k image requests", () => {
    assert.equal(requestCreditCost({ channelMode: "remote", modelCosts, model: "gpt-image-2", count: 1, size: "4k:3840x2160", referenceCount: 1 }), 8);
});

test("falls back to non-4k credits when model does not support 4k", () => {
    assert.equal(requestCreditCost({ channelMode: "remote", modelCosts, model: "image-lite", count: 1, quality: "4k" }), 2);
});

test("does not add 4k credits for unmarked api size dimensions", () => {
    assert.equal(requestCreditCost({ channelMode: "remote", modelCosts, model: "gpt-image-2", count: 1, size: "3840x2160" }), 5);
});

test("defaults missing supports4K to true", () => {
    assert.equal(modelSupports4K([{ model: "legacy-image", credits: 4 }], "legacy-image"), true);
});
