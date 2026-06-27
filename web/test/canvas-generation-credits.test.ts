import test from "node:test";
import assert from "node:assert/strict";

import { canvasGenerationCredits } from "../src/constant/credit-cost.ts";

const modelCosts = [
    { model: "gpt-image-2", credits: 5, supports4K: true },
    { model: "gpt-4o-mini", credits: 2 },
];

test("calculates image generation credits from model, count and references", () => {
    assert.equal(
        canvasGenerationCredits({
            channelMode: "remote",
            model: "gpt-image-2",
            count: "2",
            size: "1024x1024",
            quality: "standard",
            mode: "image",
            modelCosts,
            imageReferenceCount: 3,
        }),
        14,
    );
});

test("calculates concrete text generation credits for config composer", () => {
    assert.equal(
        canvasGenerationCredits({
            channelMode: "remote",
            model: "gpt-4o-mini",
            count: "8",
            mode: "text",
            modelCosts,
            imageReferenceCount: 1,
        }),
        2,
    );
});
