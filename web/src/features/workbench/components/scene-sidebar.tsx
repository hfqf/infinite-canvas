import type { WorkbenchScene, WorkbenchSceneId } from "../types";

export function SceneSidebar({ scenes, activeSceneId, onChange }: { scenes: WorkbenchScene[]; activeSceneId: WorkbenchSceneId; onChange: (sceneId: WorkbenchSceneId) => void }) {
    return (
        <aside className="hidden w-40 shrink-0 border-r border-[#27272a] bg-[#09090b] p-3 md:block">
            <div className="mb-3 px-2 text-[10px] font-bold uppercase tracking-wider text-zinc-500">选择场景模板</div>
            <div className="space-y-2">
                {scenes.map((scene) => {
                    const active = scene.id === activeSceneId;
                    return (
                        <button key={scene.id} type="button" onClick={() => onChange(scene.id)} className={`w-full rounded-xl border p-3 text-left transition ${active ? "border-indigo-500 bg-indigo-500/15 text-white" : "border-[#27272a] bg-[#121214] text-zinc-400 hover:border-zinc-600 hover:text-white"}`}>
                            <div className="mb-2 flex size-8 items-center justify-center rounded-lg bg-white/10 text-sm font-black">{scene.name.slice(0, 1)}</div>
                            <div className="text-sm font-black">{scene.name}</div>
                            <div className="mt-1 line-clamp-2 text-[10px] leading-4 text-zinc-500">{scene.description}</div>
                        </button>
                    );
                })}
            </div>
        </aside>
    );
}
