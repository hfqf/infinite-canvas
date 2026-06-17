"use client";

import { DeleteOutlined, PlusOutlined, ReloadOutlined, SaveOutlined } from "@ant-design/icons";
import { App, Button, Card, Checkbox, Col, Divider, Flex, Form, Input, InputNumber, Row, Select, Space, Switch, Tabs, Typography } from "antd";
import { useEffect, useMemo, useState } from "react";

import { fetchAdminSettings, saveAdminSettings, type AdminModelChannel, type AdminModelChannelRoute, type AdminPublicModelSpec, type AdminSettings } from "@/services/api/admin";
import { useUserStore } from "@/stores/use-user-store";

const gptImage2Specs: AdminPublicModelSpec[] = [
    { model: "gpt-image-2-0.5k", capability: "image", enabled: true, giftEligible: true, defaultCredits: 1 },
    { model: "gpt-image-2-1k", capability: "image", enabled: true, giftEligible: false, defaultCredits: 8 },
    { model: "gpt-image-2-2k", capability: "image", enabled: true, giftEligible: false, defaultCredits: 18 },
    { model: "gpt-image-2-4k", capability: "image", enabled: false, giftEligible: false, defaultCredits: 36 },
];

const emptySettings: AdminSettings = {
    public: {
        modelChannel: {
            models: [],
            defaultModel: "",
            defaultImageModel: "",
            defaultVideoModel: "",
            defaultTextModel: "",
            systemPrompt: "",
            allowCustomChannel: false,
        },
        auth: { allowRegister: true, linuxDo: { enabled: false } },
    },
    private: { channels: [], promptSync: { enabled: true, cron: "*/5 * * * *" }, auth: { linuxDo: { clientId: "", clientSecret: "" } } },
};

const emptyChannel: AdminModelChannel = { protocol: "openai", name: "", baseUrl: "", apiKey: "", routes: [], enabled: true, remark: "" };
const emptyRoute: AdminModelChannelRoute = { model: "", upstreamModel: "gpt-image-2", credits: 0, weight: 1, enabled: true };
const capabilityOptions = [
    { label: "图片", value: "image" },
    { label: "视频", value: "video" },
    { label: "文本", value: "text" },
    { label: "音频", value: "audio" },
];

export default function AdminSettingsPage() {
    const token = useUserStore((state) => state.token);
    const { message } = App.useApp();
    const [form] = Form.useForm<AdminSettings>();
    const [isLoading, setIsLoading] = useState(false);
    const [isSaving, setIsSaving] = useState(false);
    const publicModels = Form.useWatch(["public", "modelChannel", "models"], form) || [];
    const modelOptions = useMemo(() => uniqueModels(publicModels.map((item) => item.model)).map((model) => ({ label: model, value: model })), [publicModels]);

    const loadSettings = async () => {
        if (!token) return;
        setIsLoading(true);
        try {
            form.setFieldsValue(normalizeSettings(await fetchAdminSettings(token)));
        } catch (error) {
            message.error(error instanceof Error ? error.message : "读取设置失败");
        } finally {
            setIsLoading(false);
        }
    };

    useEffect(() => {
        void loadSettings();
    }, [token]);

    const saveSettings = async () => {
        if (!token) return;
        const values = normalizeSettings(form.getFieldsValue(true) as AdminSettings);
        setIsSaving(true);
        try {
            const saved = await saveAdminSettings(token, withRepairedDefaults(values));
            form.setFieldsValue(normalizeSettings(saved));
            message.success("已保存");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "保存失败");
        } finally {
            setIsSaving(false);
        }
    };

    const appendGptImage2Specs = () => {
        const current = normalizePublicModels(form.getFieldValue(["public", "modelChannel", "models"]) || []);
        const existing = new Set(current.map((item) => item.model));
        form.setFieldValue(["public", "modelChannel", "models"], [...current, ...gptImage2Specs.filter((item) => !existing.has(item.model))]);
    };

    return (
        <main style={{ padding: 24 }}>
            <Flex vertical gap={16}>
                <Card variant="borderless">
                    <Flex justify="space-between" align="center" gap={16} wrap>
                        <Typography.Title level={4} style={{ margin: 0 }}>
                            系统设置
                        </Typography.Title>
                        <Space>
                            <Button icon={<ReloadOutlined />} loading={isLoading} onClick={() => void loadSettings()}>
                                刷新
                            </Button>
                            <Button type="primary" icon={<SaveOutlined />} loading={isSaving} onClick={() => void saveSettings()}>
                                保存设置
                            </Button>
                        </Space>
                    </Flex>
                </Card>

                <Form form={form} layout="vertical" requiredMark={false} initialValues={emptySettings}>
                    <Tabs
                        items={[
                            {
                                key: "public",
                                label: "公开配置",
                                children: <PublicSettingsPanel modelOptions={modelOptions} onAppendGptImage2Specs={appendGptImage2Specs} />,
                            },
                            {
                                key: "private",
                                label: "私有配置",
                                children: <PrivateSettingsPanel modelOptions={modelOptions} />,
                            },
                        ]}
                    />
                </Form>
            </Flex>
        </main>
    );
}

