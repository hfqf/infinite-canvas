export const imageResolutionOptions = [
    { value: "1k", label: "1K" },
    { value: "2k", label: "2K" },
    { value: "4k", label: "4K" },
];

export function visibleImageResolutionOptions(allow4K: boolean) {
    return allow4K ? imageResolutionOptions : imageResolutionOptions.filter((item) => item.value !== "4k");
}
