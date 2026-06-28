import { Upload, X } from "lucide-react";

import type { WorkbenchReferenceSlot, WorkbenchSceneId } from "../types";
import type { ReferenceImage } from "@/types/image";

export type WorkbenchReference = ReferenceImage & { sceneId: WorkbenchSceneId; slotFieldName: string; role: string };

export function ReferenceSlotGrid({ className, slots, references, onAdd, onRemove }: { className?: string; slots: WorkbenchReferenceSlot[]; references: WorkbenchReference[]; onAdd: (slotFieldName: string, file: File) => void | Promise<void>; onRemove: (id: string) => void }) {
    return (
        <div className={`${className || ""} rounded-2xl border border-[#27272a] bg-[#18181b] p-4`}>
            <div className="mb-3 text-sm font-black">参考图</div>
            <div className="grid gap-3">
                {slots.map((slot) => {
                    const reference = references.find((item) => item.slotFieldName === slot.fieldName);
                    return (
                        <div key={slot.fieldName} className="rounded-xl border border-dashed border-[#3f3f46] bg-[#121214] p-3">
                            <div className="mb-2 flex items-center justify-between gap-3">
                                <div>
                                    <div className="text-xs font-black">{slot.label}</div>
                                    <div className="mt-1 text-[10px] leading-4 text-zinc-500">{slot.role}</div>
                                </div>
                                {reference ? (
                                    <button type="button" onClick={() => onRemove(reference.id)} className="rounded-lg p-1 text-zinc-500 hover:bg-white/10 hover:text-white" title="移除参考图">
                                        <X className="size-4" />
                                    </button>
                                ) : null}
                            </div>
                            {reference ? (
                                <img src={reference.dataUrl} alt={reference.name} className="h-28 w-full rounded-lg object-cover" />
                            ) : (
                                <label className="flex h-28 cursor-pointer flex-col items-center justify-center rounded-lg border border-[#27272a] text-xs font-bold text-zinc-500 transition hover:border-indigo-500 hover:text-indigo-300">
                                    <Upload className="mb-2 size-5" />
                                    上传{slot.label}
                                    <input type="file" accept="image/*" className="hidden" onChange={(event) => event.target.files?.[0] && void onAdd(slot.fieldName, event.target.files[0])} />
                                </label>
                            )}
                        </div>
                    );
                })}
            </div>
        </div>
    );
}
