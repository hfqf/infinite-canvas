"use client";

import { useEffect, useRef, useState } from "react";
import { ArrowUp, LoaderCircle, X } from "lucide-react";
import { Button } from "antd";

import { ModelPicker } from "@/components/model-picker";
import { defaultConfig, useConfigStore, useEffectiveConfig, type AiConfig } from "@/stores/use-config-store";
import { CreditSymbol, canvasGenerationCredits } from "@/constant/credits";
import { canvasThemes } from "@/lib/canvas-theme";
import { useThemeStore } from "@/stores/use-theme-store";
import { CanvasImageSettingsPopover } from "./canvas-image-settings-popover";
import { CanvasPromptLibrary } from "./canvas-prompt-library";
import { CanvasAudioSettingsPopover, type CanvasAudioSettingKey } from "./canvas-audio-settings-popover";
import { CanvasResourceMentionTextarea } from "./canvas-resource-mention-textarea";
import { CanvasVideoSettingsPopover } from "./canvas-video-settings-popover";
import { CanvasNodeType, type CanvasGenerationMode, type CanvasNodeData } from "../types";
import { MAX_CANVAS_REFERENCE_IMAGES, type CanvasResourceReference } from "../utils/canvas-resource-references";

export type CanvasNodeGenerationMode = CanvasGenerationMode;

type CanvasNodePromptPanelProps = {
    node: CanvasNodeData;
    isRunning: boolean;
    onPromptChange: (nodeId: string, prompt: string) => void;
    onConfigChange: (nodeId: string, patch: Partial<CanvasNodeData["metadata"]>) => void;
    onGenerate: (nodeId: string, mode: CanvasNodeGenerationMode, prompt: string) => void;
    mentionReferences?: CanvasResourceReference[];
    onRemoveReference?: (nodeId: string, reference: CanvasResourceReference) => void;
    onImageSettingsOpenChange?: (open: boolean) => void;
};

