import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

test("Texas Roadhouse store troubleshooting uses the shipped coordinate flags", async () => {
  const readmePath = resolve(
    process.cwd(),
    "../library/food-and-dining/texas-roadhouse/README.md",
  );
  const readme = await readFile(readmePath, "utf8");

  assert.doesNotMatch(readme, /stores --latitude/);
  assert.doesNotMatch(readme, /--longitude <lon>/);
  assert.match(readme, /stores --lat <lat> --long <lon>/);
});
