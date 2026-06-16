# 图片节点预设编辑工具 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use $superpower-subagents (recommended) or $superpower-executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking via update_plan.

**Goal:** 在图片节点悬浮工具栏新增“转矢量 / 平面拆解 / 模糊变高清”三个一键预设编辑工具，并全部复用现有 OpenAI 兼容图片编辑接口。

**Architecture:** 前端将三个新按钮建模为图片工具定义，提示词和文案统一放在 `canvas/constants.ts`，点击后统一派发到画布页面的 `presetEditImageNode`。执行函数复用现有 `requestEdit`、本地媒体上传、节点创建、节点连线和错误状态更新流程；后端代理接口不变。

**Tech Stack:** Next.js App Router、React、TypeScript、Ant Design、lucide-react、现有 OpenAI 兼容 `/images/edits` 调用、Go Gin 后端代理。

---

### Task 1: 增加图片预设编辑统一配置

**Files:**
- Modify: `web/src/app/(user)/canvas/constants.ts`

- [ ] **Step 1: 添加预设类型和配置**

在节点规格后增加：

```ts
export type CanvasImagePresetEditId = "vectorize" | "decompose" | "clarify";

export const IMAGE_PRESET_EDIT_CONFIG = {
    vectorize: {
        title: "Vectorized Image",
        label: "转矢量",
        panelLabel: "转矢量",
        tooltip: "生成矢量插画风格图片",
        error: "转矢量失败",
        prompt: "将参考图转换为干净的矢量插画风格，保留主体轮廓和关键细节，使用清晰边缘、纯色块、少量渐变，整体像可用于图标、贴纸或平面设计的矢量稿，背景保持简洁。",
    },
    decompose: {
        title: "Decomposed Image",
        label: "平面拆解",
        panelLabel: "平面拆解",
        tooltip: "生成平面元素拆解图",
        error: "平面拆解失败",
        prompt: "将参考图拆解为平面设计元素展示，分离主体、背景、装饰元素、文字区域和关键形状，用整洁排版展示这些元素，保持原图的主要视觉信息和色彩关系，生成一张清晰的元素拆解图。",
    },
    clarify: {
        title: "Clear Image",
        label: "变高清",
        panelLabel: "模糊变高清",
        tooltip: "修复模糊并提升清晰度",
        error: "变高清失败",
        prompt: "修复参考图的模糊、噪点和压缩痕迹，提升清晰度、边缘质量和细节表现，保持原始构图、主体身份、颜色和风格不变，不要改变画面内容。",
    },
} satisfies Record<CanvasImagePresetEditId, { title: string; label: string; panelLabel: string; tooltip: string; error: string; prompt: string }>;
```

### Task 2: 扩展图片悬浮工具定义

**Files:**
- Modify: `web/src/app/(user)/canvas/components/canvas-image-toolbar-tools.tsx`

- [ ] **Step 1: 扩展类型和 handler**

将工具 ID 和 handler 类型改成下面结构：

```ts
import { Brush, Camera, Copy, FileText, Focus, Grid2x2, Layers3, Lock, LockOpen, Maximize2, Scissors, Sparkles, Upload, WandSparkles, ZoomIn } from "lucide-react";

import { IMAGE_PRESET_EDIT_CONFIG, type CanvasImagePresetEditId } from "../constants";
export type ImageNodeActionToolId =
    | "copyPrompt"
    | "reversePrompt"
    | "replace"
    | "resize"
    | "maskEdit"
    | "crop"
    | "split"
    | "upscale"
    | "superResolve"
    | "angle"
    | "view"
    | CanvasImagePresetEditId;

export type ImageToolHandlers = {
    onUpload: (node: CanvasNodeData) => void;
    onToggleFreeResize: (node: CanvasNodeData) => void;
    onMaskEdit: (node: CanvasNodeData) => void;
    onCrop: (node: CanvasNodeData) => void;
    onSplit: (node: CanvasNodeData) => void;
    onUpscale: (node: CanvasNodeData) => void;
    onSuperResolve: (node: CanvasNodeData) => void;
    onAngle: (node: CanvasNodeData) => void;
    onViewImage: (node: CanvasNodeData) => void;
    onCopyPrompt: (node: CanvasNodeData) => void;
    onReversePrompt: (node: CanvasNodeData) => void;
    onPresetEdit: (node: CanvasNodeData, preset: CanvasImagePresetEditId) => void;
};
```

- [ ] **Step 2: 升级本地配置 key**

将：

```ts
export const IMAGE_QUICK_TOOLS_STORAGE_KEY = "canvas-image-quick-tools-v6";
```

改为：

```ts
export const IMAGE_QUICK_TOOLS_STORAGE_KEY = "canvas-image-quick-tools-v7";
```

这样已有浏览器本地配置不会隐藏新工具。

- [ ] **Step 3: 添加三个工具定义**

