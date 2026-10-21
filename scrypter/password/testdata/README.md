# Unicode compatibility fixtures

Run `node password/testdata/generate.mjs` from the scrypter module.
The script uses Node.js NFKC and crypto.scryptSync independently of Go.
The committed JSON records its Node, ICU and Unicode versions.

Cases marked unsupported must return an explicit error from both Hash and
Verify: Go's stream-safe normalization inserts U+034F, while JavaScript does
not. Cases with original U+034F prove that it must not be stripped.
Normal tests read the committed JSON and do not require Node.js.
