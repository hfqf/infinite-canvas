export const COOKIE_AUTH_TOKEN = "__cookie_session__";

export function authHeaderForToken(token?: string) {
    return token && token !== COOKIE_AUTH_TOKEN ? { Authorization: `Bearer ${token}` } : undefined;
}

export function isCookieAuthToken(token: string) {
    return token === COOKIE_AUTH_TOKEN;
}
