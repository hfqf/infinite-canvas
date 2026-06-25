"use client";

import { App, Button, Card, Form, InputNumber, Slider, Space, Typography } from "antd";
import { useEffect, useState } from "react";

import { fetchAdminSettings, saveAdminSettings, type AdminSettings } from "@/services/api/admin";
import { useConfigStore } from "@/stores/use-config-store";
import { useUserStore } from "@/stores/use-user-store";

const defaultQuality = 0.8;

export default function AdminSystemConfigPage() {
    const token = useUserStore((state) => state.token);
    const { message } = App.useApp();
    const [form] = Form.useForm<{ referenceCompressionQuality: number }>();
    const [settings, setSettings] = useState<AdminSettings | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [isSaving, setIsSaving] = useState(false);
    const quality = Form.useWatch("referenceCompressionQuality", form) ?? defaultQuality;

    const loadSettings = async () => {
        if (!token) return;
        setIsLoading(true);
        try {
            const data = normalizeSettings(await fetchAdminSettings(token));
            setSettings(data);
            form.setFieldsValue({ referenceCompressionQuality: data.public.image.referenceCompressionQuality });
        } catch (error) {
            message.error(error instanceof Error ? error.message : "读取系统配置失败");
        } finally {
            setIsLoading(false);
        }
    };

    useEffect(() => {
        void loadSettings();
    }, [token]);

    const save = async () => {
        if (!token || !settings) return;
        const values = await form.validateFields();
        const nextSettings = normalizeSettings({
            ...settings,
            public: {
                ...settings.public,
                image: {
                    ...settings.public.image,
                    referenceCompressionQuality: normalizeReferenceCompressionQuality(values.referenceCompressionQuality),
                },
            },
        });
        setIsSaving(true);
        try {
            const saved = normalizeSettings(await saveAdminSettings(token, nextSettings));
            setSettings(saved);
            useConfigStore.setState({ publicSettings: saved.public });
            form.setFieldsValue({ referenceCompressionQuality: saved.public.image.referenceCompressionQuality });
            message.success("已保存");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存系统配置失败");
        } finally {
            setIsSaving(false);
        }
    };

    return (
        <main style={{ padding: 24 }}>
            <Space direction="vertical" size={16} style={{ width: "100%" }}>
                <Card variant="borderless" loading={isLoading}>
                    <Space direction="vertical" size={18} style={{ width: "100%" }}>
                        <div>
                            <Typography.Text type="secondary" style={{ letterSpacing: 4 }}>
                                SYSTEM
                            </Typography.Text>
                            <Typography.Title level={3} style={{ margin: 0 }}>
                                图片处理配置
                            </Typography.Title>
                        </div>
                        <Form form={form} layout="vertical" initialValues={{ referenceCompressionQuality: defaultQuality }} requiredMark={false}>
                            <Form.Item label="编辑图参考图 JPEG 压缩系数" name="referenceCompressionQuality" rules={[{ required: true, message: "请输入压缩系数" }]}>
                                <Space.Compact style={{ width: "100%" }}>
                                    <Slider min={0.1} max={1} step={0.05} value={quality} onChange={(value) => form.setFieldValue("referenceCompressionQuality", value)} style={{ flex: 1, marginInline: 12 }} />
                                    <InputNumber min={0.1} max={1} step={0.05} precision={2} value={quality} onChange={(value) => form.setFieldValue("referenceCompressionQuality", normalizeReferenceCompressionQuality(value))} style={{ width: 120 }} />
                                </Space.Compact>
                            </Form.Item>
                            <Space>
                                <Button type="primary" loading={isSaving} onClick={() => void save()}>
                                    保存配置
                                </Button>
                                <Button loading={isLoading} onClick={() => void loadSettings()}>
                                    刷新
                                </Button>
                            </Space>
                        </Form>
                    </Space>
                </Card>
            </Space>
        </main>
    );
}

function normalizeSettings(settings: AdminSettings): AdminSettings {
    return {
        ...settings,
        public: {
            ...settings.public,
            image: {
                referenceCompressionQuality: normalizeReferenceCompressionQuality(settings.public.image?.referenceCompressionQuality),
            },
        },
    };
}

function normalizeReferenceCompressionQuality(value: unknown) {
    const quality = Number(value ?? defaultQuality);
    if (!Number.isFinite(quality)) return defaultQuality;
    return Math.min(1, Math.max(0.1, quality));
}
