"use client";

import { useEffect, useState, type CSSProperties } from "react";
import { QRCodeSVG } from "@rc-component/qrcode";
import { App, Button, Modal, Segmented, Tooltip } from "antd";
import { Check, Cloud, Crown, Gem, Rocket, Star, UserRound, WalletCards } from "lucide-react";

import { CreditSymbol } from "@/constant/credits";
import { cn } from "@/lib/utils";
import { createRechargeOrder, fetchRechargeOrder, type RechargeOrder } from "@/services/api/recharge";
import { useUserStore } from "@/stores/use-user-store";

type RechargeButtonProps = {
    className?: string;
    style?: CSSProperties;
};

type RechargeMemberType = "monthly" | "annual";
type RechargePlanId = "monthly-basic" | "monthly-advanced" | "monthly-premium" | "annual-standard" | "annual-basic" | "annual-advanced" | "annual-premium";

type RechargePlan = {
    id: RechargePlanId;
    memberType: RechargeMemberType;
    typeName: "月度" | "年度";
    levelName: string;
    title: string;
    audience: string;
    amountYuan: number;
    credits: number;
    period: "月" | "年";
    badge?: string;
    creditLines: string[];
    icon: typeof UserRound;
    tone: {
        text: string;
        border: string;
        glow: string;
        button: string;
    };
    features: string[];
    benefits: string[];
    service: string;
};

const commonFeatures = ["4K 图片生成", "图片去除水印", "专享提示词库", "专享工作流库", "高清画质输出"];
const testAmountFenOptions = [10, 20, 30, 40, 50];

