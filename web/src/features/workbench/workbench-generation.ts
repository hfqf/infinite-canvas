import type { ImageRequestMetadata, ImageRequestResult } from "../../services/api/image";
import type { AiConfig } from "../../stores/use-config-store";
import type { ReferenceImage } from "../../types/image";
import { workbenchRemoteImageConfig } from "./remote-config";

type GenerateFn = (config: AiConfig, prompt: string, metadata?: ImageRequestMetadata) => Promise<ImageRequestResult>;
type EditFn = (config: AiConfig, prompt: string, references: ReferenceImage[], mask?: ReferenceImage, metadata?: ImageRequestMetadata) => Promise<ImageRequestResult>;

type GenerateWorkbenchImageInput = {
    config: AiConfig;
    prompt: string;
    selectedModel?: string;
    references?: ReferenceImage[];
    metadata?: ImageRequestMetadata;
    generate?: GenerateFn;
    edit?: EditFn;
};

export async function generateWorkbenchImage(input: GenerateWorkbenchImageInput) {
    const config = workbenchRemoteImageConfig(input.config, input.selectedModel);
    const references = input.references || [];
    if (references.length) {
        const edit = input.edit || (await import("../../services/api/image")).requestEditWithTask;
        return edit(config, input.prompt, references, undefined, input.metadata);
    }
    const generate = input.generate || (await import("../../services/api/image")).requestGenerationWithTask;
    return generate(config, input.prompt, input.metadata);
}
