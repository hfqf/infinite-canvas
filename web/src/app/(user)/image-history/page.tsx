"use client";

import { ImageTaskHistory } from "@/components/image-tasks/image-task-history";
import { fetchMyImageTasks } from "@/services/api/image-tasks";
import { useUserStore } from "@/stores/use-user-store";

export default function ImageHistoryPage() {
    const user = useUserStore((state) => state.user);
    const userName = user?.email || user?.displayName || user?.username || "当前用户";
    return <ImageTaskHistory defaultUserName={userName} loadTasks={fetchMyImageTasks} />;
}