function PublicSettingsPanel({ modelOptions, onAppendGptImage2Specs }: { modelOptions: Array<{ label: string; value: string }>; onAppendGptImage2Specs: () => void }) {
    return (
        <Flex vertical gap={16}>
            <Card
                title="公开模型规格"
                extra={
                    <Button size="small" icon={<PlusOutlined />} onClick={onAppendGptImage2Specs}>
                        插入 GPT Image 2 规格
                    </Button>
                }
                variant="borderless"
            >
                <Form.List name={["public", "modelChannel", "models"]}>
                    {(fields, { add, remove }) => (
                        <Flex vertical gap={12}>
                            {fields.map((field) => (
                                <Card key={field.key} size="small" variant="outlined">
                                    <Row gutter={12} align="middle">
                                        <Col xs={24} lg={6}>
                                            <Form.Item {...field} name={[field.name, "model"]} label="模型" rules={[{ required: true, message: "请输入模型" }]}>
                                                <Input placeholder="gpt-image-2-1k" />
                                            </Form.Item>
                                        </Col>
                                        <Col xs={12} lg={4}>
                                            <Form.Item {...field} name={[field.name, "capability"]} label="能力">
                                                <Select options={capabilityOptions} />
                                            </Form.Item>
                                        </Col>
                                        <Col xs={12} lg={4}>
                                            <Form.Item {...field} name={[field.name, "defaultCredits"]} label="默认价格">
                                                <InputNumber min={0} step={0.1} className="!w-full" />
                                            </Form.Item>
                                        </Col>
                                        <Col xs={12} lg={3}>
                                            <Form.Item {...field} name={[field.name, "enabled"]} label="启用" valuePropName="checked">
                                                <Switch />
                                            </Form.Item>
                                        </Col>
                                        <Col xs={12} lg={4}>
                                            <Form.Item {...field} name={[field.name, "giftEligible"]} label="赠送额度" valuePropName="checked">
                                                <Switch />
                                            </Form.Item>
                                        </Col>
                                        <Col xs={24} lg={3}>
                                            <Button danger icon={<DeleteOutlined />} onClick={() => remove(field.name)}>
                                                删除
                                            </Button>
                                        </Col>
                                    </Row>
                                </Card>
                            ))}
                            <Button icon={<PlusOutlined />} onClick={() => add({ model: "", capability: "image", enabled: true, giftEligible: false, defaultCredits: 0 })}>
                                新增公开模型
                            </Button>
                        </Flex>
                    )}
                </Form.List>
            </Card>

            <Card title="默认模型与公开开关" variant="borderless">
                <Row gutter={16}>
                    <Col xs={24} md={12} xl={6}>
                        <Form.Item name={["public", "modelChannel", "defaultImageModel"]} label="默认图片模型">
                            <Select allowClear showSearch options={modelOptions} />
                        </Form.Item>
                    </Col>
                    <Col xs={24} md={12} xl={6}>
                        <Form.Item name={["public", "modelChannel", "defaultTextModel"]} label="默认文本模型">
                            <Select allowClear showSearch options={modelOptions} />
                        </Form.Item>
                    </Col>
                    <Col xs={24} md={12} xl={6}>
                        <Form.Item name={["public", "modelChannel", "defaultVideoModel"]} label="默认视频模型">
                            <Select allowClear showSearch options={modelOptions} />
                        </Form.Item>
                    </Col>
                    <Col xs={24} md={12} xl={6}>
                        <Form.Item name={["public", "modelChannel", "defaultModel"]} label="默认文本兜底模型">
                            <Select allowClear showSearch options={modelOptions} />
                        </Form.Item>
                    </Col>
                    <Col span={24}>
                        <Form.Item name={["public", "modelChannel", "systemPrompt"]} label="系统提示词">
                            <Input.TextArea rows={3} />
                        </Form.Item>
                    </Col>
                    <Col xs={24} md={12}>
                        <Form.Item name={["public", "auth", "allowRegister"]} valuePropName="checked">
                            <Checkbox>允许账号密码注册</Checkbox>
                        </Form.Item>
                    </Col>
                    <Col xs={24} md={12}>
                        <Form.Item name={["public", "auth", "linuxDo", "enabled"]} valuePropName="checked">
                            <Checkbox>启用 Linux.do 登录</Checkbox>
                        </Form.Item>
                    </Col>
                </Row>
            </Card>
        </Flex>
    );
}

