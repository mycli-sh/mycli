import { readFileSync, writeFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { Resvg } from "@resvg/resvg-js";

const __dirname = dirname(fileURLToPath(import.meta.url));
const svgPath = join(__dirname, "..", "public", "og-image.svg");
const fontPath = join(__dirname, "..", "assets", "JetBrainsMono-Bold.ttf");
const pngPath = join(__dirname, "..", "public", "og-image.png");

const svg = readFileSync(svgPath, "utf8");

const resvg = new Resvg(svg, {
  fitTo: { mode: "width", value: 1200 },
  font: {
    fontFiles: [fontPath],
    loadSystemFonts: true,
    defaultFontFamily: "Arial",
  },
});
const png = resvg.render().asPng();

writeFileSync(pngPath, png);
console.log(`og-image.png written (${(png.length / 1024).toFixed(1)} KB)`);
