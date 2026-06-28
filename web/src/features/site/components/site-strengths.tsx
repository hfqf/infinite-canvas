import { CheckCircle, Grid, RotateCcw, SlidersHorizontal, Upload } from "lucide-react";

const strengths = [
    { title: "彻底场景化的商业出图流程", desc: "围绕门头、海报、菜单等真实项目拆字段，不要求老板或设计师先学复杂咒语。", icon: SlidersHorizontal, className: "md:col-span-3", color: "bg-blue-100 text-blue-600" },
    { title: "参考图/现场原图融合", desc: "支持现场照片、产品图和风格参考图，用图片角色约束构图、透视、材质和主体一致性。", icon: Upload, className: "md:col-span-3", color: "bg-indigo-100 text-indigo-600" },
    { title: "实体物料材质表达", desc: "强调金属、木质、背发光、岩石肌理等商业制作常见材质，方便方案沟通。", icon: CheckCircle, className: "md:col-span-2", color: "bg-violet-100 text-violet-600" },
    { title: "动态模板管线", desc: "预设不是摆设，会进入提示词和生成元数据，后续可继续扩充更多行业模板。", icon: Grid, className: "md:col-span-2", color: "bg-emerald-100 text-emerald-600" },
    { title: "可迭代复用", desc: "生成结果可继续作为参考图，围绕同一方案反复换材质、换文案、换版式。", icon: RotateCcw, className: "md:col-span-2", color: "bg-amber-100 text-amber-600" },
];

export function SiteStrengths() {
    return (
        <section className="border-y border-slate-200/70 bg-slate-50 px-4 py-16 sm:px-6 lg:px-8">
            <div className="mx-auto max-w-7xl space-y-10">
                <div className="mx-auto max-w-2xl space-y-2 text-center">
                    <span className="text-[11px] font-black uppercase tracking-widest text-indigo-600">Product Strengths / 好图秀核心能力</span>
                    <h2 className="text-3xl font-black tracking-tight text-slate-900">为什么适合商业设计沟通？</h2>
                    <p className="text-sm leading-7 text-slate-500">用场景、字段、参考图和扣费流水把前端体验接到统一 canvas 服务上。</p>
                </div>
                <div className="grid gap-6 md:grid-cols-6">
                    {strengths.map((item) => {
                        const Icon = item.icon;
                        return (
                            <div key={item.title} className={`${item.className} rounded-2xl border border-slate-200 bg-white p-6 shadow-sm transition hover:border-blue-400`}>
                                <div className={`mb-4 flex size-10 items-center justify-center rounded-xl ${item.color}`}>
                                    <Icon className="size-5" />
                                </div>
                                <h3 className="text-lg font-black text-slate-900">{item.title}</h3>
                                <p className="mt-3 text-sm leading-7 text-slate-500">{item.desc}</p>
                            </div>
                        );
                    })}
                </div>
            </div>
        </section>
    );
}
