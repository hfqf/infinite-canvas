"use client";

import { CalendarDays, Copy, Download, Eye, ImageIcon, RefreshCw, Search, Star, X } from "lucide-react";
import { App, Button, DatePicker, Empty, Image, Input, Modal, Pagination, Select, Space, Spin, Tag, Typography } from "antd";
import type { Dayjs } from "dayjs";
import dayjs from "dayjs";
import { saveAs } from "file-saver";
import { useEffect, useMemo, useState } from "react";

import { formatBytes, formatDuration } from "@/lib/image-utils";
import type { AIImageTask, AIImageTaskListResponse, AIImageTaskQuery } from "@/services/api/image-tasks";
import { useUserStore } from "@/stores/use-user-store";

type GeneratedImage = {
    id: string;
    dataUrl: string;
    durationMs: number;
    width: number;
    height: number;
    bytes: number;
    mimeType?: string;
};

type GenerationLog = {
    id: string;
    taskId: string;
    userId: string;
    createdAt: number;
    updatedAt: number;
    title: string;
    prompt: string;
    model: string;
    config: { model?: string; imageModel?: string; quality?: string; size?: string; count?: string | number };
    durationMs: number;
    credits: number;
    referenceCount: number;
    size: string;
    quality: string;
    status: "成功" | "失败";
    featured: boolean;
    image: GeneratedImage | null;
};

type HistoryItem = {
    id: string;
    log: GenerationLog;
    image: GeneratedImage;
    mode: "generate" | "edit";
};

type Props = {
    eyebrow?: string;
    title?: string;
    emptyText?: string;
    defaultUserName?: string;
    loadTasks: (token: string, query: AIImageTaskQuery) => Promise<AIImageTaskListResponse>;
    onToggleFeatured?: (token: string, task: AIImageTask, featured: boolean) => Promise<AIImageTask>;
};

