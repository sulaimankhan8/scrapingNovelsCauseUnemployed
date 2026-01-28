const fs = require("fs");

/* ========= INPUT ========= */
const RAW_FILE = "chapters.txt";
const OUTPUT_FILE = "chapters.json";

const NOVEL_ID = "endless-evolution-last-star";
const BASE_URL = "https://novelbin.com/b/endless-evolution-last-star";

/* ========= HELPERS ========= */
function slugify(title) {
  return title
    .toLowerCase()
    .replace(/['"”“’]/g, "")
    .replace(/[^a-z0-9\s-]/g, "")
    .trim()
    .replace(/\s+/g, "-");
}

function cleanTitle(raw, chapterNumber) {
  return raw
    // remove "Chapter xxx", "Capítulo xxx"
    .replace(/chapter\s*\d+/i, "")
    .replace(/cap[ií]tulo\s*\d+/i, "")
    // remove duplicated numbers like "783 783"
    .replace(new RegExp(`\\b${chapterNumber}\\b`, "g"), "")
    // remove leading symbols
    .replace(/^[-:–\s]+/, "")
    .trim();
}

/* ========= MAIN ========= */
const raw = fs.readFileSync(RAW_FILE, "utf-8");

const chapters = raw
  .split("\n")
  .map(line => line.trim())
  .filter(Boolean)
  .map(line => {
    // Capture chapter number
    const numMatch = line.match(/chapter\s*(\d+)/i);
    if (!numMatch) return null;

    const number = Number(numMatch[1]);
    const title = cleanTitle(line, number);

    if (!title) return null;

    return {
      number,
      title,
      url: `${BASE_URL}/chapter-${number}-${slugify(title)}`
    };
  })
  .filter(Boolean)
  .sort((a, b) => a.number - b.number);

const output = {
  novel_id: NOVEL_ID,
  base_url: BASE_URL,
  total_chapters: chapters.length,
  chapters
};

fs.writeFileSync(OUTPUT_FILE, JSON.stringify(output, null, 2));

console.log(`✅ Generated ${chapters.length} chapters`);
