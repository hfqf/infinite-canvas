import test from "node:test";
import assert from "node:assert/strict";

import { isBusiness4KImageRequest, mark4KImageSize, parseImageDimensions, stripImageSizeMarker, validateImageGenerationSize } from "../src/constant/image-generation-constraints.ts";

test("marks and strips business 4k image sizes", () => {
    assert.equal(mark4KImageSize("3840x2160"), "4k:3840x2160");
    assert.equal(stripImageSizeMarker("4k:3840x2160"), "3840x2160");
    assert.deepEqual(parseImageDimensions("4k:2160x3840"), { width: 2160, height: 3840 });
});

test("treats only business marker or quality 4k as 4k billing", () => {
    assert.equal(isBusiness4KImageRequest("3840x2160", "medium"), false);
    assert.equal(isBusiness4KImageRequest("4k:3840x2160", "medium"), true);
    assert.equal(isBusiness4KImageRequest("1024x1024", "4k"), true);
});

test("validates global image generation constraints", () => {
    assert.doesNotThrow(() => validateImageGenerationSize(3840, 2160));
    assert.throws(() => validateImageGenerationSize(3840, 3840), /8294400/);
    assert.throws(() => validateImageGenerationSize(1000, 1000), /16/);
    assert.throws(() => validateImageGenerationSize(4096, 1024), /3840/);
    assert.throws(() => validateImageGenerationSize(3088, 1024), /3:1/);
});
