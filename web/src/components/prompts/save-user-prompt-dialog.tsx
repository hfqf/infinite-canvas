"use client";

import { useEffect } from "react";
import { Button, Form, Input, Modal, Space } from "antd";

import type { UserPrompt, UserPromptPayload } from "@/services/api/prompts";

type SaveUserPromptForm = {
    title: string;
    category: string;
    tagText: string;
    prompt: string;
};

export function SaveUserPromptDialog({
    open,
    initialPrompt,
    initialTitle = "",
    initialCategory = "",
    initialTags = [],
    source = "",
    editingItem,
    saving = false,
    onCancel,
    onSave,
}: {
    open: boolean;
    initialPrompt: string;
    initialTitle?: string;
    initialCategory?: string;
    initialTags?: string[];
    source?: string;
    editingItem?: UserPrompt;
    saving?: boolean;
    onCancel: () => void;
    onSave: (payload: UserPromptPayload) => Promise<void> | void;
}) {
    const [form] = Form.useForm<SaveUserPromptForm>();

    useEffect(() => {
        if (!open) return;
        form.setFieldsValue({
            title: editingItem?.title || initialTitle,
            category: editingItem?.category || initialCategory || "默认",
            tagText: (editingItem?.tags || initialTags).join("，"),
            prompt: editingItem?.prompt || initialPrompt,
        });
    }, [editingItem, form, initialCategory, initialPrompt, initialTags, initialTitle, open]);

    const handleSubmit = async () => {
        const values = await form.validateFields();
        const tags = values.tagText
            .split(/[，,\n]/)
            .map((item) => item.trim())
            .filter(Boolean);
        await onSave({
            id: editingItem?.id,
            title: values.title.trim(),
            category: values.category.trim(),
            prompt: values.prompt.trim(),
            tags,
            source: editingItem?.source || source,
            sortOrder: editingItem?.sortOrder,
        });
    };

    return (
        <Modal
            title={editingItem ? "编辑我的提示词" : "保存到我的提示词"}
            open={open}
            onCancel={onCancel}
            footer={
                <Space>
                    <Button onClick={onCancel}>取消</Button>
                    <Button type="primary" loading={saving} onClick={handleSubmit}>
                        保存
                    </Button>
                </Space>
            }
            destroyOnHidden
        >
            <Form form={form} layout="vertical" className="pt-2">
                <Form.Item name="title" label="标题">
                    <Input maxLength={80} placeholder="不填则自动取提示词前 24 个字" />
                </Form.Item>
                <Form.Item name="category" label="分类">
                    <Input maxLength={40} placeholder="默认" />
                </Form.Item>
                <Form.Item name="tagText" label="标签">
                    <Input placeholder="用逗号分隔，例如：海报，电商，常用" />
                </Form.Item>
                <Form.Item name="prompt" label="提示词" rules={[{ required: true, message: "请输入提示词" }]}>
                    <Input.TextArea autoSize={{ minRows: 6, maxRows: 12 }} />
                </Form.Item>
            </Form>
        </Modal>
    );
}