export function CanvasNodePromptPanel({ node, isRunning, onPromptChange, onConfigChange, onGenerate, mentionReferences = [], onRemoveReference, onImageSettingsOpenChange }: CanvasNodePromptPanelProps) {
    const globalConfig = useEffectiveConfig();
    const modelCosts = useConfigStore((state) => state.publicSettings?.modelChannel.modelCosts);
    const openConfigDialog = useConfigStore((state) => state.openConfigDialog);
    const theme = canvasThemes[useThemeStore((state) => state.theme)];
    const mode = defaultMode(node.type);
    const config = buildNodeConfig(globalConfig, node, mode);
    const hasTextContent = node.type === CanvasNodeType.Text && Boolean(node.metadata?.content?.trim());
    const hasImageContent = node.type === CanvasNodeType.Image && Boolean(node.metadata?.content);
    const isEditingExistingContent = hasTextContent || hasImageContent;
    const initialPrompt = hasImageContent || !isEditingExistingContent ? node.metadata?.prompt || "" : "";
    const [prompt, setPrompt] = useState(initialPrompt);
    const textareaRef = useRef<HTMLTextAreaElement>(null);
    const allImageReferences = mentionReferences.filter((item) => item.kind === "image");
    const imageReferences = allImageReferences.slice(0, MAX_CANVAS_REFERENCE_IMAGES);
    const hiddenImageReferenceCount = Math.max(0, allImageReferences.length - imageReferences.length);
    const referenceCount = mode === "image" ? (allImageReferences.length ? imageReferences.length : hasImageContent ? 1 : 0) : 0;
    const credits = canvasGenerationCredits({ channelMode: config.channelMode, modelCosts, model: config.model, mode, count: config.count, size: config.size, quality: config.quality, imageReferenceCount: referenceCount });

    useEffect(() => {
        setPrompt(initialPrompt);
    }, [initialPrompt, node.id]);

    useEffect(() => {
        const textarea = textareaRef.current;
        if (!textarea) return;
        textarea.style.height = "auto";
        const minHeight = 112;
        const maxHeight = 260;
        const nextHeight = Math.min(maxHeight, Math.max(minHeight, textarea.scrollHeight));
        textarea.style.height = `${nextHeight}px`;
        textarea.style.overflowY = textarea.scrollHeight > maxHeight ? "auto" : "hidden";
    }, [prompt]);

    useEffect(() => {
        const textarea = textareaRef.current;
        if (!textarea) return;
        requestAnimationFrame(() => {
            textarea.focus();
            textarea.setSelectionRange(textarea.value.length, textarea.value.length);
        });
    }, [node.id]);

    const updatePrompt = (value: string) => {
        setPrompt(value);
        if (!hasTextContent) onPromptChange(node.id, value);
    };

    const submit = () => {
        const text = prompt.trim();
        if (!text || isRunning) return;
        onGenerate(node.id, mode, text);
        if (!hasImageContent) setPrompt("");
    };

    return (
        <div
            className="rounded-2xl border p-3 shadow-2xl backdrop-blur"
            style={{ background: theme.toolbar.panel, borderColor: theme.toolbar.border, color: theme.node.text }}
            onMouseDown={(event) => event.stopPropagation()}
            onPointerDown={(event) => event.stopPropagation()}
            onWheel={(event) => event.stopPropagation()}
        >
            {mode === "image" && imageReferences.length ? (
                <div className="mb-2">
                    <div className="thin-scrollbar flex max-w-full gap-2 overflow-x-auto pb-1">
                        {imageReferences.map((reference, index) => {
                            const canRemove = Boolean(onRemoveReference && reference.nodeId !== node.id);
                            return (
                                <div
                                    key={reference.nodeId}
                                    className="group relative h-12 w-12 shrink-0 overflow-hidden rounded-lg border"
                                    style={{ background: theme.node.fill, borderColor: theme.node.stroke }}
                                    title={reference.title}
                                >
                                    {reference.previewUrl ? <img src={reference.previewUrl} alt={reference.label} className="h-full w-full object-cover" draggable={false} /> : null}
                                    <span
                                        className="absolute left-1 top-1 flex h-5 min-w-5 items-center justify-center rounded-full px-1 text-[11px] font-semibold leading-none"
                                        style={{ background: theme.toolbar.panel, color: theme.node.text, border: `1px solid ${theme.toolbar.border}` }}
                                    >
                                        {index + 1}
                                    </span>
                                    {canRemove ? (
                                        <button
                                            type="button"
                                            className="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full opacity-90 transition hover:opacity-100"
                                            style={{ background: theme.toolbar.panel, color: theme.node.text, border: `1px solid ${theme.toolbar.border}` }}
                                            aria-label={`删除参考图${index + 1}`}
                                            onClick={(event) => {
                                                event.stopPropagation();
                                                onRemoveReference?.(node.id, reference);
                                            }}
                                        >
                                            <X className="size-3" />
                                        </button>
                                    ) : null}
                                </div>
                            );
                        })}
                        {hiddenImageReferenceCount ? (
                            <div className="flex h-12 min-w-16 shrink-0 items-center justify-center rounded-lg border px-2 text-xs" style={{ background: theme.node.fill, borderColor: theme.node.stroke, color: theme.node.muted }}>
                                +{hiddenImageReferenceCount}
                            </div>
                        ) : null}
                    </div>
                    <div className="mt-1 text-[11px]" style={{ color: theme.node.muted }}>
                        已选择 {imageReferences.length} 张参考图，最多 20 张
                    </div>
                </div>
            ) : null}
            <CanvasResourceMentionTextarea
                ref={textareaRef}
                value={prompt}
                references={mentionReferences}
                onChange={updatePrompt}
                className="thin-scrollbar min-h-28 max-h-[260px] w-full resize-none rounded-xl border px-3 py-2 text-sm leading-5 outline-none"
                style={{ background: theme.node.fill, borderColor: theme.node.stroke, color: theme.node.text, caretColor: theme.node.text }}
                placeholder={promptPlaceholder(mode, hasImageContent, hasTextContent)}
            />

            <div className="mt-2 flex min-w-0 items-center justify-between gap-2">
                <div className="flex min-w-0 items-center gap-2">
                    <CanvasPromptLibrary onSelect={updatePrompt} />
                    {mode === "image" ? (
                        <>
                            <ModelPicker config={config} value={config.model} onChange={(model) => onConfigChange(node.id, { model })} capability="image" onMissingConfig={() => openConfigDialog(true)} />
                            <CanvasImageSettingsPopover
                                config={config}
                                placement="topLeft"
                                buttonClassName="!h-10 !max-w-[170px] !justify-start !rounded-full !px-3"
                                onConfigChange={(key, value) => onConfigChange(node.id, key === "count" ? { count: Number(value) || 1 } : { [key]: value })}
                                onMissingConfig={() => openConfigDialog(true)}
                                onOpenChange={onImageSettingsOpenChange}
                            />
                        </>
                    ) : mode === "video" ? (
                        <>
                            <ModelPicker config={config} value={config.model} onChange={(model) => onConfigChange(node.id, { model })} capability="video" onMissingConfig={() => openConfigDialog(true)} />
                            <CanvasVideoSettingsPopover config={config} buttonClassName="!h-10 !max-w-[170px] !justify-start !rounded-full !px-3" onConfigChange={(key, value) => onConfigChange(node.id, videoConfigPatch(key, value))} />
                        </>
                    ) : mode === "audio" ? (
                        <>
                            <ModelPicker config={config} value={config.model} onChange={(model) => onConfigChange(node.id, { model })} capability="audio" onMissingConfig={() => openConfigDialog(true)} />
                            <CanvasAudioSettingsPopover config={config} buttonClassName="!h-10 !max-w-[170px] !justify-start !rounded-full !px-3" onConfigChange={(key, value) => onConfigChange(node.id, audioConfigPatch(key, value))} />
                        </>
                    ) : (
                        <ModelPicker config={config} value={config.model} onChange={(model) => onConfigChange(node.id, { model })} capability="text" onMissingConfig={() => openConfigDialog(true)} />
                    )}
                </div>
                <Button
                    type="primary"
                    className="!h-10 !min-w-16 shrink-0 !rounded-full !px-3"
                    disabled={isRunning || !prompt.trim()}
                    onClick={submit}
                    aria-label="生成"
                >
                    <span className="flex items-center gap-1.5">
                        <span className="inline-flex items-center gap-1 text-xs font-medium tabular-nums">
                            <CreditSymbol />
                            {credits.toLocaleString()}
                        </span>
                        {isRunning ? <LoaderCircle className="size-4 animate-spin" /> : <ArrowUp className="size-4" />}
                    </span>
                </Button>
            </div>
        </div>
    );
}

