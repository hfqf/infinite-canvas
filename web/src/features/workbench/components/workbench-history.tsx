import type { WorkbenchGenerationSnapshot } from "../workbench-log-store";

export function WorkbenchHistory({ snapshots, onRestore }: { snapshots: WorkbenchGenerationSnapshot[]; onRestore: (snapshot: WorkbenchGenerationSnapshot) => void }) {
    return (
        <aside className="hidden min-h-0 overflow-y-auto border-l border-[#27272a] bg-[#09090b] p-4 xl:block">
            <div className="mb-3">
                <div className="text-sm font-black">工作台历史</div>
                <div className="text-[11px] text-zinc-500">按 canvas taskId 保存表单快照</div>
            </div>
            <div className="space-y-2">
                {snapshots.map((snapshot) => (
                    <button key={snapshot.taskId} type="button" onClick={() => onRestore(snapshot)} className="w-full rounded-xl border border-[#27272a] bg-[#121214] p-3 text-left transition hover:border-indigo-500">
                        <div className="truncate text-xs font-black">{snapshot.sceneName}</div>
                        <div className="mt-1 truncate text-[11px] text-zinc-500">{snapshot.templateName || snapshot.model}</div>
                        <div className="mt-2 line-clamp-2 text-[10px] leading-4 text-zinc-600">{snapshot.fields.mainText || snapshot.prompt}</div>
                    </button>
                ))}
                {!snapshots.length ? <div className="rounded-xl border border-dashed border-[#27272a] p-4 text-center text-xs text-zinc-600">暂无快照</div> : null}
            </div>
        </aside>
    );
}