function PrivateSettingsPanel({ modelOptions }: { modelOptions: Array<{ label: string; value: string }> }) {
    return (
        <Flex vertical gap={16}>
            <Card title="模型服务商渠道" variant="borderless">
                <Form.List name={["private", "channels"]}>
                    {(fields, { add, remove }) => (
                        <Flex vertical gap={12}>
                            {fields.map((field) => (
                                <Card key={field.key} size="small" title={`渠道 ${field.name + 1}`} extra={<Button danger icon={<DeleteOutlined />} onClick={() => remove(field.name)} />}>
                                    <Row gutter={12}>
                                        <Col xs={24} md={8}>
                                            <Form.Item {...field} name={[field.name, "name"]} label="渠道名称" rules={[{ required: true, message: "请输入渠道名称" }]}>
                                                <Input />
                                            </Form.Item>
                                        </Col>
                                        <Col xs={24} md={4}>
                                            <Form.Item {...field} name={[field.name, "protocol"]} label="协议">
                                                <Select options={[{ label: "OpenAI", value: "openai" }]} />
                                            </Form.Item>
                                        </Col>
                                        <Col xs={12} md={4}>
                                            <Form.Item {...field} name={[field.name, "enabled"]} label="启用" valuePropName="checked">
                                                <Switch />
                                            </Form.Item>
                                        </Col>
                                        <Col xs={24} md={12}>
                                            <Form.Item {...field} name={[field.name, "baseUrl"]} label="接口地址" rules={[{ required: true, message: "请输入接口地址" }]}>
                                                <Input />
                                            </Form.Item>
                                        </Col>
                                        <Col xs={24} md={12}>
                                            <Form.Item {...field} name={[field.name, "apiKey"]} label="API Key">
                                                <Input.Password placeholder="留空则沿用已保存的 API Key" />
                                            </Form.Item>
                                        </Col>
                                        <Col span={24}>
                                            <Form.Item {...field} name={[field.name, "remark"]} label="备注">
                                                <Input.TextArea rows={2} />
                                            </Form.Item>
                                        </Col>
                                    </Row>
                                    <Divider>路由</Divider>
                                    <Form.List name={[field.name, "routes"]}>
                                        {(routeFields, routeOps) => (
                                            <Flex vertical gap={10}>
                                                {routeFields.map((routeField) => (
                                                    <Row key={routeField.key} gutter={10} align="middle">
                                                        <Col xs={24} lg={6}>
                                                            <Form.Item {...routeField} name={[routeField.name, "model"]} label="公开模型" rules={[{ required: true, message: "请选择公开模型" }]}>
                                                                <Select showSearch options={modelOptions} />
                                                            </Form.Item>
                                                        </Col>
                                                        <Col xs={24} lg={6}>
                                                            <Form.Item {...routeField} name={[routeField.name, "upstreamModel"]} label="上游模型">
                                                                <Input placeholder="gpt-image-2" />
                                                            </Form.Item>
                                                        </Col>
                                                        <Col xs={12} lg={4}>
                                                            <Form.Item {...routeField} name={[routeField.name, "credits"]} label="价格">
                                                                <InputNumber min={0} step={0.1} className="!w-full" />
                                                            </Form.Item>
                                                        </Col>
                                                        <Col xs={12} lg={3}>
                                                            <Form.Item {...routeField} name={[routeField.name, "weight"]} label="权重">
                                                                <InputNumber min={1} step={1} className="!w-full" />
                                                            </Form.Item>
                                                        </Col>
                                                        <Col xs={12} lg={3}>
                                                            <Form.Item {...routeField} name={[routeField.name, "enabled"]} label="启用" valuePropName="checked">
                                                                <Switch />
                                                            </Form.Item>
                                                        </Col>
                                                        <Col xs={12} lg={2}>
                                                            <Button danger icon={<DeleteOutlined />} onClick={() => routeOps.remove(routeField.name)} />
                                                        </Col>
                                                    </Row>
                                                ))}
                                                <Button icon={<PlusOutlined />} onClick={() => routeOps.add({ ...emptyRoute, model: modelOptions[0]?.value || "" })}>
                                                    新增路由
                                                </Button>
                                            </Flex>
                                        )}
                                    </Form.List>
                                </Card>
                            ))}
                            <Button icon={<PlusOutlined />} onClick={() => add(emptyChannel)}>
                                新增渠道
                            </Button>
                        </Flex>
                    )}
                </Form.List>
            </Card>

            <Card title="提示词同步与登录密钥" variant="borderless">
                <Row gutter={16}>
                    <Col xs={24} md={8}>
                        <Form.Item name={["private", "promptSync", "enabled"]} valuePropName="checked">
                            <Checkbox>启用提示词定时同步</Checkbox>
                        </Form.Item>
                    </Col>
                    <Col xs={24} md={8}>
                        <Form.Item name={["private", "promptSync", "cron"]} label="同步 Cron">
                            <Input />
                        </Form.Item>
                    </Col>
                    <Col xs={24} md={12}>
                        <Form.Item name={["private", "auth", "linuxDo", "clientId"]} label="Linux.do Client ID">
                            <Input />
                        </Form.Item>
                    </Col>
                    <Col xs={24} md={12}>
                        <Form.Item name={["private", "auth", "linuxDo", "clientSecret"]} label="Linux.do Client Secret">
                            <Input.Password placeholder="留空则沿用已保存的密钥" />
                        </Form.Item>
                    </Col>
                </Row>
            </Card>
        </Flex>
    );
}

