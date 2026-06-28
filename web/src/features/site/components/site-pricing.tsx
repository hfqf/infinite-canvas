import Link from "next/link";

export function SitePricing() {
    return (
        <section id="credits-pricing-section" className="bg-white px-4 py-16 sm:px-6 lg:px-8">
            <div className="mx-auto max-w-7xl">
                <div className="overflow-hidden rounded-3xl bg-gradient-to-r from-blue-600 to-indigo-700 p-6 text-white shadow-xl lg:p-10">
                    <div className="grid items-center gap-8 lg:grid-cols-12">
                        <div className="space-y-4 lg:col-span-8">
                            <span className="inline-flex rounded-full bg-white/20 px-3 py-1 text-[10px] font-black uppercase tracking-widest">积分消耗系统 · 统一 canvas 账户</span>
                            <h2 className="text-3xl font-black tracking-tight">极速省心的积分消耗制，拒绝繁琐隐性绑定</h2>
                            <p className="max-w-2xl text-sm leading-7 text-blue-50">工作台生成会走 canvas 统一远程生图服务，扣费流水、生图历史、余额刷新和后台管理保持一套账。页面不保留 figo 的模拟充值逻辑。</p>
                            <div className="flex flex-wrap gap-4 text-xs font-bold text-indigo-100">
                                <span>✓ 自动预扣与释放</span>
                                <span>✓ 生成记录进入生图历史</span>
                                <span>✓ 消费流水两端可查</span>
                            </div>
                        </div>
                        <div className="flex flex-col items-stretch gap-3 lg:col-span-4 lg:items-end">
                            <div className="w-full max-w-sm rounded-2xl border border-white/20 bg-white/10 p-5 text-center backdrop-blur-md">
                                <p className="font-mono text-[10px] font-black uppercase text-blue-100">MY CREDITS</p>
                                <h3 className="py-1 text-3xl font-black text-white">按账户余额结算</h3>
                                <p className="text-[10px] leading-5 text-blue-100">以 canvas 后台真实套餐与扣费规则为准</p>
                            </div>
                            <Link href="/deduction-logs" className="inline-flex w-full max-w-sm justify-center rounded-xl bg-white px-6 py-3 text-sm font-black text-blue-900 shadow-lg transition hover:bg-slate-100">
                                查看我的流水
                            </Link>
                        </div>
                    </div>
                </div>
            </div>
        </section>
    );
}
