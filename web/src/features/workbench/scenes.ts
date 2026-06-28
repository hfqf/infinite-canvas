import { WorkbenchSceneId, type WorkbenchPreset, type WorkbenchScene } from "./types";

export const WORKBENCH_SCENES: WorkbenchScene[] = [
    {
        id: WorkbenchSceneId.Storefront,
        name: "门头招牌",
        description: "临街店铺、网红空间、外立面升级设计",
        category: "户外实体",
    },
    {
        id: WorkbenchSceneId.Poster,
        name: "海报设计",
        description: "商业广告、主视觉高精度版式编排",
        category: "创意宣发",
    },
    {
        id: WorkbenchSceneId.Menu,
        name: "菜单设计",
        description: "折页菜单、台卡点单、精致菜谱排版",
        category: "商业物料",
    },
];

export const WORKBENCH_PRESETS: Record<WorkbenchSceneId, WorkbenchPreset[]> = {
    [WorkbenchSceneId.Storefront]: [
        {
            id: "storefront-1",
            imgUrl: "https://images.unsplash.com/photo-1543007630-9710e4a00a20?auto=format&fit=crop&w=1200&q=80",
            title: "AETHER 咖啡生活馆",
            description: "冷灰色金属拉丝底板，配温和暖白色背发光立体字",
            styleName: "现代极简",
            colors: ["#FFFFFF", "#E2E8F0", "#1E293B"],
            baseSlogan: "COFFEE & SPACE",
            defaultText: "AETHER COFFEE",
            highlightCoordinates: { x: "42%", y: "36%", w: "40%", h: "8%" },
        },
        {
            id: "storefront-2",
            imgUrl: "https://images.unsplash.com/photo-1554118811-1e0d58224f24?auto=format&fit=crop&w=1200&q=80",
            title: "森之物语 面包手作",
            description: "原木质感格栅底板，配深古铜色质感金属雕刻字",
            styleName: "日式复古",
            colors: ["#D97706", "#78350F", "#FEF3C7"],
            baseSlogan: "ARTISAN BAKERY",
            defaultText: "森 の 手 作",
            highlightCoordinates: { x: "35%", y: "28%", w: "45%", h: "10%" },
        },
    ],
    [WorkbenchSceneId.Poster]: [
        {
            id: "poster-1",
            imgUrl: "https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?auto=format&fit=crop&w=1200&q=80",
            title: "第二维度 艺术大展",
            description: "瑞士酸性排版，高饱和蓝紫流动渐变与极简几何字形相撞",
            styleName: "酸性设计",
            colors: ["#C084FC", "#6366F1", "#EC4899"],
            baseSlogan: "NEW HORIZON 2026",
            defaultText: "D-2 DIMENSION",
            highlightCoordinates: { x: "15%", y: "15%", w: "70%", h: "60%" },
        },
    ],
    [WorkbenchSceneId.Menu]: [
        {
            id: "menu-1",
            imgUrl: "https://images.unsplash.com/photo-1514933651103-005eec06c04b?auto=format&fit=crop&w=1200&q=80",
            title: "暮光之下 奢享西餐",
            description: "黑灰色岩石肌理，双栏排版，无衬线极细金色字体",
            styleName: "黑底高奢西餐",
            colors: ["#F59E0B", "#1E293B", "#F3F4F6"],
            baseSlogan: "FINE DINING & WINE",
            defaultText: "TWILIGHT GRILL",
            highlightCoordinates: { x: "20%", y: "15%", w: "60%", h: "70%" },
        },
    ],
};
