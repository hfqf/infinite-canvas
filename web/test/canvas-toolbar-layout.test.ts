import test from "node:test";
import assert from "node:assert/strict";

import { resolveCanvasToolbarDock, resolveCanvasTopBarMode } from "../src/app/(user)/canvas/utils/canvas-toolbar-layout.ts";

test("uses right dock for narrow mobile canvas screens", () => {
    assert.equal(resolveCanvasToolbarDock(430), "right");
    assert.equal(resolveCanvasToolbarDock(640), "right");
});

test("uses bottom dock for wider canvas screens", () => {
    assert.equal(resolveCanvasToolbarDock(641), "bottom");
    assert.equal(resolveCanvasToolbarDock(1280), "bottom");
});

test("uses compact top bar actions for narrow mobile canvas screens", () => {
    assert.equal(resolveCanvasTopBarMode(430), "compact");
    assert.equal(resolveCanvasTopBarMode(640), "compact");
});

test("uses full top bar actions for wider canvas screens", () => {
    assert.equal(resolveCanvasTopBarMode(641), "full");
    assert.equal(resolveCanvasTopBarMode(1280), "full");
});
