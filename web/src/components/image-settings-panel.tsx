"use client";

import { type ReactNode, useState } from "react";
import { ConfigProvider, Switch } from "antd";

import { type CanvasTheme } from "@/lib/canvas-theme";
import type { AiConfig } from "@/stores/use-config-store";

const qualityOptions = [
    { value: "auto", label: "自动" },
    { value: "high", label: "高" },
    { value: "medium", label: "中" },
    { value: "low", label: "低" },
];
const DIMENSION_STEP = 16;

type AspectOption = { value: string; label: string; width: number; height: number; icon: string; size?: string };

const aspectOptions: AspectOption[] = [
    { value: "1:1", label: "1:1", width: 1024, height: 1024, icon: "square" },
    { value: "3:2", label: "3:2", width: 1536, height: 1024, icon: "landscape" },
    { value: "2:3", label: "2:3", width: 1024, height: 1536, icon: "portrait" },
    { value: "4:3", label: "4:3", width: 1360, height: 1024, icon: "landscape" },
    { value: "3:4", label: "3:4", width: 1024, height: 1360, icon: "portrait" },
    { value: "16:9", label: "16:9", width: 1824, height: 1024, icon: "landscape" },
    { value: "9:16", label: "9:16", width: 1024, height: 1824, icon: "portrait" },
    { value: "1:1-2k", label: "1:1(2k)", size: "2048x2048", width: 2048, height: 2048, icon: "square" },
    { value: "16:9-2k", label: "16:9(2k)", size: "2048x1152", width: 2048, height: 1152, icon: "landscape" },
    { value: "9:16-2k", label: "9:16(2k)", size: "1152x2048", width: 1152, height: 2048, icon: "portrait" },
    { value: "16:9-4k", label: "16:9(4k)", size: "3840x2160", width: 3840, height: 2160, icon: "landscape" },
    { value: "9:16-4k", label: "9:16(4k)", size: "2160x3840", width: 2160, height: 3840, icon: "portrait" },
    { value: "auto", label: "auto", width: 0, height: 0, icon: "auto" },
];
const canvasAspectOptions: AspectOption[] = [
    { value: "custom", label: "自定义", width: 0, height: 0, icon: "custom" },
    { value: "1:1", label: "1:1", width: 1, height: 1, icon: "square" },
    { value: "1:2", label: "1:2", width: 1, height: 2, icon: "portrait" },
    { value: "2:1", label: "2:1", width: 2, height: 1, icon: "landscape" },
    { value: "9:16", label: "9:16", width: 9, height: 16, icon: "portrait" },
    { value: "16:9", label: "16:9", width: 16, height: 9, icon: "landscape" },
    { value: "3:4", label: "3:4", width: 3, height: 4, icon: "portrait" },
    { value: "4:3", label: "4:3", width: 4, height: 3, icon: "landscape" },
    { value: "3:2", label: "3:2", width: 3, height: 2, icon: "landscape" },
    { value: "2:3", label: "2:3", width: 2, height: 3, icon: "portrait" },
    { value: "5:4", label: "5:4", width: 5, height: 4, icon: "landscape" },
    { value: "4:5", label: "4:5", width: 4, height: 5, icon: "portrait" },
    { value: "21:9", label: "21:9", width: 21, height: 9, icon: "landscape" },
    { value: "9:21", label: "9:21", width: 9, height: 21, icon: "portrait" },
    { value: "3:1", label: "3:1", width: 3, height: 1, icon: "landscape" },
    { value: "1:3", label: "1:3", width: 1, height: 3, icon: "portrait" },
];
const resolutionOptions = [
    { value: "1k", label: "1K" },
    { value: "2k", label: "2K" },
    { value: "4k", label: "4K" },
];

type ImageSettingsPanelProps = {
    config: AiConfig;
    onConfigChange: (key: "quality" | "size" | "count", value: string) => void;
    theme: CanvasTheme;
    showTitle?: boolean;
    className?: string;
    maxCount?: number;
    quickCount?: number;
    variant?: "default" | "canvas";
};