const rechargePlans: Record<RechargeMemberType, RechargePlan[]> = {
    monthly: [
        {
            id: "monthly-basic",
            memberType: "monthly",
            typeName: "月度",
            levelName: "基础版",
            title: "个人会员基础版",
            audience: "高频创作 / 设计师",
            amountYuan: 59,
            credits: 590,
            period: "月",
            creditLines: ["590 积分"],
            icon: UserRound,
            tone: {
                text: "text-sky-300",
                border: "border-sky-400/70",
                glow: "shadow-[0_0_34px_rgba(56,189,248,0.24)]",
                button: "from-sky-500 to-blue-600",
            },
            features: commonFeatures,
            benefits: ["并发数：6", "优先级：优先"],
            service: "存储空间：不限量",
        },
        {
            id: "monthly-advanced",
            memberType: "monthly",
            typeName: "月度",
            levelName: "高级版",
            title: "个人会员高级版",
            audience: "团队提效 / 专业设计",
            amountYuan: 99,
            credits: 1100,
            period: "月",
            badge: "享 9 折优惠",
            creditLines: ["1100 积分"],
            icon: Gem,
            tone: {
                text: "text-violet-300",
                border: "border-violet-400/70",
                glow: "shadow-[0_0_36px_rgba(167,139,250,0.28)]",
                button: "from-violet-500 to-fuchsia-600",
            },
            features: commonFeatures,
            benefits: ["并发数：6", "优先级：优先"],
            service: "存储空间：不限量",
        },
        {
            id: "monthly-premium",
            memberType: "monthly",
            typeName: "月度",
            levelName: "尊享版",
            title: "个人会员尊享版",
            audience: "大型商单 / 广告合成",
            amountYuan: 199,
            credits: 2488,
            period: "月",
            badge: "享 8 折优惠",
            creditLines: ["2488 积分"],
            icon: Crown,
            tone: {
                text: "text-amber-300",
                border: "border-amber-400/70",
                glow: "shadow-[0_0_40px_rgba(251,191,36,0.22)]",
                button: "from-amber-400 to-orange-600",
            },
            features: [...commonFeatures, "nanobanana pro 多通道", "高级进阶 AI 课程"],
            benefits: ["并发数：8", "优先级：优先"],
            service: "存储空间：不限量",
        },
    ],
    annual: [
        {
            id: "annual-standard",
            memberType: "annual",
            typeName: "年度",
            levelName: "普通版",
            title: "个人会员普通版",
            audience: "个人创作 / 日常出图",
            amountYuan: 499,
            credits: 5000,
            period: "年",
            badge: "当前套餐",
            creditLines: ["全年 5000 积分", "每月到账 417 积分", "约可生成 1668 张图"],
            icon: UserRound,
            tone: {
                text: "text-sky-300",
                border: "border-sky-400/70",
                glow: "shadow-[0_0_34px_rgba(56,189,248,0.24)]",
                button: "from-sky-500 to-blue-600",
            },
            features: [...commonFeatures, "高级进阶 AI 课程"],
            benefits: ["并发数：4 路", "优先级：优先"],
            service: "不限量存储空间 / 有效期 1 年",
        },
        {
            id: "annual-basic",
            memberType: "annual",
            typeName: "年度",
            levelName: "基础版",
            title: "个人会员基础版",
            audience: "高频创作 / 设计师",
            amountYuan: 699,
            credits: 6996,
            period: "年",
            badge: "年付省 ¥9",
            creditLines: ["全年 6996 积分", "每月到账 583 积分", "约可生成 2330 张图"],
            icon: UserRound,
            tone: {
                text: "text-sky-300",
                border: "border-sky-400/70",
                glow: "shadow-[0_0_34px_rgba(56,189,248,0.24)]",
                button: "from-sky-500 to-blue-600",
            },
            features: [...commonFeatures, "高级进阶 AI 课程"],
            benefits: ["并发数：6 路", "优先级：优先"],
            service: "与普通版一致",
        },
        {
            id: "annual-advanced",
            memberType: "annual",
            typeName: "年度",
            levelName: "高级版",
            title: "个人会员高级版",
            audience: "团队提效 / 专业设计",
            amountYuan: 999,
            credits: 10020,
            period: "年",
            badge: "年付省 ¥189",
            creditLines: ["全年 10020 积分", "每月到账 835 积分", "约可生成 3340 张图"],
            icon: Gem,
            tone: {
                text: "text-violet-300",
                border: "border-violet-400/70",
                glow: "shadow-[0_0_36px_rgba(167,139,250,0.28)]",
                button: "from-violet-500 to-fuchsia-600",
            },
            features: [...commonFeatures, "高级进阶 AI 课程"],
            benefits: ["并发数：6 路", "优先级：优先"],
            service: "与普通版一致",
        },
        {
            id: "annual-premium",
            memberType: "annual",
            typeName: "年度",
            levelName: "尊享版",
            title: "个人会员尊享版",
            audience: "大型商单 / 广告合成",
            amountYuan: 1999,
            credits: 21044,
            period: "年",
            badge: "年付省 ¥389",
            creditLines: ["全年 21044 积分", "每月到账 1753 积分", "约可生成 7014 张图"],
            icon: Crown,
            tone: {
                text: "text-amber-300",
                border: "border-amber-400/70",
                glow: "shadow-[0_0_40px_rgba(251,191,36,0.22)]",
                button: "from-amber-400 to-orange-600",
            },
            features: [...commonFeatures, "高级进阶 AI 课程", "nanobanana pro 多通道可选", "优化提示词免费"],
            benefits: ["并发数：10 路", "优先级：优先"],
            service: "与普通版一致",
        },
    ],
};

