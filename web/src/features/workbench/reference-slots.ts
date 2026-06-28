import { WorkbenchSceneId, type WorkbenchReferenceSlot } from "./types";

export const REFERENCE_SLOT_INSTRUCTIONS: Record<WorkbenchSceneId, WorkbenchReferenceSlot[]> = {
    [WorkbenchSceneId.Storefront]: [
        { fieldName: "storefront-plan", label: "平面设计图", role: "作为招牌版式、文字排布和结构比例依据" },
        { fieldName: "storefront-site", label: "现场照片", role: "作为真实建筑外立面、透视、安装位置和环境光依据" },
        { fieldName: "storefront-reference", label: "参考图", role: "作为视觉风格、材质、灯光氛围和质感参考" },
    ],
    [WorkbenchSceneId.Poster]: [
        { fieldName: "poster-product", label: "产品图", role: "作为主体产品，必须保留其身份、外形和关键特征" },
        { fieldName: "poster-reference", label: "参考图", role: "作为海报风格、背景、光照、构图和配色参考" },
    ],
    [WorkbenchSceneId.Menu]: [
        { fieldName: "menu-reference", label: "参考图", role: "作为菜单版式、风格、配色和排版密度参考" },
        { fieldName: "menu-product", label: "产品图", role: "作为菜品或商品主体素材，必须保留主体身份和关键外观" },
    ],
};
