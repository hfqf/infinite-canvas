import type { WorkbenchPreset, WorkbenchScene } from "../types";

export function SceneForm({ scene, fields, fieldLabels, presets, selectedPresetId, onFieldChange, onPresetChange }: { scene: WorkbenchScene; fields: Record<string, string>; fieldLabels: Record<string, string>; presets: WorkbenchPreset[]; selectedPresetId: string; onFieldChange: (key: string, value: string) => void; onPresetChange: (id: string) => void }) {
    return (
        <div className="rounded-2xl border border-[#27272a] bg-[#18181b] p-4">
            <div className="mb-4">
                <div className="text-lg font-black">{scene.name}</div>
                <div className="text-xs leading-5 text-zinc-500">{scene.description}</div>
            </div>
            <div className="mb-4 grid grid-cols-1 gap-2">
                {presets.map((preset) => (
                    <button key={preset.id} type="button" onClick={() => onPresetChange(preset.id)} className={`flex items-center gap-3 rounded-xl border p-2 text-left transition ${preset.id === selectedPresetId ? "border-indigo-500 bg-indigo-500/15" : "border-[#27272a] bg-[#121214] hover:border-zinc-600"}`}>
                        <img src={preset.imgUrl} alt={preset.title} className="size-14 rounded-lg object-cover" referrerPolicy="no-referrer" />
                        <span className="min-w-0">
                            <span className="block truncate text-sm font-black">{preset.styleName}</span>
                            <span className="line-clamp-2 text-[11px] leading-4 text-zinc-500">{preset.description}</span>
                        </span>
                    </button>
                ))}
            </div>
            <div className="space-y-3">
                {Object.entries(fields).map(([key, value]) => (
                    <label key={key} className="block space-y-1.5">
                        <span className="text-[10px] font-black uppercase tracking-wider text-zinc-500">{fieldLabels[key] || key}</span>
                        {key === "detail" || key === "menuItems" || key === "body" ? (
                            <textarea value={value} onChange={(event) => onFieldChange(key, event.target.value)} rows={4} className="workbench-input min-h-24 resize-y" />
                        ) : (
                            <input value={value} onChange={(event) => onFieldChange(key, event.target.value)} className="workbench-input" />
                        )}
                    </label>
                ))}
            </div>
        </div>
    );
}
