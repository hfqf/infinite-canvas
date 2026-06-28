export enum WorkbenchSceneId {
    Storefront = "STOREFRONT",
    Poster = "POSTER",
    Menu = "MENU",
}

export type WorkbenchScene = {
    id: WorkbenchSceneId;
    name: string;
    description: string;
    category: string;
};

export type WorkbenchPreset = {
    id: string;
    imgUrl: string;
    title: string;
    description: string;
    styleName: string;
    colors: string[];
    baseSlogan?: string;
    defaultText: string;
    highlightCoordinates?: { x: string; y: string; w: string; h: string };
};

export type WorkbenchPromptField = {
    label: string;
    value: string;
};

export type WorkbenchReferenceSlot = {
    fieldName: string;
    label: string;
    role: string;
};

export type WorkbenchGenerationMetadata = {
    source: "workbench";
    sceneId: WorkbenchSceneId;
    sceneName: string;
    templateName?: string;
};