export function ImageSettingsPanel({ config, onConfigChange, theme, showTitle = true, className = "w-[320px] space-y-4 rounded-2xl px-1 py-0.5", maxCount = 15, quickCount = 10, variant = "default" }: ImageSettingsPanelProps) {
    const [snapDimensionToStep, setSnapDimensionToStep] = useState(true);
    const isCanvas = variant === "canvas";
    const quality = config.quality || "auto";
    const count = Math.max(1, Math.min(maxCount, Math.floor(Math.abs(Number(config.count)) || 1)));
    const activeSize = config.size || "auto";
    const selectedAspect = findAspectOption(activeSize, isCanvas);
    const dimensions = readSizeDimensions(activeSize, selectedAspect || aspectOptions[0]);
    const activeResolution = readResolution(activeSize, selectedAspect, dimensions);
    const displayedAspectOptions = isCanvas ? canvasAspectOptions : aspectOptions;
    const selectAspect = (value: string) => {
        if (value === "custom") return;
        const option = displayedAspectOptions.find((item) => item.value === value);
        onConfigChange("size", isCanvas && option ? dimensionsForRatio(option.width, option.height, activeResolution) : option?.size || option?.value || "auto");
    };
    const selectResolution = (value: string) => {
        const ratio = selectedAspect?.value === "custom" ? dimensions : selectedAspect;
        onConfigChange("size", dimensionsForRatio(ratio?.width || dimensions.width || 1, ratio?.height || dimensions.height || 1, value));
    };
    const updateDimension = (key: "width" | "height", value: number | null) => {
        const next = Math.max(1, Math.floor(value || dimensions[key] || 1024));
        const width = key === "width" ? next : dimensions.width;
        const height = key === "height" ? next : dimensions.height;
        onConfigChange("size", `${alignDimension(width, snapDimensionToStep)}x${alignDimension(height, snapDimensionToStep)}`);
    };

    return (
        <ImageSettingsTheme theme={theme}>
            <div
                className={className}
                style={{ color: theme.node.text }}
                onMouseDown={(event) => {
                    event.stopPropagation();
                    if (event.target instanceof HTMLInputElement) return;
                    if (document.activeElement instanceof HTMLInputElement && event.currentTarget.contains(document.activeElement)) document.activeElement.blur();
                }}
            >
                {showTitle ? <div className={isCanvas ? "text-[22px] font-semibold leading-7" : "text-lg font-semibold"}>图像设置</div> : null}
                <div className="space-y-2.5">
                    <SettingTitle color={theme.node.muted}>质量</SettingTitle>
                    <div className="grid grid-cols-4 gap-2.5">
                        {qualityOptions.map((item) => (
                            <OptionPill key={item.value} selected={quality === item.value} theme={theme} variant={variant} onClick={() => onConfigChange("quality", item.value)}>
                                {item.label}
                            </OptionPill>
                        ))}
                    </div>
                </div>
                <div className="space-y-2.5">
                    <div className="flex items-center justify-between gap-3">
                        <SettingTitle color={theme.node.muted}>尺寸</SettingTitle>
                        <div className="flex items-center gap-2">
                            <span className="text-xs font-medium" style={{ color: theme.node.muted }}>
                                16倍数对齐
                            </span>
                            <span title="输入完成后自动向上补成 16 的倍数" onMouseDown={(event) => event.stopPropagation()}>
                                <Switch size="small" checked={snapDimensionToStep} onChange={setSnapDimensionToStep} />
                            </span>
                        </div>
                    </div>
                    <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-2.5">
                        <DimensionInput prefix="W" value={dimensions.width} disabled={activeSize === "auto"} theme={theme} alignToStep={snapDimensionToStep} variant={variant} onChange={(value) => updateDimension("width", value)} />
                        <span className="text-lg opacity-45">↔</span>
                        <DimensionInput prefix="H" value={dimensions.height} disabled={activeSize === "auto"} theme={theme} alignToStep={snapDimensionToStep} variant={variant} onChange={(value) => updateDimension("height", value)} />
                    </div>
                </div>
                {isCanvas ? (
                    <div className="space-y-2.5">
                        <SettingTitle color={theme.node.muted}>清晰度</SettingTitle>
                        <div className="grid grid-cols-3 overflow-hidden rounded-[14px] p-1" style={{ background: theme.node.fill }}>
                            {resolutionOptions.map((item) => (
                                <button
                                    key={item.value}
                                    type="button"
                                    className="h-11 cursor-pointer rounded-[11px] text-base font-semibold transition hover:opacity-85"
                                    style={{ background: activeResolution === item.value ? theme.node.placeholder : "transparent", color: activeResolution === item.value ? theme.node.panel : theme.node.text }}
                                    onMouseDown={(event) => event.stopPropagation()}
                                    onClick={() => selectResolution(item.value)}
                                >
                                    {item.label}
                                </button>
                            ))}
                        </div>
                    </div>
                ) : null}
                <div className="space-y-2.5">
                    <SettingTitle color={theme.node.muted}>{isCanvas ? "比例" : "宽高比"}</SettingTitle>
                    <div className="grid grid-cols-4 gap-2">
                        {displayedAspectOptions.map((item) => (
                            <button
                                key={item.value}
                                type="button"
                                className={`${isCanvas ? "h-[72px] rounded-[10px] text-[13px] font-semibold" : "h-[72px] rounded-xl text-sm"} flex cursor-pointer flex-col items-center justify-center gap-1.5 border bg-transparent transition hover:opacity-80`}
                                style={{ borderColor: selectedAspect?.value === item.value ? theme.node.placeholder : theme.node.stroke, background: selectedAspect?.value === item.value ? theme.node.stroke : "transparent", color: selectedAspect?.value === item.value ? theme.node.text : theme.node.muted }}
                                onMouseDown={(event) => event.stopPropagation()}
                                onClick={() => selectAspect(item.value)}
                            >
                                <AspectIcon type={item.icon} width={item.width} height={item.height} color={selectedAspect?.value === item.value ? theme.node.text : theme.node.muted} />
                                <span>{item.label}</span>
                            </button>
                        ))}
                    </div>
                </div>
                <div className="space-y-2.5">
                    <SettingTitle color={theme.node.muted}>生成张数</SettingTitle>
                    <div className="grid grid-cols-4 gap-2.5">
                        {Array.from({ length: quickCount }, (_, index) => index + 1).map((value) => (
                            <OptionPill key={value} selected={count === value} theme={theme} variant={variant} onClick={() => onConfigChange("count", String(value))}>
                                {value} 张
                            </OptionPill>
                        ))}
                        <CountInput value={count} max={maxCount} theme={theme} onChange={(value) => onConfigChange("count", String(value || 1))} />
                    </div>
                </div>
            </div>
        </ImageSettingsTheme>
    );
}

