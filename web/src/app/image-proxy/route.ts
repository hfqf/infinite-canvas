import type { NextRequest } from "next/server";

export const runtime = "nodejs";
export const maxDuration = 300;

export async function GET(request: NextRequest) {
    const url = request.nextUrl.searchParams.get("url") || "";
    if (!isAllowedImageUrl(url)) {
        return new Response("Invalid image url", { status: 400 });
    }

    try {
        const response = await fetch(url, { redirect: "follow" });
        if (!response.ok) {
            return new Response("Failed to fetch image", { status: response.status });
        }
        const contentType = response.headers.get("content-type") || "application/octet-stream";
        if (!contentType.toLowerCase().startsWith("image/")) {
            return new Response("Unsupported media type", { status: 415 });
        }

        const headers = new Headers();
        headers.set("content-type", contentType);
        headers.set("cache-control", "private, max-age=3600");
        return new Response(response.body, { status: 200, headers });
    } catch (error) {
        console.error("Failed to proxy image", url, error);
        return new Response("Failed to fetch image", { status: 502 });
    }
}

function isAllowedImageUrl(value: string) {
    try {
        const url = new URL(value);
        return url.protocol === "http:" || url.protocol === "https:";
    } catch {
        return false;
    }
}