在 `imageToolDefinitions` 中放到 `superResolve` 后、`angle` 前：

```tsx
{
    id: "vectorize",
    defaultVisible: true,
    panelLabel: IMAGE_PRESET_EDIT_CONFIG.vectorize.panelLabel,
    label: IMAGE_PRESET_EDIT_CONFIG.vectorize.label,
    title: IMAGE_PRESET_EDIT_CONFIG.vectorize.tooltip,
    icon: () => <WandSparkles className="size-4" />,
    run: (node, handlers) => handlers.onPresetEdit(node, "vectorize"),
},
{
    id: "decompose",
    defaultVisible: true,
    panelLabel: IMAGE_PRESET_EDIT_CONFIG.decompose.panelLabel,
    label: IMAGE_PRESET_EDIT_CONFIG.decompose.label,
    title: IMAGE_PRESET_EDIT_CONFIG.decompose.tooltip,
    icon: () => <Layers3 className="size-4" />,
    run: (node, handlers) => handlers.onPresetEdit(node, "decompose"),
},
{
    id: "clarify",
    defaultVisible: true,
    panelLabel: IMAGE_PRESET_EDIT_CONFIG.clarify.panelLabel,
    label: IMAGE_PRESET_EDIT_CONFIG.clarify.label,
    title: IMAGE_PRESET_EDIT_CONFIG.clarify.tooltip,
    icon: () => <Focus className="size-4" />,
    run: (node, handlers) => handlers.onPresetEdit(node, "clarify"),
},
```

- [ ] **Step 4: 检查类型引用**

运行：

```powershell
rg "CanvasImagePresetEditId|vectorize|decompose|clarify|canvas-image-quick-tools-v7" web\src\app\(user)\canvas\components\canvas-image-toolbar-tools.tsx
```

Expected: 能看到新类型、三个工具 ID 和 `v7` storage key。

- [ ] **Step 5: Commit**

```bash
git add web/src/app/(user)/canvas/components/canvas-image-toolbar-tools.tsx
git commit -m "feat: add image preset tool definitions"
```

### Task 3: 让悬浮工具栏派发预设编辑事件

**Files:**
- Modify: `web/src/app/(user)/canvas/components/canvas-node-hover-toolbar.tsx`

- [ ] **Step 1: 导入新类型**

从统一配置中导入 `CanvasImagePresetEditId`：

```ts
import type { CanvasImagePresetEditId } from "../constants";
```

- [ ] **Step 2: 扩展 props**

在 `CanvasNodeHoverToolbarProps` 中加入：

```ts
onPresetEdit: (node: CanvasNodeData, preset: CanvasImagePresetEditId) => void;
```

在函数参数解构中加入：

```ts
onPresetEdit,
```

- [ ] **Step 3: 传给图片工具构建函数**

将 `buildImageToolbarTools` 调用改为：

```ts
const imageTools = buildImageToolbarTools(node, {
    onUpload,
    onToggleFreeResize,
    onMaskEdit,
    onCrop,
    onSplit,
    onUpscale,
    onSuperResolve,
    onAngle,
    onViewImage,
    onCopyPrompt: copyImagePrompt,
    onReversePrompt,
    onPresetEdit,
});
```

- [ ] **Step 4: 检查没有漏传**

运行：

```powershell
rg "onPresetEdit|CanvasImagePresetEditId|buildImageToolbarTools" "web\src\app\(user)\canvas\components\canvas-node-hover-toolbar.tsx"
```

Expected: props、解构和 `buildImageToolbarTools` 调用都包含 `onPresetEdit`。

- [ ] **Step 5: Commit**

```bash
git add web/src/app/(user)/canvas/components/canvas-node-hover-toolbar.tsx
git commit -m "feat: wire image preset edit toolbar actions"
```

### Task 4: 实现画布页面的一键预设编辑

**Files:**
- Modify: `web/src/app/(user)/canvas/[id]/canvas-client-page.tsx`

- [ ] **Step 1: 导入预设类型**

在现有图片工具相关 import 附近增加：

```ts
import { IMAGE_PRESET_EDIT_CONFIG, NODE_DEFAULT_SIZE, getNodeSpec, type CanvasImagePresetEditId } from "../constants";
```

- [ ] **Step 2: 添加执行函数**

在 `generateAngleNode` 附近新增函数，复用同一批依赖：