export function RechargeButton({ className, style }: RechargeButtonProps) {
    const { message } = App.useApp();
    const token = useUserStore((state) => state.token);
    const user = useUserStore((state) => state.user);
    const hydrateUser = useUserStore((state) => state.hydrateUser);
    const [open, setOpen] = useState(false);
    const [memberType, setMemberType] = useState<RechargeMemberType>("monthly");
    const [selectedPlanId, setSelectedPlanId] = useState<RechargePlanId>("monthly-advanced");
    const [isTestRecharge, setIsTestRecharge] = useState(false);
    const [loading, setLoading] = useState(false);
    const [order, setOrder] = useState<RechargeOrder | null>(null);
    const plans = rechargePlans[memberType];
    const selectedPlan = plans.find((plan) => plan.id === selectedPlanId) || plans[0];
    const title = memberType === "annual" ? "年度会员套餐对比表" : "会员套餐价目表";

    useEffect(() => {
        setIsTestRecharge(new URLSearchParams(window.location.search).get("test") === "1");
    }, []);

    useEffect(() => {
        if (!open || !order || order.status !== "pending" || !token) return;
        const timer = window.setInterval(async () => {
            try {
                const next = await fetchRechargeOrder(token, order.id);
                setOrder(next);
                if (next.status === "paid") {
                    window.clearInterval(timer);
                    await hydrateUser();
                    message.success("充值成功");
                }
            } catch {
                // 保持二维码展示，下一轮继续查询。
            }
        }, 2000);
        return () => window.clearInterval(timer);
    }, [hydrateUser, message, open, order, token]);

    const submit = async () => {
        if (!user || !token) {
            message.info("请先登录");
            return;
        }
        setLoading(true);
        try {
            setOrder(await createRechargeOrder(token, selectedPlan.amountYuan));
        } catch (error) {
            message.error(error instanceof Error ? error.message : "创建充值订单失败");
        } finally {
            setLoading(false);
        }
    };

    const submitTest = async (amountFen: number) => {
        if (!user || !token) {
            message.info("请先登录");
            return;
        }
        setLoading(true);
        try {
            setOrder(await createRechargeOrder(token, { amountFen }));
        } catch (error) {
            message.error(error instanceof Error ? error.message : "创建测试订单失败");
        } finally {
            setLoading(false);
        }
    };

    return (
        <>
            <Tooltip title="充值积分" placement="bottom">
                <button
                    type="button"
                    className={cn("inline-flex size-7 shrink-0 items-center justify-center text-stone-600 transition hover:text-stone-950 dark:text-stone-300 dark:hover:text-white [&_svg]:size-4", className)}
                    style={style}
                    onClick={() => {
                        setOpen(true);
                        setOrder(null);
                    }}
                    aria-label="充值"
                    title="充值"
                >
                    <WalletCards className="size-4" />
                </button>
            </Tooltip>
            <Modal title={null} open={open} onCancel={() => setOpen(false)} footer={null} width={memberType === "annual" ? 1180 : 980} destroyOnHidden styles={{ body: { padding: 0 } }}>
                <div className="overflow-hidden rounded-lg bg-[#06101f] text-white">
                    <div className="relative border border-sky-500/25 bg-[radial-gradient(circle_at_top,rgba(56,189,248,0.18),transparent_34%),linear-gradient(135deg,#06101f_0%,#06142a_45%,#090b17_100%)] px-5 py-6 sm:px-7">
                        <div className="pointer-events-none absolute inset-x-6 top-8 hidden h-px bg-gradient-to-r from-transparent via-sky-400/55 to-transparent sm:block" />
                        <div className="relative text-center">
                            <div className="text-2xl font-semibold tracking-[0.18em] text-white sm:text-4xl">{title}</div>
                            <div className="mt-2 text-sm tracking-[0.24em] text-sky-100/60">高效创作 / 专业设计 / 商业级 AI 创作平台</div>
                            <Segmented
                                className="mt-5 bg-slate-950/70 p-1 text-sky-50"
                                value={memberType}
                                options={[
                                    { label: "月度会员", value: "monthly" },
                                    { label: "年度会员", value: "annual" },
                                ]}
                                onChange={(value) => {
                                    const nextType = value as RechargeMemberType;
                                    setMemberType(nextType);
                                    setSelectedPlanId(nextType === "annual" ? "annual-standard" : "monthly-advanced");
                                    setOrder(null);
                                }}
                            />
                        </div>
                        <div className={cn("mt-6 grid gap-3", memberType === "annual" ? "xl:grid-cols-4" : "lg:grid-cols-3")}>
                            {plans.map((plan) => (
                                <PlanCard
                                    key={plan.id}
                                    plan={plan}
                                    selected={selectedPlan.id === plan.id}
                                    onSelect={() => {
                                        setSelectedPlanId(plan.id);
                                        setOrder(null);
                                    }}
                                />
                            ))}
                        </div>
                        <div className="mt-4 grid gap-4">
                            <div className="rounded-md border border-sky-400/20 bg-black/18 px-4 py-3 text-xs leading-6 text-sky-50/68">
                                <div>温馨提示：会员购买与积分充值订单均可开具发票，开票金额仅限实际支付额度。</div>
                                <div>{memberType === "annual" ? "年度套餐自充值日起 1 年内有效，积分到期清零，套餐不退不换。" : "月度套餐自充值日起 1 个月内有效，积分到期清零，套餐不退不换。"}</div>
                            </div>
                            <div className="rounded-md border border-sky-400/25 bg-slate-950/55 p-4">
                                <div className="flex items-center justify-between text-sm text-sky-50/72">
                                    <span>当前选择</span>
                                    <span className={selectedPlan.tone.text}>
                                        {selectedPlan.typeName} · {selectedPlan.levelName}
                                    </span>
                                </div>
                                <div className="mt-2 flex items-end justify-between">
                                    <div className="flex items-center gap-1.5 text-sm text-sky-50/70">
                                        <CreditSymbol />
                                        <span>{selectedPlan.credits.toLocaleString()} 积分</span>
                                    </div>
                                    <div className="text-3xl font-semibold">
                                        <span className={selectedPlan.tone.text}>¥{selectedPlan.amountYuan}</span>
                                    </div>
                                </div>
                                <Button type="primary" block loading={loading} className={`mt-4 border-0 bg-gradient-to-r ${selectedPlan.tone.button}`} onClick={() => void submit()}>
                                    {order?.status === "pending" ? `重新创建 ¥${selectedPlan.amountYuan} 订单` : `微信支付 ¥${selectedPlan.amountYuan}`}
                                </Button>
                                {isTestRecharge ? (
                                    <div className="mt-4 rounded-md border border-amber-300/30 bg-amber-300/8 p-3">
                                        <div className="mb-2 text-xs text-amber-100/80">测试支付入口</div>
                                        <div className="grid grid-cols-5 gap-2">
                                            {testAmountFenOptions.map((amountFen) => (
                                                <button key={amountFen} type="button" disabled={loading} onClick={() => void submitTest(amountFen)} className="rounded border border-amber-300/35 bg-amber-300/12 px-2 py-1.5 text-xs font-medium text-amber-100 transition hover:bg-amber-300/20 disabled:cursor-not-allowed disabled:opacity-50">
                                                    ¥{formatAmountFen(amountFen)}
                                                </button>
                                            ))}
                                        </div>
                                    </div>
                                ) : null}
                                {order ? <OrderQrPanel order={order} fallbackPlan={selectedPlan} /> : null}
                            </div>
                        </div>
                    </div>
                </div>
            </Modal>
        </>
    );
}