export function ImageTaskHistory({ eyebrow = "IMAGES", title = "图片管理", emptyText = "暂无生成记录", defaultUserName = "当前用户", loadTasks, onToggleFeatured }: Props) {
    const { message } = App.useApp();
    const token = useUserStore((state) => state.token);
    const clearSession = useUserStore((state) => state.clearSession);
    const [tasks, setTasks] = useState<AIImageTask[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(12);
    const [loading, setLoading] = useState(true);
    const [keywordText, setKeywordText] = useState("");
    const [keyword, setKeyword] = useState("");
    const [sizeText, setSizeText] = useState("");
    const [size, setSize] = useState("");
    const [status, setStatus] = useState("");
    const [mode, setMode] = useState("");
    const [dateRange, setDateRange] = useState<[Dayjs, Dayjs] | null>(null);
    const [appliedDateRange, setAppliedDateRange] = useState<[Dayjs, Dayjs] | null>(null);
    const [detail, setDetail] = useState<HistoryItem | null>(null);
    const [markingId, setMarkingId] = useState("");

    const refresh = async () => {
        if (!token) {
            setTasks([]);
            setTotal(0);
            setLoading(false);
            return;
        }
        setLoading(true);
        try {
            const result = await loadTasks(token, {
                page,
                pageSize,
                keyword,
                type: mode,
                status,
                size,
                dateFrom: appliedDateRange?.[0].startOf("day").format("YYYY-MM-DD HH:mm:ss"),
                dateTo: appliedDateRange?.[1].endOf("day").format("YYYY-MM-DD HH:mm:ss"),
            });
            setTasks(result.items);
            setTotal(result.total);
        } catch (error) {
            const text = error instanceof Error ? error.message : "读取生图历史失败";
            if (text.includes("未登录") || text.includes("登录状态无效")) clearSession();
            message.error(text);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        void refresh();
    }, [appliedDateRange, keyword, mode, page, pageSize, size, status, token]);

    const items = useMemo<HistoryItem[]>(
        () =>
            tasks.map((task) => {
                const log = taskToLog(task);
                return { id: log.id, log, image: log.image || emptyHistoryImage(log), mode: log.referenceCount ? "edit" : "generate" };
            }),
        [tasks],
    );

    const search = () => {
        setKeyword(keywordText.trim());
        setSize(sizeText.trim());
        setAppliedDateRange(dateRange);
        setPage(1);
    };

    const reset = () => {
        setKeywordText("");
        setKeyword("");
        setSizeText("");
        setSize("");
        setStatus("");
        setMode("");
        setDateRange(null);
        setAppliedDateRange(null);
        setPage(1);
    };

    const copyPrompt = async (prompt: string) => {
        await navigator.clipboard.writeText(prompt);
        message.success("已复制提示词");
    };

    const toggleFeatured = async (taskId: string, featured: boolean) => {
        if (!onToggleFeatured) return;
        const task = tasks.find((item) => item.taskId === taskId || item.id === taskId);
        if (!task) return;
        setMarkingId(taskId);
        try {
            if (!token) return;
            const saved = await onToggleFeatured(token, task, featured);
            setTasks((current) => current.map((item) => (item.id === saved.id || item.taskId === saved.taskId ? saved : item)));
            message.success(featured ? "已展示到首页" : "已取消首页展示");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "更新首页展示失败");
        } finally {
            setMarkingId("");
        }
    };

    return (
        <main className="h-full min-h-0 overflow-y-auto bg-[#f7f3ee] px-3 py-6 text-stone-950 sm:px-6">
            <div className="mb-6 flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
                <div>
                    <Typography.Text className="tracking-[0.32em] !text-stone-500">{eyebrow}</Typography.Text>
                    <Typography.Title level={2} className="!m-0">
                        {title}
                    </Typography.Title>
                </div>
                <div className="grid gap-3 md:grid-cols-2 xl:flex xl:items-center">
                    <DatePicker.RangePicker value={dateRange} onChange={(dates) => setDateRange(dates as [Dayjs, Dayjs] | null)} placeholder={["选择日期范围", "结束日期"]} suffixIcon={<CalendarDays className="size-4" />} className="h-11 min-w-[260px] rounded-2xl" />
                    <Input value={keywordText} onChange={(event) => setKeywordText(event.target.value)} onPressEnter={search} placeholder="提示词关键词" allowClear className="h-11 rounded-2xl" />
                    <Input value={sizeText} onChange={(event) => setSizeText(event.target.value)} onPressEnter={search} placeholder="尺寸 1024x1024" allowClear className="h-11 rounded-2xl" />
                    <Select value={status} onChange={setStatus} className="h-11 min-w-[140px]" options={[{ value: "", label: "全部状态" }, { value: "success", label: "成功" }, { value: "failed", label: "失败" }]} />
                    <Select value={mode} onChange={setMode} className="h-11 min-w-[140px]" options={[{ value: "", label: "全部模式" }, { value: "generate", label: "文生图" }, { value: "edit", label: "图生图" }]} />
                    <Button className="h-11 rounded-2xl" onClick={reset}>
                        清除筛选条件
                    </Button>
                    <Button type="primary" icon={<Search className="size-4" />} className="h-11 rounded-2xl !bg-black px-6" onClick={search}>
                        查询
                    </Button>
                </div>
            </div>

            <section className="overflow-hidden rounded-2xl border border-stone-200 bg-white shadow-sm">
                <div className="flex items-center justify-between border-b border-stone-100 px-6 py-5 text-stone-600">
                    <Space>
                        <ImageIcon className="size-4" />
                        <span>共 {total} 条</span>
                    </Space>
                    <Button type="text" icon={<RefreshCw className="size-4" />} onClick={() => void refresh()}>
                        刷新
                    </Button>
                </div>
                <Spin spinning={loading}>
                    {items.length ? (
                        <div className="grid grid-cols-1 divide-y divide-stone-100 md:grid-cols-2 md:divide-x md:divide-y-0 xl:grid-cols-4">
                            {items.map((item) => (
                                <HistoryCard key={item.id} item={item} userName={item.log.userId || defaultUserName} marking={markingId === item.log.taskId} onDetail={setDetail} onCopyPrompt={copyPrompt} onToggleFeatured={onToggleFeatured ? toggleFeatured : undefined} />
                            ))}
                        </div>
                    ) : (
                        <div className="py-24">
                            <Empty description={loading ? "正在读取生成记录" : emptyText} />
                        </div>
                    )}
                </Spin>
                <div className="flex justify-center border-t border-stone-100 px-6 py-5">
                    <Pagination
                        current={page}
                        pageSize={pageSize}
                        total={total}
                        showSizeChanger
                        pageSizeOptions={[12, 24, 48, 96]}
                        showTotal={(value) => `共 ${value} 条`}
                        onChange={(nextPage, nextPageSize) => {
                            if (nextPageSize !== pageSize) {
                                setPageSize(nextPageSize);
                                setPage(1);
                                return;
                            }
                            setPage(nextPage);
                        }}
                    />
                </div>
            </section>

            <Modal open={Boolean(detail)} footer={null} width={1180} centered closeIcon={<X className="size-5" />} onCancel={() => setDetail(null)} styles={{ body: { padding: 28 } }}>
                {detail ? <HistoryDetail item={detail} userName={detail.log.userId || defaultUserName} onCopyPrompt={copyPrompt} onClose={() => setDetail(null)} /> : null}
            </Modal>
        </main>
    );
}

