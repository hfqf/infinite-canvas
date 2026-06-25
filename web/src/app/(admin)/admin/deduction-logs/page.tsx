"use client";

import { LinkOutlined, ReloadOutlined, SearchOutlined } from "@ant-design/icons";
import { ProTable, type ProColumns } from "@ant-design/pro-components";
import { useQuery } from "@tanstack/react-query";
import { Button, Card, Form, Input, Space, Tag, Typography, Image } from "antd";
import dayjs from "dayjs";
import { useEffect, useMemo, useState } from "react";

import { fetchAdminDeductionLogs, type AdminCreditLog } from "@/services/api/admin";
import { useUserStore } from "@/stores/use-user-store";

type DeductionExtra = {
    model?: string;
    path?: string;
    prompt?: string;
    imageUrl?: string;
    taskId?: string;
    frozenCredits?: number;
};

type DeductionRow = AdminCreditLog & { parsedExtra: DeductionExtra };

const defaultPageSize = 10;

export default function AdminDeductionLogsPage() {
    const token = useUserStore((state) => state.token);
    const clearSession = useUserStore((state) => state.clearSession);
    const [keyword, setKeyword] = useState("");
    const [keywordText, setKeywordText] = useState("");
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(defaultPageSize);
    const query = useQuery({
        queryKey: ["admin", "deduction-logs", token, keyword, page, pageSize],
        queryFn: () => fetchAdminDeductionLogs(token, { keyword, page, pageSize }),
        enabled: Boolean(token),
        retry: false,
    });

    useEffect(() => {
        if (!query.isError) return;
        const message = query.error instanceof Error ? query.error.message : "读取扣款流水失败";
        if (message.includes("未登录") || message.includes("权限不足") || message.includes("登录状态无效")) clearSession();
    }, [clearSession, query.error, query.isError]);

    const rows = useMemo<DeductionRow[]>(() => (query.data?.items || []).map((item) => ({ ...item, parsedExtra: parseDeductionExtra(item.extra) })), [query.data?.items]);

    const search = (value = keywordText) => {
        setKeyword(value.trim());
        setPage(1);
    };

    const columns: ProColumns<DeductionRow>[] = [
        {
            title: "用户 ID",
            dataIndex: "userId",
            width: 210,
            render: (_, item) => <Typography.Text copyable>{item.userId}</Typography.Text>,
        },
        {
            title: "类型",
            dataIndex: "type",
            width: 100,
            render: (_, item) => <Tag color={deductionTypeColor(item.type)}>{deductionTypeLabel(item.type)}</Tag>,
        },
        {
            title: "金额",
            dataIndex: "amount",
            width: 110,
            render: (_, item) => deductionAmountCell(item),
        },
        {
            title: "可用余额",
            dataIndex: "balance",
            width: 90,
        },
        {
            title: "模型",
            width: 150,
            render: (_, item) => <Tag>{item.parsedExtra.model || "-"}</Tag>,
        },
        {
            title: "任务 ID",
            width: 220,
            render: (_, item) => <Typography.Text copyable>{item.parsedExtra.taskId || item.relatedId || "-"}</Typography.Text>,
        },
        {
            title: "提示词",
            ellipsis: true,
            render: (_, item) => <Typography.Paragraph ellipsis={{ rows: 2, tooltip: item.parsedExtra.prompt }} style={{ marginBottom: 0 }}>{item.parsedExtra.prompt || "-"}</Typography.Paragraph>,
        },
        {
            title: "图片",
            width: 120,
            render: (_, item) => imageCell(item.parsedExtra.imageUrl),
        },
        {
            title: "时间",
            dataIndex: "createdAt",
            width: 180,
            render: (_, item) => <Typography.Text type="secondary">{item.createdAt ? dayjs(item.createdAt).format("YYYY-MM-DD HH:mm:ss") : "-"}</Typography.Text>,
        },
    ];

    return (
        <main style={{ padding: 24 }}>
            <Space direction="vertical" size={16} style={{ width: "100%" }}>
                <Card variant="borderless">
                    <Form layout="vertical">
                        <Form.Item label="关键词">
                            <Space.Compact style={{ width: 520, maxWidth: "100%" }}>
                                <Input value={keywordText} placeholder="搜索用户、任务、提示词、图片链接或赠送记录" allowClear onChange={(event) => setKeywordText(event.target.value)} onPressEnter={() => search()} />
                                <Button icon={<SearchOutlined />} type="primary" onClick={() => search()}>
                                    查询
                                </Button>
                                <Button
                                    onClick={() => {
                                        setKeywordText("");
                                        setKeyword("");
                                        setPage(1);
                                    }}
                                >
                                    重置
                                </Button>
                            </Space.Compact>
                        </Form.Item>
                    </Form>
                </Card>
                <ProTable<DeductionRow>
                    rowKey="id"
                    columns={columns}
                    dataSource={rows}
                    loading={query.isFetching}
                    search={false}
                    defaultSize="middle"
                    tableLayout="fixed"
                    cardProps={{ variant: "borderless" }}
                    headerTitle={
                        <Space>
                            <Typography.Text strong>积分流水</Typography.Text>
                            <Tag>{query.data?.total || 0} 条</Tag>
                        </Space>
                    }
                    options={{ density: true, setting: true, reload: () => void query.refetch() }}
                    toolBarRender={() => [
                        <Button key="reload" icon={<ReloadOutlined />} onClick={() => void query.refetch()}>
                            刷新
                        </Button>,
                    ]}
                    pagination={{
                        current: page,
                        pageSize,
                        total: query.data?.total || 0,
                        showSizeChanger: true,
                        pageSizeOptions: [10, 20, 50, 100],
                        showTotal: (value) => `共 ${value} 条`,
                        onChange: (nextPage, nextPageSize) => {
                            if (nextPageSize !== pageSize) {
                                setPageSize(nextPageSize);
                                setPage(1);
                                return;
                            }
                            setPage(nextPage);
                        },
                    }}
                />
            </Space>
        </main>
    );
}

