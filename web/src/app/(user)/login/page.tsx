"use client";

import { LinkOutlined, LockOutlined, MailOutlined, SafetyCertificateOutlined, UserOutlined } from "@ant-design/icons";
import { Alert, App, Button, Form, Input, Segmented, Space } from "antd";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

import { fetchCurrentUser, requestVerificationCode } from "@/services/api/auth";
import { useConfigStore } from "@/stores/use-config-store";
import { useUserStore } from "@/stores/use-user-store";

type LoginFormValues = {
    username: string;
    password: string;
    email?: string;
    verificationCode?: string;
    inviteCode?: string;
    confirmPassword?: string;
};

// 仅放行站内相对路径，拦截开放重定向。浏览器会忽略 URL 中的 Tab/换行/回车，并把
// //host 或 /\host 解析为协议相对的跨站地址，因此先剥离控制字符，再拒绝 // 与 /\ 前缀。
function safeRedirect(value: string | null): string {
    const cleaned = (value ?? "").replace(/[\t\n\r]/g, "");
    if (!cleaned.startsWith("/") || cleaned.startsWith("//") || cleaned.startsWith("/\\")) {
        return "/";
    }
    return cleaned;
}

export default function LoginPage() {
    return (
        <Suspense fallback={null}>
            <LoginContent />
        </Suspense>
    );
}