function OrderQrPanel({ order, fallbackPlan }: { order: RechargeOrder; fallbackPlan: RechargePlan }) {
    const codeUrl = order.codeUrl?.trim();
    const amountLabel = formatAmountFen(order.amountFen);
    return (
        <div className="mt-4 flex flex-col items-center gap-3 rounded-md border border-sky-400/35 bg-slate-950/82 p-3 text-center shadow-[0_0_34px_rgba(56,189,248,0.16)]">
            {order.status === "paid" ? (
                <div className="text-base font-medium text-emerald-300">已充值成功</div>
            ) : (
                <>
                    <div className="shrink-0 rounded bg-white p-3">
                        {codeUrl ? <QRCodeSVG value={codeUrl} size={156} level="M" marginSize={2} bgColor="#ffffff" fgColor="#000000" title="微信支付二维码" /> : <div className="grid h-[156px] w-[156px] place-items-center text-sm text-slate-500">二维码生成中</div>}
                    </div>
                    <div className="text-sm leading-6 text-sky-50/76">
                        <div className="font-medium text-white">微信扫码支付 ¥{amountLabel}</div>
                        <div>购买 {order.productName || `${fallbackPlan.typeName}${fallbackPlan.levelName}`}，到账 {order.credits.toLocaleString()} 积分</div>
                    </div>
                </>
            )}
        </div>
    );
}

