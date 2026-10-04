import { test } from "node:test";
import assert from "node:assert/strict";
import { parseHash, fileHash } from "../src/core/router.js";

test("file routes round-trip reserved characters and reject malformed escapes", () => {
  globalThis.location = { hash: fileHash("docs/a#b?c% d.txt") };
  assert.equal(parseHash().path, "docs/a#b?c% d.txt");
  location.hash = "#/file/%E0%A4%A";
  assert.equal(parseHash().path, null);
});
