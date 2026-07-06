import test from "node:test";
import assert from "node:assert/strict";

import { runCanvasImageDownloadStrategy } from "../src/app/(user)/canvas/utils/canvas-mobile-image-download.ts";

test("mobile image download saves locally before reusing existing OSS url", async () => {
    const calls: string[] = [];
    const blob = new Blob(["image"], { type: "image/png" });

    const result = await runCanvasImageDownloadStrategy({
        blob,
        fileName: "canvas-image.png",
        currentUrl: "https://oss.example.com/canvas/a.png",
        isMobile: true,
        saveBlob: () => calls.push("save"),
        ensureRemoteImage: async () => {
            calls.push("upload");
            return { url: "https://oss.example.com/canvas/uploaded.png" };
        },
    });

    assert.deepEqual(calls, ["save"]);
    assert.equal(result.shareUrl, "https://oss.example.com/canvas/a.png");
    assert.equal(result.uploaded, undefined);
});

test("mobile image download uploads only when no shareable url exists", async () => {
    const calls: string[] = [];
    const blob = new Blob(["image"], { type: "image/png" });

    const result = await runCanvasImageDownloadStrategy({
        blob,
        fileName: "canvas-image.png",
        currentUrl: "data:image/png;base64,abc",
        isMobile: true,
        saveBlob: () => calls.push("save"),
        ensureRemoteImage: async () => {
            calls.push("upload");
            return { url: "https://oss.example.com/canvas/uploaded.png", storageKey: "oss:canvas/uploaded.png" };
        },
    });

    assert.deepEqual(calls, ["save", "upload"]);
    assert.equal(result.shareUrl, "https://oss.example.com/canvas/uploaded.png");
    assert.equal(result.uploaded?.storageKey, "oss:canvas/uploaded.png");
});

test("desktop image download only saves local file", async () => {
    const calls: string[] = [];
    const blob = new Blob(["image"], { type: "image/png" });

    const result = await runCanvasImageDownloadStrategy({
        blob,
        fileName: "canvas-image.png",
        currentUrl: "https://oss.example.com/canvas/a.png",
        isMobile: false,
        saveBlob: () => calls.push("save"),
        ensureRemoteImage: async () => {
            calls.push("upload");
            return { url: "https://oss.example.com/canvas/uploaded.png" };
        },
    });

    assert.deepEqual(calls, ["save"]);
    assert.equal(result.shareUrl, "");
    assert.equal(result.uploaded, undefined);
});

test("mobile image download keeps local save successful when remote fallback fails", async () => {
    const calls: string[] = [];
    const blob = new Blob(["image"], { type: "image/png" });

    const result = await runCanvasImageDownloadStrategy({
        blob,
        fileName: "canvas-image.png",
        currentUrl: "blob:http://localhost/image",
        isMobile: true,
        saveBlob: () => calls.push("save"),
        ensureRemoteImage: async () => {
            calls.push("upload");
            throw new Error("OSS 上传失败");
        },
    });

    assert.deepEqual(calls, ["save", "upload"]);
    assert.equal(result.shareUrl, "");
    assert.equal(result.uploadError instanceof Error, true);
});
