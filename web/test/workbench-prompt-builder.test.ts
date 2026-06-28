import test from "node:test";
import assert from "node:assert/strict";

import { buildWorkbenchPrompt } from "../src/features/workbench/prompt-builder.ts";
import { WorkbenchSceneId } from "../src/features/workbench/types.ts";

test("builds storefront prompt with scene fields and reference roles", () => {
    const result = buildWorkbenchPrompt({
        sceneId: WorkbenchSceneId.Storefront,
        sceneName: "门头招牌",
        caption: "AETHER COFFEE",
        subtext: "COFFEE & SPACE",
        presetStyle: "现代极简",
        presetDescription: "冷灰色金属拉丝底板，配温和暖白色背发光立体字",
        additionalPrompt: "保留现场建筑透视",
        negativePrompt: "文字乱码",
        sceneFields: [
            { label: "细节描述", value: "三开间临街店铺" },
            { label: "效果", value: "夜间效果" },
        ],
        referenceSlots: [
            { fieldName: "storefront-plan", label: "平面设计图", role: "作为招牌版式、文字排布和结构比例依据" },
            { fieldName: "storefront-site", label: "现场照片", role: "作为真实建筑外立面、透视、安装位置和环境光依据" },
        ],
    });

    assert.equal(result.metadata.source, "workbench");
    assert.equal(result.metadata.sceneId, "STOREFRONT");
    assert.match(result.prompt, /场景类型：门头招牌/);
    assert.match(result.prompt, /主标题：AETHER COFFEE/);
    assert.match(result.prompt, /风格预设：现代极简/);
    assert.match(result.prompt, /图1：平面设计图，作为招牌版式/);
    assert.match(result.prompt, /图2：现场照片，作为真实建筑外立面/);
    assert.match(result.prompt, /补充要求：保留现场建筑透视/);
    assert.match(result.prompt, /避免：文字乱码/);
    assert.match(result.prompt, /中文使用标准简体字/);
});

test("builds poster prompt with product-oriented commercial copy", () => {
    const result = buildWorkbenchPrompt({
        sceneId: WorkbenchSceneId.Poster,
        sceneName: "海报设计",
        caption: "夏日新品上市",
        subtext: "清爽低糖气泡水",
        presetStyle: "酸性设计",
        presetDescription: "瑞士酸性排版，高饱和蓝紫流动渐变",
        additionalPrompt: "",
        negativePrompt: "",
        sceneFields: [
            { label: "正文内容", value: "限时第二件半价" },
            { label: "其他要求", value: "突出产品瓶身" },
        ],
        referenceSlots: [{ fieldName: "poster-product", label: "产品图", role: "作为主体产品，必须保留其身份、外形和关键特征" }],
    });

    assert.equal(result.metadata.sceneId, "POSTER");
    assert.match(result.prompt, /场景类型：海报设计/);
    assert.match(result.prompt, /副标题：清爽低糖气泡水/);
    assert.match(result.prompt, /正文内容：限时第二件半价/);
    assert.match(result.prompt, /图1：产品图，作为主体产品/);
    assert.match(result.prompt, /高品质商业视觉效果图/);
});

test("builds menu prompt with menu dimensions and content", () => {
    const result = buildWorkbenchPrompt({
        sceneId: WorkbenchSceneId.Menu,
        sceneName: "菜单设计",
        caption: "TWILIGHT GRILL",
        subtext: "FINE DINING & WINE",
        presetStyle: "黑底高奢西餐",
        presetDescription: "黑灰色岩石肌理，双栏排版，无衬线极细金色字体",
        additionalPrompt: "保留高端西餐厅氛围",
        negativePrompt: "廉价传单感",
        sceneFields: [
            { label: "制作方式", value: "输入文字生成" },
            { label: "成品尺寸", value: "210mm x 297mm" },
            { label: "菜单内容", value: "牛排 128 / 红酒 68 / 甜品 38" },
        ],
        referenceSlots: [{ fieldName: "menu-reference", label: "参考图", role: "作为菜单版式、风格、配色和排版密度参考" }],
    });

    assert.equal(result.metadata.sceneName, "菜单设计");
    assert.equal(result.metadata.templateName, "黑底高奢西餐");
    assert.match(result.prompt, /成品尺寸：210mm x 297mm/);
    assert.match(result.prompt, /菜单内容：牛排 128/);
    assert.match(result.prompt, /图1：参考图，作为菜单版式/);
});
