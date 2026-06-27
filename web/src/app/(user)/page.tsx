"use client";

import { ArrowRight } from "lucide-react";
import { type ReactNode, useEffect, useMemo, useState } from "react";
import { App, Button, Empty, Image, Modal, Space, Tag, Typography } from "antd";
import dayjs from "dayjs";

import { navigationTools } from "@/constant/navigation-tools";
import { fetchFeaturedImageTasks, type AIImageTask } from "@/services/api/image-tasks";

function Highlighter({ action, color, children }: { action: "highlight" | "underline"; color: string; children: ReactNode }) {
    return (
        <span className="relative inline-block px-1">
            {action === "highlight" ? (
                <span className="absolute inset-x-0 bottom-0 top-1 rounded-sm opacity-45" style={{ backgroundColor: color }} />
            ) : (
                <span className="absolute inset-x-0 bottom-0 h-1 rounded-full opacity-80" style={{ backgroundColor: color }} />
            )}
            <span className="relative font-medium text-stone-800 dark:text-stone-200">{children}</span>
        </span>
    );
}

export default function IndexPage() {
    const { message } = App.useApp();
    const [primaryTool] = navigationTools;
    const [items, setItems] = useState<AIImageTask[]>([]);
    const [detail, setDetail] = useState<AIImageTask | null>(null);

    useEffect(() => {
        void fetchFeaturedImageTasks({ pageSize: 30 })
            .then((data) => setItems(data.items))
            .catch((error) => message.error(error instanceof Error ? error.message : "获取首页图片失败"));
    }, [message]);

    return (
        <main className="relative h-full overflow-y-auto bg-background bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] [background-size:16px_16px] text-stone-950 dark:bg-[radial-gradient(rgba(245,245,244,.18)_1px,transparent_1px)] dark:text-stone-100">
            <section className="relative mx-auto min-h-[calc(100vh-4rem)] max-w-7xl overflow-hidden px-6">
                <div className="pointer-events-none absolute left-[15%] top-24 size-20 rounded-full border border-dashed border-stone-200 dark:border-stone-800" />
                <div className="pointer-events-none absolute right-[23%] top-[48%] size-20 rounded-full border border-dashed border-stone-200 dark:border-stone-800" />

                <div className="relative flex min-h-[620px] flex-col items-center justify-center pt-10 text-center">
                    <h1 className="ai-title-aurora max-w-5xl text-balance text-5xl font-semibold tracking-normal sm:text-7xl lg:text-8xl">无限画布</h1>
                    <p className="mt-8 max-w-3xl text-balance text-lg leading-8 text-stone-500 dark:text-stone-400">
                        在
                        <Highlighter action="underline" color="#FF9800">
                            好图秀AI画布
                        </Highlighter>
                        中生成、连接和重组
                        <Highlighter action="highlight" color="#87CEFA">
                            图片、文字与图形
                        </Highlighter>
                        ，让创作从单次生成变成连续推演。
                    </p>
                    <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
                        <Button type="primary" size="large" href={`/${primaryTool.slug}`} icon={<ArrowRight className="size-4" />} iconPlacement="end">
                            开始使用
                        </Button>
                        <Button size="large" href="/canvas">
                            打开画布
                        </Button>
                    </div>
                </div>

                <section className="relative mx-auto mb-20 max-w-6xl border-t border-stone-200 pt-12 dark:border-stone-800">
                    <div className="mb-8 grid gap-4 md:grid-cols-[1fr_auto_1fr] md:items-start">
                        <div />
                        <div className="max-w-2xl text-center">
                            <h2 className="text-3xl font-semibold text-stone-950 dark:text-stone-100">来自画布的真实作品</h2>
                            <p className="mt-3 text-base leading-7 text-stone-500 dark:text-stone-400">这里展示的图片全部来自系统内已完成的生图记录，由管理员精选后按时间倒序展示。</p>
                        </div>
                        <Button type="link" href="/canvas" className="justify-self-center md:justify-self-end" icon={<ArrowRight className="size-4" />} iconPlacement="end">
                            去创作
                        </Button>
                    </div>
                    {items.length ? <HomeImageMasonry items={items} onDetail={setDetail} /> : <Empty className="py-20" description="暂无精选图片" />}
                </section>
                <SiteFooter />
            </section>
            <Modal open={Boolean(detail)} footer={null} width={980} centered onCancel={() => setDetail(null)}>
                {detail ? <HomeImageDetail item={detail} /> : null}
            </Modal>
        </main>
    );
}

