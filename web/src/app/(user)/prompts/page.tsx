"use client";

import { ArrowDown, ArrowUp, FolderPlus, Search, Trash2 } from "lucide-react";
import { type UIEvent, useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { App, Button, Empty, Input, Popconfirm, Segmented, Space, Spin, Tag } from "antd";

import { PromptCard } from "@/components/prompts/prompt-card";
import { PromptDetailDialog } from "@/components/prompts/prompt-detail-dialog";
import { SaveUserPromptButton } from "@/components/prompts/save-user-prompt-button";
import { usePromptList, useUserPromptList } from "@/components/prompts/use-prompt-list";
import { useCopyText } from "@/hooks/use-copy-text";
import { cn } from "@/lib/utils";
import { useAssetStore } from "@/stores/use-asset-store";
import { useUserStore } from "@/stores/use-user-store";
import { ALL_PROMPTS_OPTION, deleteUserPrompt, reorderUserPrompts, type Prompt, type UserPrompt, userPromptToPrompt } from "@/services/api/prompts";

type PromptLibrarySource = "system" | "mine";

export default function PromptsPage() {
    const { message } = App.useApp();
    const queryClient = useQueryClient();
    const token = useUserStore((state) => state.token);
    const [librarySource, setLibrarySource] = useState<PromptLibrarySource>("system");
    const [titleKeyword, setTitleKeyword] = useState("");
    const [selectedTags, setSelectedTags] = useState<string[]>([]);
    const [selectedCategory, setSelectedCategory] = useState(ALL_PROMPTS_OPTION);
    const [selectedPrompt, setSelectedPrompt] = useState<Prompt | null>(null);
    const addAsset = useAssetStore((state) => state.addAsset);
    const copyText = useCopyText();
    const systemList = usePromptList({ keyword: titleKeyword, tags: selectedTags, category: selectedCategory, enabled: librarySource === "system" });
    const userList = useUserPromptList({ keyword: titleKeyword, tags: selectedTags, category: selectedCategory, enabled: librarySource === "mine" });
    const activeList = librarySource === "mine" ? userList : systemList;
    const query = activeList.query;
    const promptItems = librarySource === "mine" ? userList.items.map(userPromptToPrompt) : systemList.items;
    const userPromptItems = userList.items;
    const promptTags = activeList.tags;
    const promptCategoryOptions = activeList.categories;
    const totalPrompts = activeList.total;
    const deleteMutation = useMutation({
        mutationFn: (id: string) => deleteUserPrompt(token, id),
        onSuccess: async () => {
            message.success("提示词已删除");
            await queryClient.invalidateQueries({ queryKey: ["user-prompts"] });
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "删除失败"),
    });
    const reorderMutation = useMutation({
        mutationFn: (items: Array<{ id: string; sortOrder: number }>) => reorderUserPrompts(token, items),
        onSuccess: async () => {
            await queryClient.invalidateQueries({ queryKey: ["user-prompts"] });
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "排序失败"),
    });

    useEffect(() => {
        if (query.isError) {
            message.error(query.error instanceof Error ? query.error.message : "获取提示词失败");
        }
    }, [message, query.error, query.isError]);

    const toggleTag = (tag: string) => {
        if (tag === ALL_PROMPTS_OPTION) return setSelectedTags([]);
        setSelectedTags((items) => (items.includes(tag) ? items.filter((item) => item !== tag) : [...items, tag]));
    };

    const switchLibrarySource = (value: PromptLibrarySource) => {
        setLibrarySource(value);
        setSelectedTags([]);
        setSelectedCategory(ALL_PROMPTS_OPTION);
    };

    const savePromptAsset = (item: Prompt) => {
        addAsset({ kind: "text", title: item.title, coverUrl: item.coverUrl, tags: item.tags, source: item.category, data: { content: item.prompt }, metadata: { source: "prompt-library", promptId: item.id, githubUrl: item.githubUrl } });
        message.success("已加入我的素材");
    };

    const handleListScroll = (event: UIEvent<HTMLDivElement>) => {
        const target = event.currentTarget;
        if (query.hasNextPage && !query.isFetchingNextPage && target.scrollTop + target.clientHeight >= target.scrollHeight - 160) {
            void query.fetchNextPage();
        }
    };

    const moveUserPrompt = (item: UserPrompt, direction: -1 | 1) => {
        const index = userPromptItems.findIndex((prompt) => prompt.id === item.id);
        const target = userPromptItems[index + direction];
        if (!target) return;
        void reorderMutation.mutateAsync([
            { id: item.id, sortOrder: target.sortOrder },
            { id: target.id, sortOrder: item.sortOrder },
        ]);
    };

    return (
        <div className="flex h-full flex-col overflow-hidden bg-background text-stone-800 dark:text-stone-100">
            <main
                className="min-h-0 flex-1 overflow-y-auto bg-background bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] px-6 py-8 [background-size:16px_16px] dark:bg-[radial-gradient(rgba(245,245,244,.16)_1px,transparent_1px)]"
                onScroll={handleListScroll}
            >
                <div className="pb-8">
                    <div className="mx-auto max-w-5xl text-center">
                        <h1 className="text-4xl font-semibold tracking-tight text-stone-950 dark:text-stone-100">提示词中心</h1>
                        <p className="mt-3 text-sm text-stone-500 dark:text-stone-400">共 {totalPrompts} 条提示词，按标题、标签与分类快速查找灵感。</p>
                        <div className="mt-5">
                            <Segmented
                                value={librarySource}
                                options={[
                                    { label: "系统提示词库", value: "system" },
                                    { label: "我的提示词库", value: "mine" },
                                ]}
                                onChange={(value) => switchLibrarySource(value as PromptLibrarySource)}
                            />
                        </div>
                    </div>
                    {query.isLoading ? (
                        <div className="flex h-60 items-center justify-center">
                            <Spin />
                        </div>
                    ) : null}
                    {!query.isLoading ? (
                        <>
                            <div className="mx-auto mt-8 w-full max-w-2xl">
                                <Input size="large" className="w-full" prefix={<Search className="size-4 text-stone-400" />} value={titleKeyword} placeholder="按标题查询" onChange={(event) => setTitleKeyword(event.target.value)} />
                            </div>
                            {librarySource === "mine" ? (
                                <div className="mx-auto mt-4 flex max-w-2xl justify-center">
                                    <SaveUserPromptButton prompt="" type="primary" source="prompt-center">
                                        新建提示词
                                    </SaveUserPromptButton>
                                </div>
                            ) : null}
                            <div className="mx-auto mt-6 grid max-w-6xl gap-3 text-left">
                                <div className="grid gap-2 sm:grid-cols-[56px_minmax(0,1fr)] sm:items-start">
                                    <div className="pt-2 text-xs font-medium text-stone-500 dark:text-stone-400">分类</div>
                                    <div className="flex flex-wrap gap-2">
                                        {promptCategoryOptions.map((category) => (
                                            <Tag.CheckableTag key={category} checked={selectedCategory === category} className={cn("prompt-filter-tag", selectedCategory === category && "is-active")} onChange={() => setSelectedCategory(category)}>
                                                {category}
                                            </Tag.CheckableTag>
                                        ))}
                                    </div>
                                </div>
                                <div className="grid gap-2 sm:grid-cols-[56px_minmax(0,1fr)] sm:items-start">
                                    <div className="pt-2 text-xs font-medium text-stone-500 dark:text-stone-400">标签</div>
                                    <div className="flex flex-wrap gap-2">
                                        {promptTags.map((tag) => (
                                            <Tag.CheckableTag
                                                key={tag}
                                                checked={tag === ALL_PROMPTS_OPTION ? selectedTags.length === 0 : selectedTags.includes(tag)}
                                                className={cn("prompt-filter-tag", (tag === ALL_PROMPTS_OPTION ? selectedTags.length === 0 : selectedTags.includes(tag)) && "is-active")}
                                                onChange={() => toggleTag(tag)}
                                            >
                                                {tag}
                                            </Tag.CheckableTag>
                                        ))}
                                    </div>
                                </div>
                            </div>
                        </>
                    ) : null}
                </div>

                {!query.isLoading ? (
                    <div>
                        <div className="mx-auto grid max-w-7xl gap-5 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                            {promptItems.map((item, index) => (
                                <PromptCard
                                    key={item.id}
                                    item={item}
                                    onOpen={() => setSelectedPrompt(item)}
                                    onCopy={() => copyText(item.prompt, "提示词已复制")}
                                    extraAction={
                                        librarySource === "mine" ? (
                                            <Space size={4} wrap>
                                                <SaveUserPromptButton prompt={item.prompt} title={item.title} category={item.category} tags={item.tags} editingItem={userPromptItems[index]} />
                                                <Button size="small" icon={<ArrowUp className="size-3.5" />} disabled={index === 0 || reorderMutation.isPending} onClick={() => moveUserPrompt(userPromptItems[index], -1)} />
                                                <Button size="small" icon={<ArrowDown className="size-3.5" />} disabled={index === userPromptItems.length - 1 || reorderMutation.isPending} onClick={() => moveUserPrompt(userPromptItems[index], 1)} />
                                                <Popconfirm title="删除这个提示词？" okText="删除" cancelText="取消" onConfirm={() => deleteMutation.mutate(item.id)}>
                                                    <Button danger size="small" icon={<Trash2 className="size-3.5" />} loading={deleteMutation.isPending} />
                                                </Popconfirm>
                                            </Space>
                                        ) : (
                                            <Space size={6} wrap>
                                                <SaveUserPromptButton prompt={item.prompt} title={item.title} category={item.category} tags={item.tags} source="system-prompt" />
                                                <Button size="small" icon={<FolderPlus className="size-3.5" />} onClick={() => savePromptAsset(item)}>
                                                    加入素材
                                                </Button>
                                            </Space>
                                        )
                                    }
                                />
                            ))}
                        </div>
                        {promptItems.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={librarySource === "mine" && !token ? "请先登录后使用我的提示词库" : "没有找到匹配的提示词"} className="py-16" /> : null}
                        <div className="mx-auto mt-6 max-w-7xl text-center text-xs text-stone-500 dark:text-stone-400">
                            {query.isFetchingNextPage ? "加载中..." : query.hasNextPage ? "继续向下滚动加载更多" : promptItems.length > 0 ? "已经到底了" : null}
                        </div>
                    </div>
                ) : null}
            </main>

            <PromptDetailDialog
                prompt={selectedPrompt}
                onClose={() => setSelectedPrompt(null)}
                onCopy={(prompt) => copyText(prompt, "提示词已复制")}
                onSaveAsset={savePromptAsset}
                extraAction={selectedPrompt ? <SaveUserPromptButton prompt={selectedPrompt.prompt} title={selectedPrompt.title} category={selectedPrompt.category} tags={selectedPrompt.tags} source={librarySource === "mine" ? "user-prompt" : "system-prompt"} /> : null}
            />
        </div>
    );
}