function HistoryCard({ item, userName, marking, onDetail, onCopyPrompt, onToggleFeatured }: { item: HistoryItem; userName: string; marking: boolean; onDetail: (item: HistoryItem) => void; onCopyPrompt: (prompt: string) => void; onToggleFeatured?: (taskId: string, featured: boolean) => void }) {
    const log = item.log;
    return (
        <article className="p-6">
            <button type="button" className="block w-full overflow-hidden rounded-lg bg-stone-100 text-left" onClick={() => onDetail(item)}>
                {item.image.dataUrl ? <Image src={item.image.dataUrl} alt={log.title || "生成图片"} preview={false} className="!h-[290px] !w-full object-cover" /> : <EmptyImagePlaceholder />}
            </button>
            <div className="mt-4 flex items-center justify-between text-sm text-stone-500">
                <span className="inline-flex items-center gap-1 font-medium">
                    <CalendarDays className="size-4" />
                    {formatDate(log.createdAt)}
                </span>
                <Space size={10}>
                    <Button type="text" size="small" icon={<Download className="size-4" />} onClick={() => saveAs(item.image.dataUrl, `image-${log.id}.png`)} />
                    <Button type="text" size="small" icon={<Copy className="size-4" />} onClick={() => void onCopyPrompt(log.prompt)} />
                </Space>
            </div>
            <div className="mt-3 rounded-lg bg-stone-50 p-3 text-sm leading-6 text-stone-600">
                <span className="font-semibold text-stone-800">{userName}</span>
                <span className="px-1">·</span>
                <span>{log.prompt || "暂无提示词"}</span>
            </div>
            <div className="mt-3 grid grid-cols-2 gap-2 text-sm text-stone-500">
                <span>{formatBytes(item.image.bytes) || "-"}</span>
                <span className="text-right">{itemSize(item)}</span>
                <span>生成用时 {formatDuration(item.image.durationMs || log.durationMs)}</span>
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
                <Tag className="m-0 rounded-full px-3">{item.mode}</Tag>
                <Tag className="m-0 rounded-full px-3">托管生图</Tag>
                <Tag className="m-0 rounded-full px-3">{statusValue(log.status)}</Tag>
                <Tag className="m-0 rounded-full px-3">{itemSize(item)}</Tag>
                <Tag className="m-0 rounded-full px-3">{log.credits ? `${log.credits} 积分` : "-"}</Tag>
                {log.featured ? <Tag color="gold" className="m-0 rounded-full px-3">首页展示</Tag> : null}
            </div>
            <Space.Compact block className="mt-3">
                <Button className="rounded-lg" icon={<Eye className="size-4" />} onClick={() => onDetail(item)}>
                    查看详情
                </Button>
                {onToggleFeatured ? (
                    <Button loading={marking} icon={<Star className="size-4" />} onClick={() => onToggleFeatured(log.taskId, !log.featured)}>
                        {log.featured ? "取消展示" : "首页展示"}
                    </Button>
                ) : null}
            </Space.Compact>
        </article>
    );
}

function HistoryDetail({ item, userName, onCopyPrompt, onClose }: { item: HistoryItem; userName: string; onCopyPrompt: (prompt: string) => void; onClose: () => void }) {
    const log = item.log;
    const fields = [
        ["用户", userName],
        ["时间", formatDate(log.createdAt)],
        ["生成用时", formatDuration(item.image.durationMs || log.durationMs)],
        ["模式", item.mode],
        ["目标尺寸", log.size || log.config.size || "-"],
        ["尺寸状态", sizeHitLabel(item)],
        ["参考图", `${log.referenceCount} 张${log.referenceCount > 1 ? ` / 附加 ${log.referenceCount - 1} 张` : ""}`],
        ["来源", "platform"],
        ["任务", log.taskId],
        ["更新", formatDate(log.updatedAt || log.createdAt)],
        ["模型", log.model || log.config.imageModel || log.config.model || "-"],
        ["生图链路", "托管生图"],
        ["实际尺寸", itemSize(item)],
        ["金额", log.credits ? `${log.credits} 积分` : "-"],
        ["状态", statusValue(log.status)],
        ["首页展示", log.featured ? "是" : "否"],
    ];

    return (
        <div>
            <Typography.Title level={3} className="!mb-6">
                生成记录详情
            </Typography.Title>
            <div className="grid gap-7 lg:grid-cols-[460px_1fr]">
                {item.image.dataUrl ? <Image src={item.image.dataUrl} alt={log.title || "生成图片"} className="!aspect-square !w-full rounded-xl object-cover" /> : <EmptyImagePlaceholder large />}
                <div>
                    <div className="grid gap-x-10 gap-y-4 md:grid-cols-2">
                        {fields.map(([label, value]) => (
                            <div key={label} className="text-lg text-stone-500">
                                {label}：<span className="text-stone-700">{value}</span>
                            </div>
                        ))}
                    </div>
                    <div className="mt-5 text-lg text-stone-500">完整提示词</div>
                    <div className="mt-3 min-h-[120px] rounded-2xl bg-stone-50 p-5 text-base leading-8 text-stone-800">{log.prompt || "暂无提示词"}</div>
                    <div className="mt-8 flex flex-wrap justify-end gap-3">
                        <Button size="large" icon={<Download className="size-5" />} onClick={() => saveAs(item.image.dataUrl, `image-${log.id}.png`)}>
                            下载图片
                        </Button>
                        <Button size="large" icon={<Copy className="size-5" />} onClick={() => void onCopyPrompt(log.prompt)}>
                            复制提示词
                        </Button>
                        <Button size="large" type="primary" className="!bg-black" onClick={onClose}>
                            关闭
                        </Button>
                    </div>
                </div>
            </div>
        </div>
    );
}

