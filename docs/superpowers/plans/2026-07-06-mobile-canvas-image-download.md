# 手机端画布图片下载实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use $superpower-subagents (recommended) or $superpower-executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking via update_plan.

**目标：** 手机浏览器点击画布图片节点现有下载按钮时，优先尝试保存到手机本地，并在保存能力不稳定时使用接口返回的 OSS 链接进行复制或系统分享兜底。

**架构：** 保持下载入口不变，继续由节点悬浮工具栏调用画布页的 `downloadNodeImage`。新增一个小型前端工具模块承载移动端图片下载策略，画布页只负责传入节点图片信息、更新节点 OSS 元数据和展示提示。

**技术栈：** Next.js App Router、React、TypeScript、Ant Design message、`file-saver`、现有 `image-storage` 服务。

---

## 范围

- 只处理图片节点。
- 桌面端图片下载保持原行为。
- SVG、视频、音频下载保持原行为。
- 不新增分享按钮，不新增分享管理页，不做多选导出，不做整张画布导出。
- 手机端点击现有下载按钮后，先触发本机保存，再提供 OSS 链接兜底。

## 任务

### Task 1: 下载策略工具

**文件：**

- 新增：`web/src/app/(user)/canvas/utils/canvas-mobile-image-download.ts`
- 新增测试：`web/test/canvas-mobile-image-download.test.ts`

- [ ] 写失败测试，覆盖移动端优先下载、已有 OSS 链接兜底、无 OSS 链接时先上传取得链接。
- [ ] 实现移动端检测、OSS 链接解析、下载后兜底动作选择。
- [ ] 工具函数不直接依赖 React，不直接读写组件状态。

### Task 2: 画布下载按钮接入

**文件：**

- 修改：`web/src/app/(user)/canvas/[id]/canvas-client-page.tsx`

- [ ] 图片节点下载时调用新工具。
- [ ] 移动端如果上传后取得新的 OSS URL，回写当前节点 `metadata.content`、`storageKey`、`mimeType`、`bytes`。
- [ ] 桌面端、SVG、视频、音频保留现有逻辑。

### Task 3: 文档留痕

**文件：**

- 修改：`docs/content/docs/progress/pending-test.mdx`

- [ ] 记录本次手机端图片下载可测试变更。
- [ ] 确认 `todo.mdx` 无需迁移已有待办。

## 验收

- 手机浏览器点击图片节点下载按钮，会先尝试保存图片到本机。
- 手机浏览器保存能力不稳定时，可以复制或系统分享 OSS 链接。
- 图片节点已有 OSS URL 时不重复上传。
- 图片节点只有本地数据时，会先通过现有上传接口取得 OSS URL。
- 桌面端图片下载行为不变。
- 视频相关功能不发生改造。
