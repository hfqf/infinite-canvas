import { Download, FolderPlus, ImagePlus } from "lucide-react";

export type WorkbenchResultImage = {
    id: string;
    dataUrl: string;
    storageKey?: string;
    width?: number;
    height?: number;
    bytes?: number;
    mimeType?: string;
};

export function ResultGallery({ results, running, sceneName, onDownload, onUseAsReference, onSaveToAssets }: { results: WorkbenchResultImage[]; running: boolean; sceneName: string; onDownload: (image: WorkbenchResultImage, index: number) => void | Promise<void>; onUseAsReference: (image: WorkbenchResultImage, index: number) => void | Promise<void>; onSaveToAssets: (image: WorkbenchResultImage, index: number) => void | Promise<void> }) {
    return (
        <main className="min-h-0 overflow-y-auto bg-[#0d0f16] p-4">
            <div className="mb-4 flex items-center justify-between">
                <div>
                    <div className="text-lg font-black">生成结果</div>
                    <div className="text-xs text-zinc-500">{sceneName} · 商业方案预览</div>
                </div>
            </div>
            {running ? (
                <div className="grid min-h-[420px] place-items-center rounded-2xl border border-[#27272a] bg-[#09090b] text-zinc-500">正在调用远程云端生图服务...</div>
            ) : results.length ? (
                <div className="grid gap-4 lg:grid-cols-2">
                    {results.map((image, index) => (
                        <div key={image.id} className="overflow-hidden rounded-2xl border border-[#27272a] bg-[#121214]">
                            <img src={image.dataUrl} alt={`${sceneName} 生成结果 ${index + 1}`} className="aspect-square w-full object-cover" />
                            <div className="grid grid-cols-3 gap-2 p-3">
                                <button type="button" onClick={() => void onDownload(image, index)} className="workbench-action-button">
                                    <Download className="size-4" />
                                    下载
                                </button>
                                <button type="button" onClick={() => void onUseAsReference(image, index)} className="workbench-action-button">
                                    <ImagePlus className="size-4" />
                                    参考
                                </button>
                                <button type="button" onClick={() => void onSaveToAssets(image, index)} className="workbench-action-button">
                                    <FolderPlus className="size-4" />
                                    素材
                                </button>
                            </div>
                        </div>
                    ))}
                </div>
            ) : (
                <div className="grid min-h-[420px] place-items-center rounded-2xl border border-[#27272a] bg-[#09090b] text-center">
                    <div>
                        <div className="text-sm font-black text-zinc-300">选择左侧场景并填写参数</div>
                        <div className="mt-1 text-xs text-zinc-600">点击生成后，结果会展示在这里，并同步保存 taskId 快照。</div>
                    </div>
                </div>
            )}
        </main>
    );
}
