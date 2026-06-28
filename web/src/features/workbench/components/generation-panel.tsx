import { Sparkles } from "lucide-react";

import type { AiConfig } from "@/stores/use-config-store";

const qualityOptions = [
    ["auto", "自动"],
    ["high", "高清"],
    ["medium", "标准"],
    ["low", "草稿"],
];
const sizeOptions = [
    ["1:1", "1:1"],
    ["16:9", "16:9"],
    ["9:16", "9:16"],
    ["3:2", "3:2"],
    ["2:3", "2:3"],
    ["4:3", "4:3"],
    ["3:4", "3:4"],
    ["4k:3840x2160", "16:9 4K"],
    ["4k:2160x3840", "9:16 4K"],
];

export function GenerationPanel({ className, config, model, modelOptions, generationCredits, running, canGenerate, promptPreview, onModelChange, onConfigChange, onGenerate }: { className?: string; config: AiConfig; model: string; modelOptions: string[]; generationCredits: number; running: boolean; canGenerate: boolean; promptPreview: string; onModelChange: (model: string) => void; onConfigChange: <K extends "quality" | "size" | "count">(key: K, value: AiConfig[K]) => void; onGenerate: () => void }) {
    return (
        <div className={`${className || ""} rounded-2xl border border-[#27272a] bg-[#18181b] p-4`}>
            <div className="mb-3 text-sm font-black">生成设置</div>
            <div className="grid gap-3">
                <label className="space-y-1.5">
                    <span className="text-[10px] font-black uppercase tracking-wider text-zinc-500">远程模型</span>
                    <select value={model} onChange={(event) => onModelChange(event.target.value)} className="workbench-input">
                        {Array.from(new Set([model, ...modelOptions].filter(Boolean))).map((item) => (
                            <option key={item} value={item}>
                                {item}
                            </option>
                        ))}
                    </select>
                </label>
                <div className="grid grid-cols-3 gap-2">
                    {qualityOptions.map(([value, label]) => (
                        <button key={value} type="button" onClick={() => onConfigChange("quality", value)} className={`rounded-lg border px-2 py-2 text-xs font-bold transition ${config.quality === value ? "border-indigo-500 bg-indigo-500/20 text-white" : "border-[#27272a] text-zinc-400 hover:border-zinc-600"}`}>
                            {label}
                        </button>
                    ))}
                </div>
                <div className="grid grid-cols-3 gap-2">
                    {sizeOptions.map(([value, label]) => (
                        <button key={value} type="button" onClick={() => onConfigChange("size", value)} className={`rounded-lg border px-2 py-2 text-xs font-bold transition ${config.size === value ? "border-indigo-500 bg-indigo-500/20 text-white" : "border-[#27272a] text-zinc-400 hover:border-zinc-600"}`}>
                            {label}
                        </button>
                    ))}
                </div>
                <label className="space-y-1.5">
                    <span className="text-[10px] font-black uppercase tracking-wider text-zinc-500">生成张数</span>
                    <input type="number" min={1} max={4} value={Math.max(1, Math.min(4, Number(config.count) || 1))} onChange={(event) => onConfigChange("count", event.target.value)} className="workbench-input" />
                </label>
                <div className="rounded-xl border border-[#27272a] bg-[#09090b] p-3">
                    <div className="mb-2 flex items-center justify-between text-xs">
                        <span className="font-bold text-zinc-400">本次渲染积分估算</span>
                        <span className="font-mono font-black text-indigo-300">-{generationCredits}</span>
                    </div>
                    <pre className="max-h-36 whitespace-pre-wrap overflow-y-auto text-[10px] leading-5 text-zinc-500">{promptPreview}</pre>
                </div>
                <button type="button" disabled={!canGenerate || running} onClick={onGenerate} className="flex h-12 items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-indigo-500 to-blue-600 text-sm font-black text-white shadow-lg shadow-indigo-500/20 transition hover:from-indigo-400 hover:to-blue-500 disabled:cursor-not-allowed disabled:opacity-45">
                    <Sparkles className="size-4" />
                    {running ? "生成中..." : "开始生成"}
                </button>
            </div>
        </div>
    );
}
