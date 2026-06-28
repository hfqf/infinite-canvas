"use client";

import { useState } from "react";
import { BookmarkPlus } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Button, message } from "antd";
import type { ButtonProps } from "antd";

import { saveUserPrompt, type UserPrompt, type UserPromptPayload } from "@/services/api/prompts";
import { useUserStore } from "@/stores/use-user-store";
import { SaveUserPromptDialog } from "./save-user-prompt-dialog";

export function SaveUserPromptButton({
    prompt,
    title = "",
    category = "",
    tags = [],
    source = "",
    editingItem,
    children,
    ...buttonProps
}: Omit<ButtonProps, "onClick"> & {
    prompt: string;
    title?: string;
    category?: string;
    tags?: string[];
    source?: string;
    editingItem?: UserPrompt;
}) {
    const [open, setOpen] = useState(false);
    const token = useUserStore((state) => state.token);
    const queryClient = useQueryClient();
    const mutation = useMutation({
        mutationFn: (payload: UserPromptPayload) => saveUserPrompt(token, payload),
        onSuccess: async () => {
            message.success(editingItem ? "提示词已更新" : "已保存到我的提示词");
            await queryClient.invalidateQueries({ queryKey: ["user-prompts"] });
            setOpen(false);
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "保存失败"),
    });

    const handleClick = () => {
        if (!token) {
            message.info("请先登录后保存到我的提示词");
            return;
        }
        setOpen(true);
    };

    return (
        <>
            <Button size="small" icon={<BookmarkPlus className="size-3.5" />} {...buttonProps} onClick={handleClick}>
                {children || (editingItem ? "编辑" : "保存")}
            </Button>
            <SaveUserPromptDialog
                open={open}
                initialPrompt={prompt}
                initialTitle={title}
                initialCategory={category}
                initialTags={tags}
                source={source}
                editingItem={editingItem}
                saving={mutation.isPending}
                onCancel={() => setOpen(false)}
                onSave={(payload) => mutation.mutateAsync(payload)}
            />
        </>
    );
}