function defaultMode(type: CanvasNodeData["type"]): CanvasNodeGenerationMode {
    return type === CanvasNodeType.Text ? "text" : type === CanvasNodeType.Video ? "video" : type === CanvasNodeType.Audio ? "audio" : "image";
}

function buildNodeConfig(globalConfig: AiConfig, node: CanvasNodeData, mode: CanvasNodeGenerationMode): AiConfig {
    const defaultModel = mode === "image" ? globalConfig.imageModel : mode === "video" ? globalConfig.videoModel : mode === "audio" ? globalConfig.audioModel : globalConfig.textModel;
    return {
        ...globalConfig,
        model: node.metadata?.model || defaultModel || (mode === "audio" ? defaultConfig.audioModel : globalConfig.model || defaultConfig.model),
        quality: node.metadata?.quality || globalConfig.quality || defaultConfig.quality,
        size: node.metadata?.size || globalConfig.size || defaultConfig.size,
        videoSeconds: node.metadata?.seconds || globalConfig.videoSeconds || defaultConfig.videoSeconds,
        vquality: node.metadata?.vquality || globalConfig.vquality || defaultConfig.vquality,
        videoGenerateAudio: node.metadata?.generateAudio || globalConfig.videoGenerateAudio || defaultConfig.videoGenerateAudio,
        videoWatermark: node.metadata?.watermark || globalConfig.videoWatermark || defaultConfig.videoWatermark,
        audioVoice: node.metadata?.audioVoice || globalConfig.audioVoice || defaultConfig.audioVoice,
        audioFormat: node.metadata?.audioFormat || globalConfig.audioFormat || defaultConfig.audioFormat,
        audioSpeed: node.metadata?.audioSpeed || globalConfig.audioSpeed || defaultConfig.audioSpeed,
        audioInstructions: node.metadata?.audioInstructions || globalConfig.audioInstructions || defaultConfig.audioInstructions,
        count: String(node.metadata?.count || (mode === "image" ? globalConfig.canvasImageCount || globalConfig.count : globalConfig.count) || defaultConfig.count),
    };
}

function promptPlaceholder(mode: CanvasNodeGenerationMode, hasImageContent: boolean, hasTextContent: boolean) {
    if (mode === "video") return "描述要生成的视频内容";
    if (mode === "audio") return "描述要生成的音频内容";
    if (mode === "image") return hasImageContent ? "请输入你想要把这张图修改成什么" : "描述要生成的图片内容";
    return hasTextContent ? "请输入你想要将本段文本修改成什么" : "请输入你想要生成的文本内容";
}

function videoConfigPatch(key: keyof AiConfig, value: string) {
    if (key === "videoSeconds") return { seconds: value };
    if (key === "videoGenerateAudio") return { generateAudio: value };
    if (key === "videoWatermark") return { watermark: value };
    return { [key]: value };
}

function audioConfigPatch(key: CanvasAudioSettingKey, value: string) {
    if (key === "audioVoice") return { audioVoice: value };
    if (key === "audioFormat") return { audioFormat: value };
    if (key === "audioSpeed") return { audioSpeed: value };
    return { audioInstructions: value };
}
