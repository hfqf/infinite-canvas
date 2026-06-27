type ClipboardFileItem = {
    kind?: string;
    type?: string;
    getAsFile?: () => File | null;
};

type ClipboardFileSource = {
    files?: ArrayLike<File>;
    items?: ArrayLike<ClipboardFileItem>;
};

const IMAGE_EXTENSIONS = /\.(png|jpe?g|webp|gif|bmp|svg)$/i;

export function getClipboardImageFiles(source: ClipboardFileSource): File[] {
    const files = Array.from(source.files || []).filter(isImageFile);
    if (files.length) return files;

    return Array.from(source.items || [])
        .filter((item) => item.kind === "file" && (!item.type || item.type.startsWith("image/")))
        .map((item) => item.getAsFile?.() || null)
        .filter((file): file is File => Boolean(file && isImageFile(file)));
}

function isImageFile(file: File) {
    return file.type ? file.type.startsWith("image/") : IMAGE_EXTENSIONS.test(file.name);
}