function formatAmountFen(amountFen: number) {
    if (amountFen % 100 === 0) return String(amountFen / 100);
    return (amountFen / 100).toFixed(2);
}

function PlanCard({ plan, selected, onSelect }: { plan: RechargePlan; selected: boolean; onSelect: () => void }) {
    const Icon = plan.icon;
    return (
        <button type="button" onClick={onSelect} className={cn("group flex min-h-[458px] flex-col rounded-lg border bg-slate-950/48 p-0 text-left transition duration-200 hover:-translate-y-0.5", plan.tone.border, selected ? `${plan.tone.glow} bg-slate-950/78` : "opacity-82 hover:opacity-100")}>
            <div className="flex min-h-24 items-center gap-4 border-b border-current/20 px-5 py-5">
                <div className={cn("grid size-12 shrink-0 place-items-center rounded-full border bg-white/5", plan.tone.border, plan.tone.text)}>
                    <Icon className="size-7" />
                </div>
                <div>
                    <div className="text-xl font-semibold text-white">{plan.title}</div>
                    <div className="mt-2 text-sm text-sky-50/56">{plan.audience}</div>
                </div>
            </div>
            <div className="border-b border-current/20 px-5 py-5">
                <div className="flex items-end gap-2">
                    <span className={cn("text-lg font-semibold", plan.tone.text)}>¥</span>
                    <span className={cn("text-5xl font-bold leading-none", plan.tone.text)}>{plan.amountYuan}</span>
                    <span className="pb-1 text-lg text-sky-50/75">/ {plan.period}</span>
                </div>
                {plan.badge ? <div className={cn("mt-3 inline-flex rounded-full border px-3 py-1 text-xs", plan.tone.border, plan.tone.text)}>{plan.badge}</div> : <div className="mt-3 h-7" />}
            </div>
            <div className="border-b border-current/20 px-5 py-4">
                {plan.creditLines.map((line, index) => (
                    <div key={line} className={cn(index === 0 ? "text-lg font-semibold" : "mt-1 text-sm", plan.tone.text)}>
                        {line}
                    </div>
                ))}
            </div>
            <div className="flex-1 px-5 py-4">
                <div className="space-y-2.5">
                    {plan.features.map((feature) => (
                        <div key={feature} className="flex items-center gap-2 text-sm text-sky-50/78">
                            <Check className={cn("size-4 shrink-0", plan.tone.text)} />
                            <span>{feature}</span>
                        </div>
                    ))}
                </div>
            </div>
            <div className="space-y-2 border-t border-current/20 px-5 py-4 text-sm text-sky-50/72">
                {plan.benefits.map((benefit, index) => {
                    const BenefitIcon = index === 0 ? Rocket : Star;
                    return (
                        <div key={benefit} className="flex items-center gap-2">
                            <BenefitIcon className={cn("size-4 shrink-0", plan.tone.text)} />
                            <span>{benefit}</span>
                        </div>
                    );
                })}
                <div className="flex items-center gap-2">
                    <Cloud className={cn("size-4 shrink-0", plan.tone.text)} />
                    <span>{plan.service}</span>
                </div>
            </div>
        </button>
    );
}