function deductionTypeLabel(type: string) {
    if (type === "ai_freeze") return "冻结";
    if (type === "ai_freeze_release") return "释放";
    if (type === "ai_consume") return "扣款";
    if (type === "invite_register_bonus") return "邀请赠送";
    return type || "-";
}

function deductionTypeColor(type: string) {
    if (type === "ai_freeze") return "gold";
    if (type === "ai_freeze_release") return "green";
    if (type === "ai_consume") return "red";
    if (type === "invite_register_bonus") return "cyan";
    return "default";
}

function deductionAmountCell(item: DeductionRow) {
    const frozenCredits = Number(item.parsedExtra.frozenCredits) || 0;
    if (item.type === "ai_freeze") return <Typography.Text type="warning">冻结 {frozenCredits}</Typography.Text>;
    if (item.type === "ai_freeze_release") return <Typography.Text type="success">释放 {frozenCredits}</Typography.Text>;
    if (item.amount > 0) return <Typography.Text type="success">+{item.amount}</Typography.Text>;
    return <Typography.Text type="danger">{item.amount}</Typography.Text>;
}

function parseDeductionExtra(extra: string): DeductionExtra {
    try {
        const parsed = JSON.parse(extra || "{}") as DeductionExtra;
        return parsed && typeof parsed === "object" ? parsed : {};
    } catch {
        return {};
    }
}

function imageCell(imageUrl?: string) {
    if (!imageUrl || imageUrl === "[b64_json]") return <Typography.Text type="secondary">{imageUrl || "-"}</Typography.Text>;
    return (
        <Space size={8}>
            <Image src={imageUrl} alt="扣款图片" width={42} height={42} style={{ objectFit: "cover", borderRadius: 6 }} />
            <Typography.Link href={imageUrl} target="_blank" rel="noreferrer">
                <LinkOutlined />
            </Typography.Link>
        </Space>
    );
}
