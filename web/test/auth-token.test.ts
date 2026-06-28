import test from "node:test";
import assert from "node:assert/strict";

import { authHeaderForToken, COOKIE_AUTH_TOKEN } from "../src/services/api/auth-token.ts";

test("authHeaderForToken omits bearer header for cookie session marker", () => {
    assert.equal(authHeaderForToken(COOKIE_AUTH_TOKEN), undefined);
});

test("authHeaderForToken returns bearer header for jwt token", () => {
    assert.deepEqual(authHeaderForToken("jwt-token"), { Authorization: "Bearer jwt-token" });
});