```ts
const presetEditImageNode = useCallback(
    async (node: CanvasNodeData, preset: CanvasImagePresetEditId) => {
        if (node.type !== CanvasNodeType.Image || !node.metadata?.content) {
            message.warning("图片节点为空，无法执行该工具");
            return;
        }
        const presetConfig = IMAGE_PRESET_EDIT_CONFIG[preset];
        const generationConfig = { ...buildGenerationConfig(effectiveConfig, node, "image"), count: "1", size: node.metadata?.size || "auto" };
        if (!isAiConfigReady(generationConfig, generationConfig.model)) {
            openConfigDialog(true);
            return;
        }

        const childId = nanoid();
        const source = {
            id: node.id,
            name: `${node.title || node.id}.png`,
            type: node.metadata.mimeType || "image/png",
            dataUrl: node.metadata.content,
            storageKey: node.metadata.storageKey,
        };
        const generationMetadata = buildImageGenerationMetadata("edit", generationConfig, 1, [source]);

        setRunningNodeId(childId);
        setNodes((prev) => [
            ...prev,
            {
                id: childId,
                type: CanvasNodeType.Image,
                title: presetConfig.title,
                position: { x: node.position.x + node.width + 96, y: node.position.y },
                width: node.width,
                height: node.height,
                metadata: { prompt: presetConfig.prompt, status: NODE_STATUS_LOADING, ...generationMetadata },
            },
        ]);
        setConnections((prev) => [...prev, { id: nanoid(), fromNodeId: node.id, toNodeId: childId }]);
        setSelectedNodeIds(new Set([childId]));
        setSelectedConnectionId(null);
        setDialogNodeId(childId);
        setContextMenu(null);

        try {
            const image = await requestEdit(generationConfig, presetConfig.prompt, [source]).then((items) => items[0]);
            const uploaded = await uploadImage(image.dataUrl);
            const size = fitNodeSize(uploaded.width, uploaded.height, node.width, node.height);
            setNodes((prev) =>
                prev.map((item) =>
                    item.id === childId
                        ? {
                              ...item,
                              width: size.width,
                              height: size.height,
                              metadata: { ...item.metadata, ...imageMetadata(uploaded), prompt: presetConfig.prompt, ...generationMetadata },
                          }
                        : item,
                ),
            );
        } catch (error) {
            const errorDetails = error instanceof Error ? error.message : presetConfig.error;
            message.error(errorDetails);
            setNodes((prev) => prev.map((item) => (item.id === childId ? { ...item, metadata: { ...item.metadata, status: NODE_STATUS_ERROR, errorDetails } } : item)));
        } finally {
            setRunningNodeId(null);
        }
    },
    [effectiveConfig, isAiConfigReady, message, openConfigDialog],
);
```

- [ ] **Step 4: 传入悬浮工具栏**

在 `<CanvasNodeHoverToolbar />` props 中增加：

```tsx
onPresetEdit={(node, preset) => void presetEditImageNode(node, preset)}
```

- [ ] **Step 5: 检查调用链**

运行：

```powershell
rg "IMAGE_PRESET_EDIT_CONFIG|presetEditImageNode|onPresetEdit|CanvasImagePresetEditId" "web\src\app\(user)\canvas\[id]\canvas-client-page.tsx"
```

Expected: 能看到常量、函数、类型导入和传参。

- [ ] **Step 6: Commit**

```bash
git add web/src/app/(user)/canvas/[id]/canvas-client-page.tsx
git commit -m "feat: implement image preset edit actions"
```

### Task 5: 更新待测试文档

**Files:**
- Modify: `docs/content/docs/progress/pending-test.mdx`

- [ ] **Step 1: 添加待测试记录**

在图片节点悬浮工具相关记录附近添加一条：

```md
- 图片节点悬浮工具栏新增“转矢量”、“平面拆解”和“模糊变高清”入口，三个工具都会复用现有图片编辑接口，把原图作为参考图生成新的右侧图片节点并自动连线；需要验证远程渠道和本地直连渠道的成功生成、失败错误状态、原图不变、结果节点尺寸回填，以及“更多”自定义工具栏中可显示/隐藏这些入口。
```

- [ ] **Step 2: Commit**

```bash
git add docs/content/docs/progress/pending-test.mdx
git commit -m "docs: 记录图片预设编辑工具待测试项"
```

### Task 6: 最终核对

**Files:**
- Inspect only.

- [ ] **Step 1: 查看工作区状态**

Run:

```powershell
git status --short
```

Expected: 没有未提交变更，除非用户工作区已有无关改动。

- [ ] **Step 2: 静态检查关键引用**

Run:

```powershell
rg "vectorize|decompose|clarify|onPresetEdit|presetEditImageNode|canvas-image-quick-tools-v7" web\src\app\(user)\canvas docs\content\docs\progress\pending-test.mdx
```

Expected: 新工具定义、事件派发、执行函数和文档记录都能被搜索到。

- [ ] **Step 3: 不执行构建**

按项目 `AGENTS.md` 要求，本任务完成后不运行构建或测试。只报告已做静态核对。

## Verification

本计划遵循规格文档：

- 三个工具全部复用现有 OpenAI 兼容图片编辑接口。
- 不新增后端路由。
- 不输出 SVG 文件。
- 点击后生成新图片节点，原节点不变。
- 新节点自动连线，失败时保留错误状态。
- 更新中文待测试文档。

## Next skill

`$superpower-subagents` 或 `$superpower-executing-plans`
