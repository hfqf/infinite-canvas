"use client";

import { useEffect, useState } from "react";

import { ProgressCircle } from "@/components/progress-circle";
import { formatDuration } from "@/lib/image-utils";
import { imageWaitDetailText, imageWaitDurationText, type ImageWaitInfo } from "@/lib/image-wait-time";
import { cn } from "@/lib/utils";

const pendingMessages = ["正在创建图片", "马上就好了", "再等等", "正在整理细节"];

export function ImageGenerationPending({ className, label, compact = false, waitInfo }: { className?: string; label?: string; compact?: boolean; waitInfo?: ImageWaitInfo }) {
    const [tick, setTick] = useState(0);

    useEffect(() => {
        const timer = window.setInterval(() => setTick((value) => value + 1), 1000);
        return () => window.clearInterval(timer);
    }, []);

    const index = Math.floor(tick / 2) % pendingMessages.length;
    const progress = Math.min(99, 10 + (tick / (waitInfo?.seconds || 120)) * 89);

    return (
        <div className={cn("relative overflow-hidden bg-stone-100 dark:bg-white/10", compact ? "min-h-24" : "aspect-[4/3]", className)}>
            <div
                className="absolute inset-0 opacity-60"
                style={{
                    backgroundImage: "radial-gradient(circle, rgba(120,113,108,0.35) 1.4px, transparent 1.6px)",
                    backgroundSize: "16px 16px",
                    maskImage: "radial-gradient(ellipse at 38% 68%, black 0%, black 28%, transparent 60%)",
                }}
            />
            <div className="absolute left-4 top-4 flex items-center gap-3 text-[15px] font-medium text-stone-500 dark:text-stone-300">
                <ProgressCircle progress={progress} size={38} stroke={3} trackColor="rgba(120, 113, 108, 0.25)" className="text-stone-700 dark:text-stone-100" />
                <span>{label || pendingMessages[index]}</span>
            </div>
            {waitInfo ? (
                <div className="absolute inset-x-4 top-16 rounded-md bg-white/70 px-3 py-2 text-xs leading-5 text-stone-500 shadow-sm backdrop-blur dark:bg-stone-950/45 dark:text-stone-300">
                    <div className="font-medium text-stone-700 dark:text-stone-100">预计 {imageWaitDurationText(waitInfo.seconds)} 左右</div>
                    <div>{imageWaitDetailText(waitInfo)}</div>
                </div>
            ) : null}
            <div className="absolute bottom-4 left-4 right-4">
                <div className="mb-2 flex items-center justify-between text-xs text-stone-500 dark:text-stone-400">
                    <span>{formatDuration(tick * 1000)}</span>
                    <span>{Math.floor(progress)}%</span>
                </div>
                <div className="h-1.5 rounded-full bg-stone-300/70 dark:bg-white/12">
                    <div className="h-full rounded-full bg-stone-900 dark:bg-stone-100" style={{ width: `${progress}%` }} />
                </div>
            </div>
        </div>
    );
}
