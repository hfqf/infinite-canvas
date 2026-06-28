import { ArrowRight, ImagePlus, LayoutGrid } from "lucide-react";
import Link from "next/link";

export function SiteHeader() {
    return (
        <header className="sticky top-0 z-10 border-b border-slate-200/80 bg-white/90 backdrop-blur-xl">
            <div className="mx-auto flex h-16 max-w-7xl items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
                <Link href="/site" className="flex min-w-0 items-center gap-2">
                    <img src="/haotushow-logo.png" alt="好图秀" className="size-8 rounded-lg object-contain" />
                    <span className="truncate text-base font-black tracking-tight text-slate-950">好图秀 AI Design</span>
                </Link>
                <nav className="hidden items-center gap-7 text-sm font-semibold text-slate-500 md:flex">
                    <a href="#scenes-section" className="transition hover:text-blue-600">
                        商业场景
                    </a>
                    <a href="#gallery-section" className="transition hover:text-blue-600">
                        方案画廊
                    </a>
                    <a href="#credits-pricing-section" className="transition hover:text-blue-600">
                        积分方案
                    </a>
                </nav>
                <div className="flex shrink-0 items-center gap-2">
                    <Link href="/canvas" className="hidden items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-bold text-slate-700 shadow-sm transition hover:border-slate-300 hover:bg-slate-50 sm:flex">
                        <LayoutGrid className="size-3.5" />
                        画布
                    </Link>
                    <Link href="/workbench" className="inline-flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-2 text-xs font-black text-white shadow-lg shadow-blue-600/20 transition hover:bg-blue-500">
                        <ImagePlus className="size-3.5" />
                        工作台
                        <ArrowRight className="size-3.5" />
                    </Link>
                </div>
            </div>
        </header>
    );
}
