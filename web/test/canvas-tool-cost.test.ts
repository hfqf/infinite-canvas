import test from "node:test";
import assert from "node:assert/strict";

import { canvasToolCostCredits, canvasToolTooltipTitle } from "../src/constant/canvas-tool-cost.ts";

test("reads configured canvas tool cost credits", () => {
    assert.equal(canvasToolCostCredits([{ tool: "superResolve", credits: 3 }], "superResolve"), 3);
    assert.equal(canvasToolCostCredits([{ tool: "superResolve", credits: 3 }], "crop"), 0);
});

test("adds credits to canvas tool tooltip only when configured", () => {
    assert.equal(canvasToolTooltipTitle("AI 超分", 3), "AI 超分 · 消耗 3 积分");
    assert.equal(canvasToolTooltipTitle("裁剪", 0), "裁剪 · 消耗 0 积分");
});

test("shows concrete model credits and total when tool also calls model", () => {
    assert.equal(canvasToolTooltipTitle("模糊变高清", 2, 5), "模糊变高清 · 工具服务费 2 积分 · 模型消耗 5 积分 · 合计 7 积分");
});
