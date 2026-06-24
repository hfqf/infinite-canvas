"use client";

import { CalendarDays, Copy, Download, Eye, ImageIcon, RefreshCw, Search, X } from "lucide-react";
import { App, Button, DatePicker, Empty, Image, Input, Modal, Select, Space, Spin, Tag, Typography } from "antd";
import type { Dayjs } from "dayjs";
import dayjs from "dayjs";
import { saveAs } from "file-saver";
import localforage from "localforage";
import { nanoid } from "nanoid";
import { useEffect, useMemo, useState } from "react";

import { formatBytes, formatDuration } from "@/lib/image-utils";
import { resolveImageUrl } from "@/services/image-storage";
import { useUserStore } from "@/stores/use-user-store";
import type { ReferenceImage } from "@/types/image";

type GeneratedImage = {
    id: string;
    dataUrl: string;
    storageKey?: string;
    durationMs: number;
    width: number;
    height: number;
    bytes: number;
    mimeType?: string;
};

type GenerationLogConfig = {
    model?: string;
    imageModel?: string;
    quality?: string;
    size?: string;
    count?: string | number;
};

type GenerationLog = {
    id: string;
    createdAt: number;
    title: string;
    prompt: string;
    time: string;
    model: string;
    config: GenerationLogConfig;
    references: ReferenceImage[];
    durationMs: number;
    successCount: number;
    failCount: number;
    imageCount: number;
    size: string;
    quality: string;
    status: "成功" | "失败";
    images: GeneratedImage[];
    thumbnails: string[];
};

type HistoryItem = {
    id: string;
    log: GenerationLog;
    image: GeneratedImage;
    imageIndex: number;
    mode: "generate" | "edit";
};

const logStore = localforage.createInstance({ name: "infinite-canvas", storeName: "image_generation_logs" });

export default function ImageHistoryPage() {
    const { message } = App.useApp();
    const user = useUserStore((state) => state.user);
    const [logs, setLogs] = useState<GenerationLog[]>([]);
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

    const refresh = async () => {
        setLoading(true);
        setLogs(await readStoredLogs());
        setLoading(false);
    };

    useEffect(() => {
        void refresh();
    }, []);

    const items = useMemo<HistoryItem[]>(
        () =>
            logs.flatMap((log) =>
                log.images.map((image, index) => ({
                    id: `${log.id}-${image.id || index}`,
                    log,
                    image,
                    imageIndex: index,
                    mode: log.references.length ? "edit" : "generate",
                })),
            ),
        [logs],
    );

    const filteredItems = useMemo(() => {
        const normalizedKeyword = keyword.trim().toLowerCase();
        const normalizedSize = size.trim().toLowerCase();
        return items.filter((item) => {
            const createdAt = dayjs(item.log.createdAt);
            if (appliedDateRange && (createdAt.isBefore(appliedDateRange[0], "day") || createdAt.isAfter(appliedDateRange[1], "day"))) return false;
            if (normalizedKeyword && !`${item.log.prompt} ${item.log.model}`.toLowerCase().includes(normalizedKeyword)) return false;
            if (normalizedSize && !itemSize(item).toLowerCase().includes(normalizedSize)) return false;
            if (status && status !== statusValue(item.log.status)) return false;
            if (mode && mode !== item.mode) return false;
            return true;
        });
    }, [appliedDateRange, items, keyword, mode, size, status]);

    const search = () => {
        setKeyword(keywordText.trim());
        setSize(sizeText.trim());
        setAppliedDateRange(dateRange);
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
    };

    const copyPrompt = async (prompt: string) => {
        await navigator.clipboard.writeText(prompt);
        message.success("已复制提示词");
    };

    return (
        <main className="min-h-screen bg-[#f7f3ee] px-3 py-6 text-stone-950 sm:px-6">
            <div className="mb-6 flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
                <div>
                    <Typography.Text className="tracking-[0.32em] !text-stone-500">IMAGES</Typography.Text>
                    <Typography.Title level={2} className="!m-0">
                        图片管理
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
                        <span>共 {filteredItems.length} 张</span>
                    </Space>
                    <Button type="text" icon={<RefreshCw className="size-4" />} onClick={() => void refresh()}>
                        刷新
                    </Button>
                </div>
                <Spin spinning={loading}>
                    {filteredItems.length ? (
                        <div className="grid grid-cols-1 divide-y divide-stone-100 md:grid-cols-2 md:divide-x md:divide-y-0 xl:grid-cols-4">
                            {filteredItems.map((item) => (
                                <HistoryCard key={item.id} item={item} userName={userLabel(user)} onDetail={setDetail} onCopyPrompt={copyPrompt} />
                            ))}
                        </div>
                    ) : (
                        <div className="py-24">
                            <Empty description={loading ? "正在读取生成记录" : "暂无生成记录"} />
                        </div>
                    )}
                </Spin>
            </section>

            <Modal open={Boolean(detail)} footer={null} width={1180} centered closeIcon={<X className="size-5" />} onCancel={() => setDetail(null)} styles={{ body: { padding: 28 } }}>
                {detail ? <HistoryDetail item={detail} userName={userLabel(user)} onCopyPrompt={copyPrompt} onClose={() => setDetail(null)} /> : null}
            </Modal>
        </main>
    );
}

