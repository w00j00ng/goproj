import assert from "node:assert/strict";
import { scryptSync } from "node:crypto";
import { writeFileSync } from "node:fs";

const salt = "00112233445566778899aabbccddeeff";
const cases = [
  { name: "30 combining marks", plain: "a" + "\u0301".repeat(30), supported: true },
  { name: "31 combining marks", plain: "a" + "\u0301".repeat(31), supported: false },
  { name: "40 combining marks", plain: "a" + "\u0301".repeat(40), supported: false },
  { name: "original joiner", plain: "a\u034f\u0301", supported: true },
  { name: "original joiner separates long sequence", plain: "a" + "\u0301".repeat(30) + "\u034f" + "\u0301".repeat(10), supported: true },
  { name: "original joiner does not hide inserted joiner", plain: "\u034fa" + "\u0301".repeat(40), supported: false },
  { name: "decomposition expands combining sequence", plain: "a" + "\u0344".repeat(16), supported: false },
];
assert.equal(cases[2].plain.normalize("NFKC"), "\u00e1" + "\u0301".repeat(39));
assert.equal(cases[3].plain.normalize("NFKC"), cases[3].plain);
const fixtures = cases.map(c => ({
  ...c,
  normalized: c.plain.normalize("NFKC"),
  hash: salt + ":" + scryptSync(c.plain.normalize("NFKC"), salt, 64,
    { N: 16384, r: 16, p: 1, maxmem: 67108864 }).toString("hex"),
}));
writeFileSync(new URL("./unicode.json", import.meta.url), JSON.stringify({
  environment: { node: process.versions.node, icu: process.versions.icu, unicode: process.versions.unicode },
  fixtures,
}, null, 2) + "\n");
