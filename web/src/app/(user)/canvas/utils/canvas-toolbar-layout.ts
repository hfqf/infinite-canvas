export type CanvasToolbarDock = "bottom" | "right";
export type CanvasTopBarMode = "full" | "compact";

export const MOBILE_CANVAS_TOOLBAR_MAX_WIDTH = 640;
export const MOBILE_CANVAS_TOP_BAR_MAX_WIDTH = 640;

export function resolveCanvasToolbarDock(width: number): CanvasToolbarDock {
    return width <= MOBILE_CANVAS_TOOLBAR_MAX_WIDTH ? "right" : "bottom";
}

export function resolveCanvasTopBarMode(width: number): CanvasTopBarMode {
    return width <= MOBILE_CANVAS_TOP_BAR_MAX_WIDTH ? "compact" : "full";
}
