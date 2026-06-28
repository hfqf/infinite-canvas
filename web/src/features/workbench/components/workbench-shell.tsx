"use client";

import "../styles.css";

import { App } from "antd";
import { nanoid } from "nanoid";
import { useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";

import { requestCreditCost } from "@/constant/credits";
import { resolveImageUrl, uploadImage } from "@/services/image-storage";
import { useAssetStore } from "@/stores/use-asset-store";
import { useConfigStore, useEffectiveConfig } from "@/stores/use-config-store";
import { useUserStore } from "@/stores/use-user-store";
import type { ReferenceImage } from "@/types/image";

import { buildWorkbenchPrompt } from "../prompt-builder";
import { REFERENCE_SLOT_INSTRUCTIONS } from "../reference-slots";
import { WORKBENCH_PRESETS, WORKBENCH_SCENES } from "../scenes";
import { generateWorkbenchImage } from "../workbench-generation";
import { listWorkbenchGenerationSnapshots, saveWorkbenchGenerationSnapshot, type WorkbenchGenerationSnapshot } from "../workbench-log-store";
import { WorkbenchSceneId, type WorkbenchPromptField } from "../types";
import { GenerationPanel } from "./generation-panel";
import { ReferenceSlotGrid, type WorkbenchReference } from "./reference-slot-grid";
import { ResultGallery, type WorkbenchResultImage } from "./result-gallery";
import { SceneForm } from "./scene-form";
import { SceneSidebar } from "./scene-sidebar";
import { WorkbenchHistory } from "./workbench-history";

type WorkbenchFields = Record<string, string>;

const fieldDefaults: Record<WorkbenchSceneId, WorkbenchFields> = {
    [WorkbenchSceneId.Storefront]: {
        mainText: "AETHER COFFEE",
        subText: "COFFEE & SPACE",
        material: "冷灰色金属拉丝底板，暖白背发光立体字",
        light: "夜景暖光，高级商业街区氛围",
        detail: "保留真实门头比例，文字清晰，适合门店方案沟通",
    },
    [WorkbenchSceneId.Poster]: {
        mainText: "新品上市",
        subText: "NEW ARRIVAL",
        body: "突出产品主体、价格权益和活动时间",
        style: "酸性设计，高饱和蓝紫色，瑞士网格排版",
        detail: "商业广告主视觉，适合线上线下宣发",
    },
    [WorkbenchSceneId.Menu]: {
        mainText: "TWILIGHT GRILL",
        subText: "FINE DINING & WINE",
        menuItems: "招牌牛排 188 / 松露意面 98 / 今日甜品 48 / 精选红酒 68",
        style: "黑底高奢西餐，金色细字，双栏菜单排版",
        detail: "菜单文字需要清晰可读，保留菜品层级和价格信息",
    },
};

const fieldLabels: Record<string, string> = {
    mainText: "主标题",
    subText: "副标题",
    material: "材质工艺",
    light: "灯光氛围",
    body: "正文信息",
    style: "风格要求",
    menuItems: "菜单内容",
    detail: "补充说明",
};

export function WorkbenchShell() {
    const { message } = App.useApp();
    const searchParams = useSearchParams();
    const effectiveConfig = useEffectiveConfig();
    const updateConfig = useConfigStore((state) => state.updateConfig);
    const isAiConfigReady = useConfigStore((state) => state.isAiConfigReady);
    const modelCosts = useConfigStore((state) => state.publicSettings?.modelChannel.modelCosts);
    const token = useUserStore((state) => state.token);
    const user = useUserStore((state) => state.user);
    const addAsset = useAssetStore((state) => state.addAsset);
    const initialScene = parseSceneId(searchParams.get("scene"));
    const [activeSceneId, setActiveSceneId] = useState<WorkbenchSceneId>(initialScene);
    const [fieldsByScene, setFieldsByScene] = useState<Record<WorkbenchSceneId, WorkbenchFields>>(fieldDefaults);
    const [selectedPresetId, setSelectedPresetId] = useState<Record<WorkbenchSceneId, string>>(() => Object.fromEntries(WORKBENCH_SCENES.map((scene) => [scene.id, WORKBENCH_PRESETS[scene.id][0]?.id || ""])) as Record<WorkbenchSceneId, string>);
    const [references, setReferences] = useState<WorkbenchReference[]>([]);
    const [selectedModel, setSelectedModel] = useState("");
    const [results, setResults] = useState<WorkbenchResultImage[]>([]);
    const [snapshots, setSnapshots] = useState<WorkbenchGenerationSnapshot[]>([]);
    const [running, setRunning] = useState(false);

    const scene = WORKBENCH_SCENES.find((item) => item.id === activeSceneId) || WORKBENCH_SCENES[0];
    const presets = WORKBENCH_PRESETS[scene.id];
    const preset = presets.find((item) => item.id === selectedPresetId[scene.id]) || presets[0];
    const sceneFields = fieldsByScene[scene.id];
    const model = selectedModel || effectiveConfig.imageModel || effectiveConfig.model;
    const generationCount = Math.max(1, Math.min(4, Number(effectiveConfig.count) || 1));
    const generationCredits = requestCreditCost({ channelMode: "remote", modelCosts, model, count: generationCount, size: effectiveConfig.size, quality: effectiveConfig.quality, referenceCount: references.length });
    const sceneReferences = references.filter((item) => item.sceneId === scene.id);
    const canGenerate = token && model && sceneFields.mainText?.trim();

    useEffect(() => {
        setActiveSceneId(parseSceneId(searchParams.get("scene")));
    }, [searchParams]);

    useEffect(() => {
        setSelectedModel(effectiveConfig.imageModel || effectiveConfig.model);
    }, [effectiveConfig.imageModel, effectiveConfig.model]);

    useEffect(() => {
        void refreshSnapshots();
    }, []);

    const promptPreview = useMemo(() => buildPrompt().prompt, [scene.id, scene.name, sceneFields, preset, sceneReferences]);

    function updateField(key: string, value: string) {
        setFieldsByScene((current) => ({ ...current, [scene.id]: { ...current[scene.id], [key]: value } }));
    }

    async function addReference(slotFieldName: string, file: File) {
        const uploaded = await uploadImage(file);
        const slot = REFERENCE_SLOT_INSTRUCTIONS[scene.id].find((item) => item.fieldName === slotFieldName);
        setReferences((current) => [...current.filter((item) => !(item.sceneId === scene.id && item.slotFieldName === slotFieldName)), { id: nanoid(), sceneId: scene.id, slotFieldName, role: slot?.role || "", name: file.name, type: uploaded.mimeType, dataUrl: uploaded.url, storageKey: uploaded.storageKey }]);
    }

    function removeReference(id: string) {
        setReferences((current) => current.filter((item) => item.id !== id));
    }

    async function generate() {
        if (!token) {
            message.warning("请先登录后再生成");
            return;
        }
        if (!isAiConfigReady({ ...effectiveConfig, channelMode: "remote" }, model)) {
            message.warning("暂无可用远程生图模型，请先检查后台模型配置");
            return;
        }
        setRunning(true);
        const startedAt = Date.now();
        try {
            const built = buildPrompt();
            const result = await generateWorkbenchImage({ config: { ...effectiveConfig, count: String(generationCount) }, selectedModel: model, prompt: built.prompt, references: sceneReferences, metadata: built.metadata });
            const uploaded = await Promise.all(result.images.map((image) => uploadImage(image.dataUrl)));
            const nextResults = uploaded.map((image) => ({ id: nanoid(), dataUrl: image.url, storageKey: image.storageKey, width: image.width, height: image.height, bytes: image.bytes, mimeType: image.mimeType }));
            setResults(nextResults);
            const taskId = result.taskId || `workbench_${nanoid()}`;
            await saveWorkbenchGenerationSnapshot({
                id: taskId,
                taskId,
                createdAt: startedAt,
                updatedAt: Date.now(),
                sceneId: scene.id,
                sceneName: scene.name,
                templateName: preset?.styleName,
                prompt: built.prompt,
                fields: sceneFields,
                references: sceneReferences,
                model,
                quality: effectiveConfig.quality,
                size: effectiveConfig.size,
                count: String(generationCount),
                resultImageUrls: nextResults.map((item) => item.dataUrl),
                metadata: built.metadata,
            });
            await refreshSnapshots();
            message.success("图片已生成，并同步写入 canvas 生图任务与扣费流水");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "生成失败");
        } finally {
            setRunning(false);
        }
    }

    function buildPrompt() {
        return buildWorkbenchPrompt({
            sceneId: scene.id,
            sceneName: scene.name,
            caption: sceneFields.mainText || preset?.defaultText || scene.name,
            subtext: sceneFields.subText,
            presetStyle: preset?.styleName,
            presetDescription: preset?.description,
            additionalPrompt: sceneFields.detail,
            sceneFields: toPromptFields(sceneFields),
            referenceSlots: REFERENCE_SLOT_INSTRUCTIONS[scene.id].filter((slot) => sceneReferences.some((item) => item.slotFieldName === slot.fieldName)),
        });
    }

    async function restoreSnapshot(snapshot: WorkbenchGenerationSnapshot) {
        setActiveSceneId(snapshot.sceneId);
        setFieldsByScene((current) => ({ ...current, [snapshot.sceneId]: { ...current[snapshot.sceneId], ...snapshot.fields } }));
        setSelectedModel(snapshot.model);
        updateConfig("quality", snapshot.quality);
        updateConfig("size", snapshot.size);
        updateConfig("count", snapshot.count);
        setReferences(
            await Promise.all(
                snapshot.references.map(async (item) => ({
                    ...item,
                    sceneId: snapshot.sceneId,
                    slotFieldName: item.slotFieldName || item.id,
                    role: item.role || "",
                    dataUrl: await resolveImageUrl(item.storageKey, item.dataUrl || ""),
                    type: item.type || "image/png",
                })),
            ),
        );
        setResults((snapshot.resultImageUrls || []).map((url) => ({ id: nanoid(), dataUrl: url })));
    }

    async function refreshSnapshots() {
        setSnapshots(await listWorkbenchGenerationSnapshots());
    }

    async function downloadResult(image: WorkbenchResultImage, index: number) {
        const anchor = document.createElement("a");
        anchor.href = image.dataUrl;
        anchor.download = `${scene.name}-${index + 1}.png`;
        anchor.click();
    }

    async function useResultAsReference(image: WorkbenchResultImage) {
        const uploaded = await uploadImage(image.dataUrl);
        const firstSlot = REFERENCE_SLOT_INSTRUCTIONS[scene.id][0];
        if (!firstSlot) return;
        setReferences((current) => [...current, { id: nanoid(), sceneId: scene.id, slotFieldName: firstSlot.fieldName, role: firstSlot.role, name: "生成结果.png", type: uploaded.mimeType, dataUrl: uploaded.url, storageKey: uploaded.storageKey }]);
        message.success("已加入参考图");
    }

    async function saveResultToAssets(image: WorkbenchResultImage, index: number) {
        const uploaded = await uploadImage(image.dataUrl);
        addAsset({
            kind: "image",
            title: `${scene.name}生成结果 ${index + 1}`,
            coverUrl: uploaded.url,
            tags: ["工作台", scene.name],
            source: "好图秀工作台",
            data: { dataUrl: uploaded.url, storageKey: uploaded.storageKey, width: uploaded.width, height: uploaded.height, bytes: uploaded.bytes, mimeType: uploaded.mimeType },
            metadata: { source: "workbench", sceneId: scene.id, sceneName: scene.name, templateName: preset?.styleName },
        });
        message.success("已保存到我的素材");
    }

    return (
        <div className="workbench-page flex h-full min-h-0 bg-[#09090b] text-white">
            <SceneSidebar scenes={WORKBENCH_SCENES} activeSceneId={scene.id} onChange={(id) => setActiveSceneId(id)} />
            <section className="flex min-w-0 flex-1 flex-col">
                <div className="flex h-14 shrink-0 items-center justify-between border-b border-[#27272a] bg-[#09090b] px-4">
                    <div className="min-w-0">
                        <div className="truncate text-sm font-black">好图秀 4K 商业引擎</div>
                        <div className="truncate text-[11px] text-zinc-500">远程云端渠道 · 统一 canvas 账户/历史/流水</div>
                    </div>
                    <div className="text-right text-[11px] text-zinc-400">
                        {user ? (
                            <>
                                <div>余额 {user.credits} 积分</div>
                                <div>冻结 {user.frozenCredits} 积分</div>
                            </>
                        ) : (
                            <a href="/login" className="font-bold text-indigo-300">
                                登录后生成
                            </a>
                        )}
                    </div>
                </div>
                <div className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden xl:grid-cols-[420px_minmax(0,1fr)_300px]">
                    <div className="min-h-0 overflow-y-auto border-r border-[#27272a] bg-[#121214] p-4">
                        <SceneForm scene={scene} fields={sceneFields} fieldLabels={fieldLabels} presets={presets} selectedPresetId={preset?.id || ""} onFieldChange={updateField} onPresetChange={(id) => setSelectedPresetId((current) => ({ ...current, [scene.id]: id }))} />
                        <ReferenceSlotGrid className="mt-4" slots={REFERENCE_SLOT_INSTRUCTIONS[scene.id]} references={sceneReferences} onAdd={addReference} onRemove={removeReference} />
                        <GenerationPanel className="mt-4" config={effectiveConfig} model={model} modelOptions={effectiveConfig.imageModels} generationCredits={generationCredits} running={running} canGenerate={Boolean(canGenerate)} promptPreview={promptPreview} onModelChange={setSelectedModel} onConfigChange={updateConfig} onGenerate={generate} />
                    </div>
                    <ResultGallery results={results} running={running} sceneName={scene.name} onDownload={downloadResult} onUseAsReference={useResultAsReference} onSaveToAssets={saveResultToAssets} />
                    <WorkbenchHistory snapshots={snapshots} onRestore={(snapshot) => void restoreSnapshot(snapshot)} />
                </div>
            </section>
        </div>
    );
}

function parseSceneId(value: string | null): WorkbenchSceneId {
    return Object.values(WorkbenchSceneId).includes(value as WorkbenchSceneId) ? (value as WorkbenchSceneId) : WorkbenchSceneId.Storefront;
}

function toPromptFields(fields: WorkbenchFields): WorkbenchPromptField[] {
    return Object.entries(fields)
        .filter(([, value]) => value.trim())
        .map(([key, value]) => ({ label: fieldLabels[key] || key, value }));
}
