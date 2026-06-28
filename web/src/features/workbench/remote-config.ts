import type { AiConfig } from "../../stores/use-config-store";

export function workbenchRemoteImageConfig(config: AiConfig, selectedModel?: string): AiConfig {
    return {
        ...config,
        channelMode: "remote",
        model: selectedModel?.trim() || config.imageModel || config.model,
    };
}
