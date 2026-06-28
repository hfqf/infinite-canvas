import Link from "next/link";

const works = [
    { title: "日式面包手作招牌面", cat: "门头招牌", imgUrl: "https://images.unsplash.com/photo-1554118811-1e0d58224f24?auto=format&fit=crop&w=600&q=80" },
    { title: "酸性画廊版式宣发海报", cat: "海报设计", imgUrl: "https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?auto=format&fit=crop&w=600&q=80" },
    { title: "暮光之下奢享西餐菜单", cat: "菜单设计", imgUrl: "https://images.unsplash.com/photo-1514933651103-005eec06c04b?auto=format&fit=crop&w=600&q=80" },
    { title: "暮光金属背光发光招牌字", cat: "门头招牌", imgUrl: "https://images.unsplash.com/photo-1543007630-9710e4a00a20?auto=format&fit=crop&w=600&q=80" },
];

export function SiteGallery() {
    return (
        <section id="gallery-section" className="bg-white px-4 py-16 sm:px-6 lg:px-8">
            <div className="mx-auto max-w-7xl space-y-10">
                <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
                    <div className="space-y-2">
                        <span className="text-[11px] font-black uppercase tracking-widest text-indigo-600">Inspirational Showcase / 商用设计库</span>
                        <h2 className="text-3xl font-black tracking-tight text-slate-900">好图秀高精方案画廊</h2>
                        <p className="max-w-xl text-sm leading-7 text-slate-500">第一版先用静态视觉卡保留 figo 首页质感；后续可切到 canvas 精选生图任务。</p>
                    </div>
                    <Link href="/workbench" className="inline-flex w-fit items-center rounded-lg border border-slate-200 bg-slate-100 px-5 py-2.5 text-xs font-black text-slate-800 transition hover:bg-slate-200">
                        去工作台生成我的设计
                    </Link>
                </div>
                <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
                    {works.map((work) => (
                        <div key={work.title} className="group overflow-hidden rounded-xl border border-slate-200 bg-slate-50 shadow-sm transition hover:shadow-md">
                            <div className="relative aspect-square overflow-hidden bg-slate-100">
                                <img src={work.imgUrl} alt={work.title} className="h-full w-full object-cover transition duration-500 group-hover:scale-105" referrerPolicy="no-referrer" />
                                <span className="absolute left-2 top-2 rounded bg-blue-600 px-2 py-1 text-[9px] font-black text-white">{work.cat}</span>
                            </div>
                            <div className="p-4">
                                <h3 className="truncate text-sm font-black text-slate-900">{work.title}</h3>
                                <p className="mt-1 text-[10px] text-slate-500">智能工艺引擎 v3</p>
                            </div>
                        </div>
                    ))}
                </div>
            </div>
        </section>
    );
}
