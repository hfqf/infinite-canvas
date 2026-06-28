import { Check, ChevronDown, Play, Sparkles } from "lucide-react";
import Link from "next/link";

const heroFeatures = ["零门槛：无需繁杂 Prompt", "高拟真：材质与灯效真实重现", "智能控：支持现场参考图", "生产级：面向实体商业落地"];

export function SiteHero() {
    return (
        <section className="relative overflow-hidden bg-gradient-to-b from-white via-slate-100 to-slate-50 px-4 py-16 sm:px-6 lg:px-8 lg:py-24">
            <div className="pointer-events-none absolute left-1/4 top-20 size-80 rounded-full bg-indigo-200/30 blur-3xl" />
            <div className="pointer-events-none absolute right-1/4 top-1/2 size-80 rounded-full bg-blue-300/20 blur-3xl" />
            <div className="relative z-[1] mx-auto grid max-w-7xl items-center gap-10 lg:grid-cols-12">
                <div className="space-y-6 text-left lg:col-span-6">
                    <div className="inline-flex items-center gap-2 rounded-full border border-blue-100 bg-blue-50 px-3 py-1 text-blue-700 shadow-sm">
                        <Sparkles className="size-3.5 text-blue-600" />
                        <span className="text-xs font-black uppercase tracking-wider">Commercial Design AI Studio · 2026</span>
                    </div>
                    <h1 className="max-w-3xl text-4xl font-black leading-tight tracking-normal text-slate-950 sm:text-5xl lg:text-6xl">
                        好图秀 <span className="bg-gradient-to-r from-blue-600 via-indigo-600 to-violet-600 bg-clip-text text-transparent">AI 商业设计图</span>一键生成平台
                    </h1>
                    <p className="max-w-2xl text-base leading-8 text-slate-600 sm:text-lg">
                        专门针对门头招牌、商业海报、菜单物料等实体商业设计的 AI 系统。选择场景、录入项目信息，快速输出可沟通、可落地的高精度方案图。
                    </p>
                    <div className="grid gap-3 pt-1 sm:grid-cols-2">
                        {heroFeatures.map((item) => (
                            <div key={item} className="flex items-center gap-2 text-sm font-bold text-slate-700">
                                <span className="flex size-4 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-emerald-600">
                                    <Check className="size-2.5" />
                                </span>
                                <span>{item}</span>
                            </div>
                        ))}
                    </div>
                    <div className="flex flex-col gap-3 pt-4 sm:flex-row">
                        <Link href="/workbench" className="inline-flex items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-blue-600 to-indigo-600 px-8 py-3.5 text-sm font-black text-white shadow-xl shadow-blue-600/25 transition hover:from-blue-500 hover:to-indigo-500">
                            <Play className="size-4 fill-current" />
                            立即进入工作台设计
                        </Link>
                        <a href="#scenes-section" className="inline-flex items-center justify-center gap-2 rounded-xl border border-slate-200 bg-white px-6 py-3.5 text-sm font-black text-slate-700 shadow-sm transition hover:border-blue-200 hover:text-blue-600">
                            浏览商业场景
                            <ChevronDown className="size-4" />
                        </a>
                    </div>
                    <p className="border-t border-slate-200 pt-4 text-xs leading-6 text-slate-500">
                        面向广告公司、零售店主、连锁品牌和商业美陈团队，把方案沟通从“反复描述”变成“先看效果”。
                    </p>
                </div>
                <div className="relative flex justify-center lg:col-span-6">
                    <div className="relative w-full max-w-xl">
                        <div className="absolute inset-0 -z-[1] rotate-2 rounded-2xl bg-gradient-to-tr from-indigo-500/20 to-transparent blur-md" />
                        <div className="rounded-2xl border border-slate-200 bg-white p-3 shadow-2xl">
                            <div className="group relative aspect-video overflow-hidden rounded-xl">
                                <img src="https://images.unsplash.com/photo-1543007630-9710e4a00a20?auto=format&fit=crop&w=1200&q=80" alt="好图秀 AI 商业招牌设计效果" className="h-full w-full object-cover transition duration-700 group-hover:scale-105" referrerPolicy="no-referrer" />
                                <div className="absolute inset-0 flex flex-col justify-end bg-gradient-to-t from-slate-950/85 via-transparent to-transparent p-5 text-white">
                                    <span className="mb-1.5 w-fit rounded bg-indigo-500 px-2 py-0.5 font-mono text-[9px] font-bold uppercase tracking-wider">#1 STOREFRONT DESIGN</span>
                                    <h2 className="text-base font-black">门店招牌效果样图：金属拉丝 + 暖背光字工艺</h2>
                                    <p className="mt-1 truncate text-[11px] text-slate-300">AETHER COFFEE LAB · 现代金属面板 · 暖黄背光源</p>
                                </div>
                            </div>
                            <div className="mt-2 grid grid-cols-3 gap-2">
                                {[
                                    ["海报/宣发", "https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?auto=format&fit=crop&w=600&q=80"],
                                    ["菜单物料", "https://images.unsplash.com/photo-1514933651103-005eec06c04b?auto=format&fit=crop&w=600&q=80"],
                                    ["品牌视觉", "https://images.unsplash.com/photo-1509343256512-d77a5cb3791b?auto=format&fit=crop&w=600&q=80"],
                                ].map(([title, imgUrl]) => (
                                    <div key={title} className="relative aspect-video overflow-hidden rounded-lg border border-slate-100">
                                        <img src={imgUrl} alt={title} className="h-full w-full object-cover" referrerPolicy="no-referrer" />
                                        <div className="absolute inset-0 flex items-center justify-center bg-slate-950/40 p-1 text-center text-[10px] font-black text-white">{title}</div>
                                    </div>
                                ))}
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </section>
    );
}