function LoginContent() {
    const { message } = App.useApp();
    const [form] = Form.useForm<LoginFormValues>();
    const router = useRouter();
    const searchParams = useSearchParams();
    const login = useUserStore((state) => state.login);
    const register = useUserStore((state) => state.register);
    const setSession = useUserStore((state) => state.setSession);
    const isLoading = useUserStore((state) => state.isLoading);
    const linuxDoEnabled = useConfigStore((state) => state.publicSettings?.auth?.linuxDo?.enabled === true);
    const allowRegister = useConfigStore((state) => state.publicSettings?.auth?.allowRegister !== false);
    const [mode, setMode] = useState<"login" | "register">("login");
    const [codeLoading, setCodeLoading] = useState(false);
    const [codeCooldown, setCodeCooldown] = useState(0);
    const redirect = safeRedirect(searchParams.get("redirect"));
    const inviteCode = (searchParams.get("inviteCode") || searchParams.get("invite") || searchParams.get("aff") || "").trim();

    useEffect(() => {
        const token = searchParams.get("token");
        const error = searchParams.get("error");
        if (error) message.error(error);
        if (!token) return;
        void fetchCurrentUser(token).then((user) => {
            setSession(token, user);
            message.success("登录成功");
            router.replace(redirect);
            router.refresh();
        });
    }, [message, redirect, router, searchParams, setSession]);

    useEffect(() => {
        if (!allowRegister && mode === "register") setMode("login");
    }, [allowRegister, mode]);

    useEffect(() => {
        if (!inviteCode || !allowRegister) return;
        setMode("register");
        form.setFieldValue("inviteCode", inviteCode);
    }, [allowRegister, form, inviteCode]);

    useEffect(() => {
        if (codeCooldown <= 0) return;
        const timer = window.setInterval(() => setCodeCooldown((value) => Math.max(0, value - 1)), 1000);
        return () => window.clearInterval(timer);
    }, [codeCooldown]);

    const sendVerificationCode = async () => {
        try {
            await form.validateFields(["email"]);
            const email = form.getFieldValue("email") || "";
            setCodeLoading(true);
            const result = await requestVerificationCode(email, "register");
            setCodeCooldown(60);
            message.success(result.debugCode ? `验证码已生成：${result.debugCode}` : "验证码已发送，请查收邮箱");
        } catch (error) {
            if (error instanceof Error) message.error(error.message);
        } finally {
            setCodeLoading(false);
        }
    };

    const submit = async (values: LoginFormValues) => {
        try {
            if (mode === "register" && !allowRegister) {
                message.error("当前未开放注册");
                return;
            }
            if (mode === "register" && values.password !== values.confirmPassword) {
                message.error("两次输入的密码不一致");
                return;
            }
            const user =
                mode === "register"
                    ? await register({ username: values.username, password: values.password, email: values.email, verificationCode: values.verificationCode, inviteCode: values.inviteCode })
                    : await login({ username: values.username, password: values.password });
            message.success(mode === "register" ? "注册成功" : "登录成功");
            router.replace(redirect);
            router.refresh();
            if (user.role !== "admin") router.replace("/");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "登录失败");
        }
    };

    return (
        <main className="flex h-full min-h-0 items-center justify-center overflow-y-auto bg-background bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] px-6 py-10 [background-size:16px_16px] dark:bg-[radial-gradient(rgba(245,245,244,.16)_1px,transparent_1px)]">
            <section className="w-full max-w-[420px]">
                <div className="mb-7 text-center">
                    <img src="/haotushow-logo.png" alt="好图秀AI" className="mx-auto mb-4 block size-14 rounded-xl object-contain" />
                    <h1 className="text-3xl font-semibold tracking-normal text-stone-950 dark:text-stone-100">账号登录</h1>
                    <p className="mt-3 text-base leading-7 text-stone-500 dark:text-stone-400">支持账号密码和 Linux.do 登录。</p>
                </div>

                <Form<LoginFormValues> form={form} layout="vertical" size="large" requiredMark={false} onFinish={submit}>
                    <Form.Item>
                        <Segmented
                            block
                            value={mode}
                            onChange={(value) => setMode(value as "login" | "register")}
                            options={allowRegister ? [{ label: "登录", value: "login" }, { label: "注册", value: "register" }] : [{ label: "登录", value: "login" }]}
                        />
                    </Form.Item>
                    <Form.Item name="username" label={<span className="font-medium text-stone-800 dark:text-stone-200">用户名</span>} rules={[{ required: true, message: "请输入用户名" }]}>
                        <Input prefix={<UserOutlined />} autoComplete="username" />
                    </Form.Item>
                    {mode === "register" ? (
                        <>
                            <Alert className="mb-4" type="success" showIcon message="使用邀请码注册，额外赠送 10% 积分" />
                            <Form.Item
                                name="email"
                                label={<span className="font-medium text-stone-800 dark:text-stone-200">邮箱</span>}
                                rules={[
                                    { required: true, message: "请输入邮箱" },
                                    { type: "email", message: "邮箱格式不正确" },
                                ]}
                            >
                                <Input prefix={<MailOutlined />} autoComplete="email" />
                            </Form.Item>
                            <Form.Item label={<span className="font-medium text-stone-800 dark:text-stone-200">邮箱验证码</span>} required>
                                <Space.Compact block>
                                    <Form.Item name="verificationCode" noStyle rules={[{ required: true, message: "请输入邮箱验证码" }]}>
                                        <Input prefix={<SafetyCertificateOutlined />} inputMode="numeric" autoComplete="one-time-code" />
                                    </Form.Item>
                                    <Button type="default" loading={codeLoading} disabled={codeCooldown > 0} onClick={sendVerificationCode}>
                                        {codeCooldown > 0 ? `${codeCooldown}s` : "获取验证码"}
                                    </Button>
                                </Space.Compact>
                            </Form.Item>
                            <Form.Item name="inviteCode" label={<span className="font-medium text-stone-800 dark:text-stone-200">邀请码</span>}>
                                <Input prefix={<LinkOutlined />} placeholder="可选" autoComplete="off" />
                            </Form.Item>
                        </>
                    ) : null}
                    <Form.Item name="password" label={<span className="font-medium text-stone-800 dark:text-stone-200">密码</span>} rules={[{ required: true, message: "请输入密码" }]}>
                        <Input.Password prefix={<LockOutlined />} autoComplete={mode === "register" ? "new-password" : "current-password"} />
                    </Form.Item>
                    {mode === "register" ? (
                        <Form.Item name="confirmPassword" label={<span className="font-medium text-stone-800 dark:text-stone-200">确认密码</span>} rules={[{ required: true, message: "请再次输入密码" }]}>
                            <Input.Password prefix={<LockOutlined />} autoComplete="new-password" />
                        </Form.Item>
                    ) : null}
                    <Space orientation="vertical" size={12} style={{ width: "100%" }}>
                        <Button block type="primary" htmlType="submit" loading={isLoading}>
                            {mode === "register" ? "注册" : "登录"}
                        </Button>
                        {linuxDoEnabled ? (
                            <Button block href={`/api/auth/linux-do/authorize?redirect=${encodeURIComponent(redirect)}`} icon={<img src="/icons/linuxdo.svg" alt="" width={18} height={18} />}>
                                使用 Linux.do 登录
                            </Button>
                        ) : null}
                    </Space>
                </Form>
            </section>
        </main>
    );
}