function normalizeSettings(settings: Partial<AdminSettings> = {}): AdminSettings {
    const publicSetting = settings.public || emptySettings.public;
    const privateSetting = settings.private || emptySettings.private;
    return {
        public: {
            modelChannel: {
                ...emptySettings.public.modelChannel,
                ...(publicSetting.modelChannel || {}),
                models: normalizePublicModels(publicSetting.modelChannel?.models || []),
                allowCustomChannel: false,
            },
            auth: {
                allowRegister: publicSetting.auth?.allowRegister !== false,
                linuxDo: { enabled: publicSetting.auth?.linuxDo?.enabled === true },
            },
        },
        private: {
            channels: (privateSetting.channels || []).map(normalizeChannel),
            promptSync: {
                enabled: privateSetting.promptSync?.enabled !== false,
                cron: privateSetting.promptSync?.cron || "*/5 * * * *",
            },
            auth: {
                linuxDo: {
                    clientId: privateSetting.auth?.linuxDo?.clientId || "",
                    clientSecret: privateSetting.auth?.linuxDo?.clientSecret || "",
                },
            },
        },
    };
}

function normalizePublicModels(models: Partial<AdminPublicModelSpec>[]) {
    return models
        .map((item) => ({
            model: item.model?.trim() || "",
            capability: item.capability || "image",
            enabled: item.enabled === true,
            giftEligible: item.giftEligible === true,
            defaultCredits: Math.max(0, Number(item.defaultCredits) || 0),
        }))
        .filter((item) => item.model) as AdminPublicModelSpec[];
}

function normalizeChannel(channel: Partial<AdminModelChannel> = {}): AdminModelChannel {
    return {
        protocol: "openai",
        name: channel.name || "",
        baseUrl: channel.baseUrl || "",
        apiKey: channel.apiKey || "",
        routes: (channel.routes || []).map(normalizeRoute),
        enabled: channel.enabled !== false,
        remark: channel.remark || "",
    };
}

function normalizeRoute(route: Partial<AdminModelChannelRoute> = {}): AdminModelChannelRoute {
    return {
        model: route.model?.trim() || "",
        upstreamModel: route.upstreamModel?.trim() || "gpt-image-2",
        credits: Math.max(0, Number(route.credits) || 0),
        weight: Math.max(1, Number(route.weight) || 1),
        enabled: route.enabled !== false,
    };
}

function withRepairedDefaults(settings: AdminSettings): AdminSettings {
    const routableModels = new Set(settings.private.channels.filter((channel) => channel.enabled).flatMap((channel) => channel.routes.filter((route) => route.enabled).map((route) => route.model)));
    const models = settings.public.modelChannel.models.map((item) => ({ ...item, enabled: item.enabled && routableModels.has(item.model) }));
    const byCapability = (capability: AdminPublicModelSpec["capability"]) => models.filter((item) => item.enabled && item.capability === capability).map((item) => item.model);
    const pick = (current: string, options: string[]) => (options.includes(current) ? current : options[0] || "");
    const imageModels = byCapability("image");
    const textModels = byCapability("text");
    const videoModels = byCapability("video");
    return {
        ...settings,
        public: {
            ...settings.public,
            modelChannel: {
                ...settings.public.modelChannel,
                models,
                allowCustomChannel: false,
                defaultImageModel: pick(settings.public.modelChannel.defaultImageModel, imageModels),
                defaultTextModel: pick(settings.public.modelChannel.defaultTextModel, textModels),
                defaultVideoModel: pick(settings.public.modelChannel.defaultVideoModel, videoModels),
                defaultModel: pick(settings.public.modelChannel.defaultModel, textModels),
            },
        },
    };
}

function uniqueModels(models: string[]) {
    return Array.from(new Set(models.map((item) => item.trim()).filter(Boolean)));
}
