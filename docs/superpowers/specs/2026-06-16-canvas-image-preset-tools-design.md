# 图片节点预设编辑工具设计

## 背景

画布图片节点已有悬浮工具栏，支持复制提示词、反推提示词、替换图片、局部编辑、裁剪、切图、放大、超分、多角度和查看大图。用户希望继续增加三个图片处理入口：

- 转矢量
- 平面拆解
- 模糊图片变高清

本次范围明确要求三个工具都走现有 OpenAI 兼容图像编辑接口，不新增专门矢量化库、图像分割模型、超分服务或后端业务接口。

## 目标

在图片节点已有内容时，悬浮工具栏新增三个一键预设编辑工具。用户点击工具后，系统基于当前图片调用现有图片编辑接口，并在原图右侧生成新的图片节点，原图保持不变。

## 非目标

- 不输出真正的 SVG 文件；“转矢量”生成的是矢量插画风格位图结果。
- 不新增新的后端 AI 路由；继续复用 `/api/v1/images/edits` 和本地直连的 `/images/edits`。
- 不引入新的图片处理库或第三方专用服务。
- 不新增复杂参数弹窗；第一版按一键预设执行。

## 用户体验

三个工具只在有内容的图片节点上展示，并参与现有“更多/自定义工具栏”配置。

- `转矢量`：生成一张轮廓清晰、色块干净、接近矢量插画效果的新图。
- `平面拆解`：生成一张对原图主体、背景、装饰和文字区域进行平面元素拆解的分析图。
- `模糊变高清`：生成一张修复模糊、噪点和压缩痕迹的新图。

点击工具后的行为：

1. 在原图右侧创建 loading 状态图片节点。
2. 新节点与原图自动连线。
3. 调用现有 `requestEdit`，把原图作为参考图。
4. 成功后上传结果图到本地图片存储，更新新节点内容、尺寸和生成元数据。
5. 失败时保留新节点，并在节点状态中记录错误信息，方便重试或排查。

## 默认提示词

### 转矢量

将参考图转换为干净的矢量插画风格，保留主体轮廓和关键细节，使用清晰边缘、纯色块、少量渐变，整体像可用于图标、贴纸或平面设计的矢量稿，背景保持简洁。

### 平面拆解

将参考图拆解为平面设计元素展示，分离主体、背景、装饰元素、文字区域和关键形状，用整洁排版展示这些元素，保持原图的主要视觉信息和色彩关系，生成一张清晰的元素拆解图。

### 模糊变高清

修复参考图的模糊、噪点和压缩痕迹，提升清晰度、边缘质量和细节表现，保持原始构图、主体身份、颜色和风格不变，不要改变画面内容。

## 前端设计

### 工具定义

在 `web/src/app/(user)/canvas/components/canvas-image-toolbar-tools.tsx` 中扩展图片工具类型：

- `vectorize`
- `decompose`
- `clarify`

每个工具定义包含：

- `defaultVisible: true`
- `panelLabel`
- `label`
- `title`
- `icon`
- `run`

本地快捷工具配置 key 从 `canvas-image-quick-tools-v6` 升级到 `canvas-image-quick-tools-v7`，确保已有用户默认能看到新增工具；用户仍可在“更多”中隐藏。

### 悬浮工具栏

在 `CanvasNodeHoverToolbarProps` 增加统一回调：

```ts
onPresetEdit: (node: CanvasNodeData, preset: CanvasImagePresetEditId) => void;
```

`buildImageToolbarTools` 接收该回调，并将三个工具统一转发到 `onPresetEdit`。这样悬浮工具组件只负责展示和派发事件，不包含具体生成逻辑。

### 画布页面执行逻辑

在 `web/src/app/(user)/canvas/constants.ts` 增加统一预设配置：

- `CanvasImagePresetEditId`
- `IMAGE_PRESET_EDIT_CONFIG`

配置集中保存三个工具的节点标题、按钮文案、面板文案、提示词、错误提示和 tooltip，避免提示词散落在组件里。

在 `canvas-client-page.tsx` 增加执行函数：

- `presetEditImageNode(node, preset)`

执行函数复用现有能力：

- `buildGenerationConfig(effectiveConfig, node, "image")`
- `isAiConfigReady`
- `requestEdit`
- `uploadImage`
- `imageMetadata`
- `buildImageGenerationMetadata`
- `fitNodeSize`

生成节点标题按工具区分：

- `Vectorized Image`
- `Decomposed Image`
- `Clear Image`

生成节点放置规则沿用现有多角度和局部编辑逻辑：位于原图右侧 `96px` 间距处，并连接原图到新节点。

## 后端设计

后端不新增接口。

当用户使用远程渠道时，前端仍请求 `/api/v1/images/edits`，由 `handler/ai.go` 现有代理逻辑选择模型渠道、扣减积分、转发到上游，并在失败时退款。

当用户使用本地直连时，前端仍请求配置的 OpenAI 兼容 `baseUrl + /images/edits`。

## 错误处理

- 图片节点为空时提示“图片节点为空，无法执行该工具”。
- AI 配置缺失时打开现有配置弹窗。
- 请求失败时：
  - 新节点状态标记为 `error`。
  - `errorDetails` 写入错误摘要。
  - 弹出错误提示。
  - `runningNodeId` 清空。

## 测试与验证

需要人工验证：

- 有内容图片节点悬浮工具栏出现三个新入口。
- 通过“更多”可显示/隐藏三个工具，并保存本地配置。
- 三个工具点击后都会创建 loading 子节点并自动连线。
- 成功返回后新节点显示结果图，原节点不变。
- 上游失败时新节点保留并显示错误状态。
- 远程渠道会正确走 `/api/v1/images/edits` 并刷新用户积分。
- 本地直连渠道会正确走自定义 OpenAI 兼容地址。

## 文档更新

实现完成后更新 `docs/content/docs/progress/pending-test.mdx`，记录新增三个图片节点悬浮工具以及需要验证的行为。
