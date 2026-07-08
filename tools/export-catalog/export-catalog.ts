// Ports: scripts/generate-models.ts (data-shape only — see docs/PORTING.md's
// "Out of scope" note: the ~2100-line generator itself is not ported).
//
// Serializes the pinned upstream checkout's generated model catalogs
// (models.generated.ts, image-models.generated.ts) to the JSON files
// ai/catalog embeds via go:embed. Byte-stable: re-running against the same
// upstream/UPSTREAM.lock commit reproduces identical output, since both
// inputs are themselves generated deterministically and this script performs
// no reordering.
//
// Usage (after upstream/sync.sh has cloned/updated the pinned checkout):
//   npx tsx tools/export-catalog/export-catalog.ts
// or, since the inputs only need type-erasure, any TS-capable runner works:
//   bun run tools/export-catalog/export-catalog.ts

import { mkdirSync, readdirSync, unlinkSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(here, "..", "..");
const checkout = process.env.UPSTREAM_CLONE_DIR ?? join(repoRoot, "upstream", ".upstream-clone");
const srcDir = join(checkout, "packages", "ai", "src");
const outDir = join(repoRoot, "ai", "catalog", "data");

function writeJSON(path: string, data: unknown): void {
	mkdirSync(dirname(path), { recursive: true });
	writeFileSync(path, `${JSON.stringify(data, null, "\t")}\n`);
}

/** Removes stale <dir>/*.json files from a previous export before writing the new set. */
function clearDir(dir: string): void {
	mkdirSync(dir, { recursive: true });
	for (const entry of readdirSync(dir)) {
		if (entry.endsWith(".json")) unlinkSync(join(dir, entry));
	}
}

async function main() {
	const { MODELS } = await import(join(srcDir, "models.generated.ts"));
	const { IMAGE_MODELS } = await import(join(srcDir, "image-models.generated.ts"));

	const modelsDir = join(outDir, "models");
	clearDir(modelsDir);
	for (const [provider, models] of Object.entries(MODELS as Record<string, Record<string, unknown>>)) {
		writeJSON(join(modelsDir, `${provider}.json`), Object.values(models));
	}

	const imagesDir = join(outDir, "images");
	clearDir(imagesDir);
	for (const [provider, models] of Object.entries(IMAGE_MODELS as Record<string, Record<string, unknown>>)) {
		writeJSON(join(imagesDir, `${provider}.json`), Object.values(models));
	}

	console.log(`Exported ${Object.keys(MODELS).length} model providers, ${Object.keys(IMAGE_MODELS).length} image providers.`);
}

main();
