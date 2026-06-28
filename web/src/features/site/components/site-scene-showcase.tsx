import { ChevronRight } from "lucide-react";
import Link from "next/link";

import { WORKBENCH_PRESETS, WORKBENCH_SCENES } from "@/features/workbench/scenes";

export function SiteSceneShowcase() {
    return (
        <section id="scenes-section" className="bg-white px-4 py-16 sm:px-6 lg:px-8">
            <div className="mx-auto max-w-7xl space-y-10">
                <div className="mx-auto max-w-3xl space-y-2 text-center">
                    <span className="text-[11px] font-black uppercase tracking-widest text-blue-600">Core Scenes / 首批商业场景</span>
                    <h2 className="text-3xl font-black tracking-tight text-slate-900">从真实商业任务开始，不从空白提示词开始</h2>
                    <p className="text-sm leading-7 text-slate-500">首批迁移门头招牌、海报设计和菜单设计。每个场景都有专属字段、参考图角色和提示词组织方式。</p>
                </div>
                <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
                    {WORKBENCH_SCENES.map((scene) => {
                        const preset = WORKBENCH_PRESETS[scene.id]?.[0];
                        return (
                            <Link key={scene.id} href={`/workbench?scene=${scene.id}`} className="group flex flex-col overflow-hidden rounded-2xl border border-slate-200 bg-slate-50 shadow-sm transition duration-300 hover:-translate-y-1 hover:border-blue-500 hover:bg-white hover:shadow-xl">
                                <div className="relative aspect-video overflow-hidden border-b border-slate-200 bg-slate-200">
                                    <img src={preset?.imgUrl} alt={scene.name} className="h-full w-full object-cover transition duration-700 group-hover:scale-105" referrerPolicy="no-referrer" />
                                    <span className="absolute left-3 top-3 rounded bg-slate-950/90 px-2 py-1 font-mono text-[9px] font-bold uppercase tracking-wide text-white">{scene.category}</span>
                                </div>
                                <div className="flex flex-1 flex-col justify-between gap-4 p-5">
                                    <div>
                                        <h3 className="text-lg font-black text-slate-900 transition group-hover:text-blue-600">{scene.name}</h3>
                                        <p className="mt-2 text-sm leading-6 text-slate-500">{scene.description}</p>
                                    </div>
                                    <div className="flex items-center justify-between border-t border-slate-200 pt-4">
                                        <span className="font-mono text-[10px] font-bold uppercase tracking-wider text-indigo-600">好图秀引擎 v3.0</span>
                                        <span className="inline-flex items-center gap-1 text-sm font-black text-blue-600 transition group-hover:translate-x-1">
                                            一键开始设计
                                            <ChevronRight className="size-4" />
                                        </span>
                                    </div>
                                </div>
                            </Link>
                        );
                    })}
                </div>
            </div>
        </section>
    );
}