export function ImageSettingsTheme({ theme, children }: { theme: CanvasTheme; children: ReactNode }) {
    return (
        <ConfigProvider
            theme={{
                token: { colorBgContainer: theme.toolbar.panel, colorBgElevated: theme.toolbar.panel, colorBorder: theme.node.stroke, colorPrimary: theme.node.activeStroke, colorText: theme.node.text, colorTextLightSolid: theme.node.panel },
                components: { Button: { defaultBg: theme.toolbar.panel, defaultBorderColor: theme.node.stroke, defaultColor: theme.node.text } },
            }}
        >
            {children}
        </ConfigProvider>
    );
}

export function imageQualityLabel(value: string) {
    return ({ auto: "自动", high: "高", medium: "中", low: "低" } as Record<string, string>)[value] || value;
}

export function imageSizeLabel(size: string) {
    return [...aspectOptions, ...canvasAspectOptions].find((item) => (item.size || item.value) === size || item.value === size)?.label || size;
}

function OptionPill({ selected, theme, variant = "default", onClick, children }: { selected: boolean; theme: CanvasTheme; variant?: "default" | "canvas"; onClick: () => void; children: ReactNode }) {
    return (
        <button
            type="button"
            className={`${variant === "canvas" ? "h-12 text-base font-semibold" : "h-9 text-sm"} cursor-pointer rounded-full border px-2 transition hover:opacity-80`}
            style={{ background: "transparent", borderColor: selected ? theme.node.text : theme.node.stroke, color: theme.node.text }}
            onMouseDown={(event) => event.stopPropagation()}
            onClick={onClick}
        >
            {children}
        </button>
    );
}

function DimensionInput({ prefix, value, disabled, theme, alignToStep, variant = "default", onChange }: { prefix: string; value: number; disabled: boolean; theme: CanvasTheme; alignToStep: boolean; variant?: "default" | "canvas"; onChange: (value: number | null) => void }) {
    const commit = (input: HTMLInputElement) => {
        const next = alignDimension(Math.max(1, Math.floor(Number(input.value) || value || 1024)), alignToStep);
        input.value = String(next);
        onChange(next);
    };

    return (
        <label className={`flex ${variant === "canvas" ? "h-12 rounded-[14px] text-base font-semibold" : "h-9 rounded-xl text-sm"} overflow-hidden`} style={{ background: theme.node.fill, color: theme.node.text, opacity: disabled ? 0.55 : 1 }}>
            <span className={`${variant === "canvas" ? "w-11" : "w-9"} grid place-items-center`} style={{ color: theme.node.muted }}>
                {prefix}
            </span>
            <input
                type="number"
                min={1}
                disabled={disabled}
                className="min-w-0 flex-1 bg-transparent px-2 outline-none [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                defaultValue={value || ""}
                key={`${prefix}-${value}`}
                onBlur={(event) => commit(event.currentTarget)}
                onKeyDown={(event) => {
                    if (event.key === "Enter") event.currentTarget.blur();
                }}
                onMouseDown={(event) => event.stopPropagation()}
            />
        </label>
    );
}