function SiteFooter() {
    return (
        <footer className="border-t border-stone-200 py-8 dark:border-stone-800">
            <div className="flex flex-col items-center justify-center gap-4 text-sm text-stone-500 md:flex-row dark:text-stone-400">
                <div className="flex flex-wrap items-center justify-center gap-1">
                    <span>© 2026 好图秀AI haotushow.com</span>
                    <span className="mx-2 text-stone-300 dark:text-stone-700">|</span>
                    <span>Made with Pure Joy</span>
                </div>

                <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer" className="group flex items-center gap-1.5 rounded-full border border-stone-200 px-3 py-1.5 text-stone-500 transition-all duration-200 hover:border-stone-400 hover:bg-stone-950/5 hover:text-stone-700 dark:border-stone-800 dark:text-stone-400 dark:hover:border-stone-600 dark:hover:bg-white/5 dark:hover:text-stone-200">
                    <svg className="h-3.5 w-3.5 transition-colors group-hover:text-stone-700 dark:group-hover:text-stone-200" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
                        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                    </svg>
                    <span className="font-medium">苏ICP备18027098号-7</span>
                </a>

                <a href="https://beian.mps.gov.cn/#/query/webSearch?code=32011502013847" target="_blank" rel="noreferrer" className="group flex items-center gap-1.5 rounded-full border border-stone-200 px-3 py-1.5 text-stone-500 transition-all duration-200 hover:border-stone-400 hover:bg-stone-950/5 hover:text-stone-700 dark:border-stone-800 dark:text-stone-400 dark:hover:border-stone-600 dark:hover:bg-white/5 dark:hover:text-stone-200">
                    <svg className="h-3.5 w-3.5 transition-colors group-hover:text-stone-700 dark:group-hover:text-stone-200" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
                        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                    </svg>
                    <span className="font-medium">苏公网安备32011502013847号</span>
                </a>
            </div>
        </footer>
    );
}

function HomeImageMasonry({ items, onDetail }: { items: AIImageTask[]; onDetail: (item: AIImageTask) => void }) {
    return (
        <div className="columns-1 gap-4 sm:columns-2 lg:columns-3 xl:columns-4">
            {items.map((item) => (
                <button key={item.id || item.taskId} type="button" className="group mb-4 block w-full break-inside-avoid overflow-hidden rounded-2xl border border-stone-200 bg-stone-100 text-left shadow-sm transition duration-300 hover:-translate-y-1 hover:shadow-xl dark:border-stone-800 dark:bg-stone-900" onClick={() => onDetail(item)}>
                    <div className="relative w-full overflow-hidden" style={{ aspectRatio: homeImageRatio(item) }}>
                        <img src={item.imageUrl} alt={item.prompt || "精选图片"} className="h-full w-full object-cover transition duration-500 group-hover:scale-[1.03]" />
                        <div className="absolute inset-x-0 bottom-0 overflow-hidden bg-gradient-to-t from-black/85 via-black/60 to-transparent p-4 text-white" style={{ maxHeight: "50%" }}>
                            <p className="line-clamp-6 text-sm leading-6">{item.prompt || "暂无提示词"}</p>
                        </div>
                    </div>
                </button>
            ))}
        </div>
    );
}

function HomeImageDetail({ item }: { item: AIImageTask }) {
    const size = parseSize(item.size);
    const ratio = size.width && size.height ? `${size.width}:${size.height}` : "-";
    const mode = item.path.includes("/images/edits") ? "图生图" : "文生图";
    const fields = [
        ["模型", item.model || "-"],
        ["模式", mode],
        ["图片比例", ratio],
        ["目标尺寸", item.size || "-"],
        ["参考图", `${item.referenceCount || 0} 张`],
        ["积分", item.credits ? `${item.credits} 积分` : "-"],
        ["状态", item.status || "-"],
        ["时间", item.createdAt ? dayjs(item.createdAt).format("YYYY-MM-DD HH:mm:ss") : "-"],
        ["任务", item.taskId || item.id],
    ];

    return (
        <div className="grid gap-6 lg:grid-cols-[420px_1fr]">
            <Image src={item.imageUrl} alt={item.prompt || "精选图片"} className="!aspect-square !w-full rounded-2xl object-cover" />
            <div>
                <Typography.Title level={3} className="!mb-4">
                    生成记录详情
                </Typography.Title>
                <div className="grid gap-3 md:grid-cols-2">
                    {fields.map(([label, value]) => (
                        <div key={label} className="text-sm text-stone-500">
                            {label}：<span className="text-stone-800">{value}</span>
                        </div>
                    ))}
                </div>
                <div className="mt-5 text-sm text-stone-500">完整提示词</div>
                <div className="mt-2 max-h-[260px] overflow-y-auto rounded-2xl bg-stone-50 p-4 text-base leading-8 text-stone-800">{item.prompt || "暂无提示词"}</div>
                <Space wrap className="mt-5">
                    <Tag>{mode}</Tag>
                    <Tag>{item.size || "-"}</Tag>
                    <Tag>{item.status || "-"}</Tag>
                </Space>
            </div>
        </div>
    );
}

function homeImageRatio(item: AIImageTask) {
    const size = parseSize(item.size);
    if (size.width && size.height) return `${size.width} / ${size.height}`;
    return item.path.includes("/images/edits") ? "1 / 1" : "4 / 5";
}

function parseSize(value: string) {
    const match = value.match(/^(\d+)x(\d+)$/i);
    return match ? { width: Number(match[1]), height: Number(match[2]) } : { width: 0, height: 0 };
}
