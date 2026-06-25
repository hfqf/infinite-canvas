"use client";

import { ImageTaskHistory } from "@/components/image-tasks/image-task-history";
import { fetchAdminImageTasks, updateAdminImageTaskFeatured } from "@/services/api/image-tasks";

export default function AdminImageHistoryPage() {
    return <ImageTaskHistory eyebrow="IMAGES" title="生图历史" emptyText="暂无生图记录" defaultUserName="用户" loadTasks={fetchAdminImageTasks} onToggleFeatured={(token, task, featured) => updateAdminImageTaskFeatured(token, task.taskId || task.id, featured)} />;
}