function HistoryCard({ item, userName, onDetail, onCopyPrompt }: { item: HistoryItem; userName: string; onDetail: (item: HistoryItem) => void; onCopyPrompt: (prompt: string) => void }) {
    const log = item.log;
    return (
        <article className="p-6">
            <button type="button" className="block w-full overflow-hidden rounded-lg bg-stone-100 text-left" onClick={() => onDetail(item)}>
                <Image src={item.image.dataUrl} alt={log.title || "生成图片"} preview={false} className="!h-[290px] !w-full object-cover" />
            </button>
            <div className="mt-4 flex items-center justify-between text-sm text-stone-500">
                <span className="inline-flex items-center gap-1 font-medium">
                    <CalendarDays className="size-4" />
                    {formatDate(log.createdAt)}
                </span>
                <Space size={10}>
                    <Button type="text" size="small" icon={<Download className="size-4" />} onClick={() => saveAs(item.image.dataUrl, `image-${log.id}-${item.imageIndex + 1}.png`)} />
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
                <Tag className="m-0 rounded-full px-3">¥-</Tag>
            </div>
            <Button block className="mt-3 rounded-lg" icon={<Eye className="size-4" />} onClick={() => onDetail(item)}>
                查看详情
            </Button>
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
        ["参考图", `${log.references.length} 张${log.references.length > 1 ? ` / 附加 ${log.references.length - 1} 张` : ""}`],
        ["来源", "platform"],
        ["任务", log.id],
        ["更新", formatDate(log.createdAt)],
        ["模型", log.model || log.config.imageModel || log.config.model || "-"],
        ["生图链路", "托管生图"],
        ["实际尺寸", itemSize(item)],
        ["金额", "¥-"],
        ["状态", statusValue(log.status)],
    ];

    return (
        <div>
            <Typography.Title level={3} className="!mb-6">
                生成记录详情
            </Typography.Title>
            <div className="grid gap-7 lg:grid-cols-[460px_1fr]">
                <Image src={item.image.dataUrl} alt={log.title || "生成图片"} className="!aspect-square !w-full rounded-xl object-cover" />
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
                        <Button size="large" icon={<Download className="size-5" />} onClick={() => saveAs(item.image.dataUrl, `image-${log.id}-${item.imageIndex + 1}.png`)}>
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

async function readStoredLogs() {
    if (typeof window === "undefined") return [];
    try {
        const values: GenerationLog[] = [];
        await logStore.iterate<GenerationLog, void>((value) => {
            values.push(value);
        });
        const logs = await Promise.all(values.map(normalizeLog));
        return logs.sort((a, b) => (b.createdAt || 0) - (a.createdAt || 0));
    } catch {
        return [];
    }
}

async function normalizeLog(log: Partial<GenerationLog>): Promise<GenerationLog> {
    const references = await Promise.all(
        (log.references || []).map(async (item) => ({
            ...item,
            dataUrl: await resolveImageUrl(item.storageKey, item.dataUrl),
        })),
    );
    const images = await Promise.all(
        (log.images || []).map(async (item) => ({
            ...item,
            dataUrl: await resolveImageUrl(item.storageKey, item.dataUrl),
        })),
    );
    const config = normalizeLogConfig(log);
    return {
        id: log.id || nanoid(),
        createdAt: log.createdAt || Date.now(),
        title: log.title || log.model || "未命名",
        prompt: log.prompt || log.title || "",
        time: log.time || new Date().toLocaleString("zh-CN", { hour12: false }),
        model: log.model || config.imageModel || "",
        config,
        references,
        durationMs: log.durationMs || 0,
        successCount: log.successCount ?? log.imageCount ?? 0,
        failCount: log.failCount || 0,
        imageCount: log.imageCount || log.successCount || 0,
        size: log.size || config.size || "",
        quality: log.quality || "",
        status: log.status || "成功",
        images,
        thumbnails: images.map((image) => image.dataUrl).filter(Boolean),
    };
}

function normalizeLogConfig(log: Partial<GenerationLog>): GenerationLogConfig {
    return {
        model: log.config?.model || log.model || "",
        imageModel: log.config?.imageModel || log.model || "",
        quality: log.config?.quality || log.quality || "",
        size: log.config?.size || log.size || "",
        count: log.config?.count || String(log.imageCount || log.successCount || 1),
    };
}

function userLabel(user: ReturnType<typeof useUserStore.getState>["user"]) {
    return user?.email || user?.displayName || user?.username || "当前用户";
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
