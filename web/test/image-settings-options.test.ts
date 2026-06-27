import test from "node:test";
import assert from "node:assert/strict";

import { visibleImageResolutionOptions } from "../src/components/image-settings-options.ts";

test("hides 4k resolution option when model does not support 4k", () => {
    assert.deepEqual(
        visibleImageResolutionOptions(false).map((item) => item.value),
        ["1k", "2k"],
    );
});

test("shows 4k resolution option when model supports 4k", () => {
    assert.deepEqual(
        visibleImageResolutionOptions(true).map((item) => item.value),
        ["1k", "2k", "4k"],
    );
});