function taskToLog(task: AIImageTask): GenerationLog {
    const createdAt = parseDate(task.createdAt);
    const updatedAt = parseDate(task.updatedAt);
    const mode = task.path.includes("/images/edits") ? "edit" : "generate";
    const durationMs = updatedAt > createdAt ? updatedAt - createdAt : 0;
    const image = task.imageUrl && task.imageUrl !== "[b64_json]" ? taskImage(task, durationMs) : null;
    return {
        id: task.id || task.taskId,
        taskId: task.taskId,
        userId: task.userId,
        createdAt,
        updatedAt,
        title: task.prompt.slice(0, 12) || task.model || "未命名",
        prompt: task.prompt,
        model: task.model,
        config: { model: task.model, imageModel: task.model, quality: task.quality, size: task.size, count: String(task.count || 1) },
        durationMs,
        credits: task.credits || 0,
        referenceCount: task.referenceCount || (mode === "edit" ? 1 : 0),
        size: task.size,
        quality: task.quality,
        status: isSuccessStatus(task.status) ? "成功" : "失败",
        featured: task.featured === true,
        image,
    };
}

function taskImage(task: AIImageTask, durationMs: number): GeneratedImage {
    const size = parseSize(task.size);
    return { id: task.taskId, dataUrl: task.imageUrl, durationMs, width: size.width, height: size.height, bytes: 0, mimeType: "image/png" };
}

function emptyHistoryImage(log: GenerationLog): GeneratedImage {
    const size = parseSize(log.size || log.config.size || "");
    return { id: `${log.id}-empty`, dataUrl: "", durationMs: log.durationMs, width: size.width, height: size.height, bytes: 0, mimeType: "image/png" };
}

function EmptyImagePlaceholder({ large = false }: { large?: boolean }) {
    return (
        <div className={`grid w-full place-items-center bg-stone-100 text-stone-400 ${large ? "aspect-square rounded-xl" : "h-[290px]"}`}>
            <div className="flex flex-col items-center gap-2">
                <ImageIcon className="size-8" />
                <span className="text-sm">暂无图片</span>
            </div>
        </div>
    );
}

function parseDate(value: string) {
    const parsed = Date.parse(value || "");
    return Number.isFinite(parsed) ? parsed : Date.now();
}

function parseSize(value: string) {
    const match = value.match(/^(\d+)x(\d+)$/i);
    return match ? { width: Number(match[1]), height: Number(match[2]) } : { width: 0, height: 0 };
}

function isSuccessStatus(status: string) {
    return ["succeeded", "success", "completed"].includes(status.trim().toLowerCase());
}

function itemSize(item: HistoryItem) {
    const width = Number(item.image.width) || 0;
    const height = Number(item.image.height) || 0;
    return width && height ? `${width}x${height}` : item.log.size || item.log.config.size || "-";
}

function sizeHitLabel(item: HistoryItem) {
    const target = item.log.size || item.log.config.size || "";
    return target && target === itemSize(item) ? "已命中" : target ? "未命中" : "-";
}

function statusValue(status: GenerationLog["status"]) {
    return status === "失败" ? "failed" : "success";
}

function formatDate(value: number) {
    return dayjs(value).format("YYYY-MM-DD HH:mm:ss");
}
