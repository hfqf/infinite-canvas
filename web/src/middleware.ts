import { NextResponse, type NextRequest } from "next/server";

const publicRootHosts = new Set(["haotushow.com", "www.haotushow.com"]);
const workbenchHosts = new Set(["workbench.haotushow.com"]);
const bypassPrefixes = ["/api", "/_next", "/image-proxy", "/webdav-proxy"];
const bypassFilePattern = /\.(?:ico|png|jpg|jpeg|webp|gif|svg|css|js|txt|xml|json|map|woff2?)$/i;
const workbenchAllowedPrefixes = ["/workbench", "/canvas", "/login", "/image-history", "/deduction-logs", "/assets", "/asset-library", "/admin"];

export function middleware(request: NextRequest) {
    const host = request.headers.get("host")?.split(":")[0].toLowerCase() || "";
    const { pathname } = request.nextUrl;
    if (shouldBypass(pathname)) return NextResponse.next();

    if (publicRootHosts.has(host) && pathname === "/") {
        return rewrite(request, "/site");
    }
    if (workbenchHosts.has(host)) {
        if (pathname === "/") return rewrite(request, "/workbench");
        if (!workbenchAllowedPrefixes.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`))) return rewrite(request, "/workbench");
    }
    return NextResponse.next();
}

function shouldBypass(pathname: string) {
    return bypassPrefixes.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`)) || bypassFilePattern.test(pathname);
}

function rewrite(request: NextRequest, pathname: string) {
    const url = request.nextUrl.clone();
    url.pathname = pathname;
    return NextResponse.rewrite(url);
}

export const config = {
    matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
