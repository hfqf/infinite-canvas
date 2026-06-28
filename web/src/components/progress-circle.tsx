"use client";

import { cn } from "@/lib/utils";

export function ProgressCircle({
    progress,
    size = 44,
    stroke = 3,
    className,
    color = "currentColor",
    trackColor = "rgba(120, 113, 108, 0.25)",
}: {
    progress: number;
    size?: number;
    stroke?: number;
    className?: string;
    color?: string;
    trackColor?: string;
}) {
    const radius = (size - stroke) / 2;
    const circumference = 2 * Math.PI * radius;
    const value = Math.max(0, Math.min(99, Math.floor(progress)));
    return (
        <div className={cn("relative grid place-items-center", className)} style={{ width: size, height: size, color }}>
            <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} className="-rotate-90">
                <circle cx={size / 2} cy={size / 2} r={radius} fill="none" stroke={trackColor} strokeWidth={stroke} />
                <circle cx={size / 2} cy={size / 2} r={radius} fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth={stroke} strokeDasharray={circumference} strokeDashoffset={circumference * (1 - value / 100)} />
            </svg>
            <span className="absolute text-[10px] font-semibold tabular-nums">{value}%</span>
        </div>
    );
}