function CountInput({ value, max, theme, onChange }: { value: number; max: number; theme: CanvasTheme; onChange: (value: number | null) => void }) {
    return (
        <label className="col-span-2 flex h-9 overflow-hidden rounded-full border text-sm" style={{ borderColor: theme.node.stroke, color: theme.node.text }}>
            <input
                type="number"
                min={1}
                max={max}
                className="min-w-0 flex-1 bg-transparent px-3 text-center outline-none [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                style={{ color: theme.node.text, WebkitTextFillColor: theme.node.text }}
                value={value || ""}
                onChange={(event) => onChange(Number(event.target.value) || null)}
                onMouseDown={(event) => event.stopPropagation()}
            />
        </label>
    );
}

function AspectIcon({ type, width, height, color }: { type: string; width: number; height: number; color: string }) {
    if (type === "auto") return null;
    if (type === "custom") {
        return (
            <span className="grid h-7 w-9 place-items-center">
                <span className="size-5 rounded-md border-2 border-dashed" style={{ borderColor: color }} />
            </span>
        );
    }
    const ratio = width / Math.max(1, height);
    const boxWidth = ratio >= 1 ? 24 : Math.max(10, 24 * ratio);
    const boxHeight = ratio >= 1 ? Math.max(10, 24 / ratio) : 24;
    return (
        <span className="grid h-7 w-9 place-items-center">
            <span className="border-2" style={{ width: boxWidth, height: boxHeight, borderColor: color }} />
        </span>
    );
}

function SettingTitle({ children, color }: { children: string; color: string }) {
    return (
        <div className="text-xs font-medium" style={{ color }}>
            {children}
        </div>
    );
}

function readSizeDimensions(size: string, fallback: { width: number; height: number }) {
    const match = size?.match(/^(\d+)x(\d+)$/);
    return {
        width: match ? Number(match[1]) : fallback.width,
        height: match ? Number(match[2]) : fallback.height,
    };
}

function alignDimension(value: number, enabled: boolean) {
    return enabled ? Math.ceil(value / DIMENSION_STEP) * DIMENSION_STEP : value;
}

function findAspectOption(size: string, canvas: boolean) {
    const options = canvas ? canvasAspectOptions : aspectOptions;
    const direct = options.find((item) => (item.size || item.value) === size || item.value === size);
    if (direct) return direct;
    if (!canvas) return undefined;
    const dimensions = readSizeDimensions(size, { width: 0, height: 0 });
    if (!dimensions.width || !dimensions.height) return canvasAspectOptions[0];
    const reduced = reduceRatio(dimensions.width, dimensions.height);
    return canvasAspectOptions.find((item) => item.width === reduced.width && item.height === reduced.height) || canvasAspectOptions[0];
}

function readResolution(size: string, selectedAspect: { value: string; width: number; height: number } | undefined, dimensions: { width: number; height: number }) {
    const legacyOption = aspectOptions.find((item) => item.size === size);
    if (legacyOption?.value.includes("4k")) return "4k";
    if (legacyOption?.value.includes("2k")) return "2k";
    if (size.toLowerCase().includes("4k") || Math.max(dimensions.width, dimensions.height) >= 3600) return "4k";
    if (size.toLowerCase().includes("2k")) return "2k";
    const ratio = selectedAspect?.value === "custom" ? dimensions : selectedAspect;
    if (ratio && size === dimensionsForRatio(ratio.width, ratio.height, "2k")) return "2k";
    return "1k";
}

function dimensionsForRatio(width: number, height: number, resolution: string) {
    const ratioWidth = Math.max(1, width);
    const ratioHeight = Math.max(1, height);
    if (resolution === "1k") {
        const scale = 1024 / Math.min(ratioWidth, ratioHeight);
        return `${alignDimension(Math.round(ratioWidth * scale), true)}x${alignDimension(Math.round(ratioHeight * scale), true)}`;
    }
    const maxPixels = 3840 * 3840;
    let scale = resolution === "4k" ? 3840 / Math.max(ratioWidth, ratioHeight) : Math.sqrt((2048 * 2048) / (ratioWidth * ratioHeight));
    let nextWidth = alignDimension(Math.round(ratioWidth * scale), true);
    let nextHeight = alignDimension(Math.round(ratioHeight * scale), true);
    if (nextWidth * nextHeight > maxPixels) {
        scale = Math.sqrt(maxPixels / (ratioWidth * ratioHeight));
        nextWidth = alignDimension(Math.floor((ratioWidth * scale) / DIMENSION_STEP) * DIMENSION_STEP, false);
        nextHeight = alignDimension(Math.floor((ratioHeight * scale) / DIMENSION_STEP) * DIMENSION_STEP, false);
    }
    return `${nextWidth}x${nextHeight}`;
}

function reduceRatio(width: number, height: number) {
    const divisor = gcd(width, height);
    return { width: width / divisor, height: height / divisor };
}

function gcd(a: number, b: number): number {
    return b ? gcd(b, a % b) : Math.max(1, Math.abs(a));
}
