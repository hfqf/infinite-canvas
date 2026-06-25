"use client";

import { ReloadOutlined, SearchOutlined } from "@ant-design/icons";
import { ProTable, type ProColumns } from "@ant-design/pro-components";
import { useQuery } from "@tanstack/react-query";
import { Button, Card, Form, Input, Space, Tag, Typography } from "antd";
import dayjs from "dayjs";
import { useEffect, useState } from "react";

import { fetchAdminInvitations, type InvitationRecord } from "@/services/api/invitations";
import { useUserStore } from "@/stores/use-user-store";

const defaultPageSize = 10;

export default function AdminInvitationsPage() {
    const token = useUserStore((state) => state.token);
    const clearSession = useUserStore((state) => state.clearSession);
    const [keyword, setKeyword] = useState("");
    const [keywordText, setKeywordText] = useState("");
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(defaultPageSize);
    const query = useQuery({
        queryKey: ["admin", "invitations", token, keyword, page, pageSize],
        queryFn: () => fetchAdminInvitations(token, { keyword, page, pageSize }),
        enabled: Boolean(token),
        retry: false,
    });

    useEffect(() => {
        if (!query.isError) return;
        const message = query.error instanceof Error ? query.error.message : "读取邀请记录失败";
        if (message.includes("未登录") || message.includes("权限不足") || message.includes("登录状态无效")) clearSession();
    }, [clearSession, query.error, query.isError]);

    const search = (value = keywordText) => {
        setKeyword(value.trim());
        setPage(1);
    };

    const columns: ProColumns<InvitationRecord>[] = [
        {
            title: "邀请人",
            width: 220,
            render: (_, item) => (
                <Space direction="vertical" size={0}>
                    <Typography.Text strong>{item.inviterDisplayName || item.inviterUsername}</Typography.Text>
                    <Typography.Text type="secondary" copyable>
                        {item.inviterId}
                    </Typography.Text>
                </Space>
            ),
        },
        {
            title: "被邀请用户",
            width: 220,
            render: (_, item) => (
                <Space direction="vertical" size={0}>
                    <Typography.Text strong>{item.inviteeDisplayName || item.inviteeUsername}</Typography.Text>
                    <Typography.Text type="secondary" copyable>
                        {item.inviteeId}
                    </Typography.Text>
                </Space>
            ),
        },
        {
            title: "邮箱",
            dataIndex: "inviteeEmail",
            width: 220,
            render: (_, item) => <Typography.Text type="secondary">{item.inviteeEmail || "-"}</Typography.Text>,
        },
        {
            title: "注册时间",
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
                                <Input value={keywordText} placeholder="搜索邀请人、被邀请人、邮箱或用户 ID" allowClear onChange={(event) => setKeywordText(event.target.value)} onPressEnter={() => search()} />
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
                <ProTable<InvitationRecord>
                    rowKey="inviteeId"
                    columns={columns}
                    dataSource={query.data?.items || []}
                    loading={query.isFetching}
                    search={false}
                    defaultSize="middle"
                    tableLayout="fixed"
                    cardProps={{ variant: "borderless" }}
                    headerTitle={
                        <Space>
                            <Typography.Text strong>邀请记录</Typography.Text>
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
