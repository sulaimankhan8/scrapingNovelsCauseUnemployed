const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");

// Retry specific chapters
const CHAPTERS_TO_RETRY = [1351];
const BASE_URL = "https://novelbin.com/b/lord-of-the-mysteries/chapter-";
const RAW_DIR = "./scraped_raw";

(async () => {
    console.log(`🔁 Retrying 1 chapters...`);
    console.log("Chapters:", CHAPTERS_TO_RETRY.join(', '));
    console.log("=" .repeat(50));
    
    for (const chapterNum of CHAPTERS_TO_RETRY) {
        console.log(`\n[${chapterNum}] Starting retry...`);
        await retryChapter(chapterNum);
    }
    
    console.log("\n✅ All retries completed!");
})();

async function retryChapter(chapterNum) {
    const chapterUrl = `${BASE_URL}${chapterNum}`;
    const rawFile = path.join(RAW_DIR, `scraped_chapter_${chapterNum}.html`);
    
    let retryCount = 0;
    const maxRetries = 3;
    
    while (retryCount < maxRetries) {
        try {
            console.log(`  Attempt ${retryCount + 1}/${maxRetries}...`);
            
            const browser = await chromium.launch({ 
                headless: false,  // Use visible browser for debugging
                args: [
                    '--disable-blink-features=AutomationControlled',
                    '--disable-dev-shm-usage',
                    '--no-sandbox',
                    '--disable-extensions',
                    '--no-first-run',
                    '--disable-gpu',
                    '--single-process',  // Faster for single pages
                    '--disable-accelerated-2d-canvas',
                    '--disable-setuid-sandbox',
                    '--window-size=1024,768',  // Smaller window size
                '--window-position=1000,1000'  
                ]
            });
            
            const context = await browser.newContext({
                viewport: { width: 1920, height: 1080 },
                userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            });
            
            // Add basic anti-detection
            await context.addInitScript(() => {
                Object.defineProperty(navigator, 'webdriver', { get: () => false });
            });
            
            const page = await context.newPage();
            page.setDefaultTimeout(45000);
            page.setDefaultNavigationTimeout(60000);
            
            // Extra headers
            await page.setExtraHTTPHeaders({
                'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
                'Accept-Language': 'en-US,en;q=0.9',
                'Referer': 'https://novelbin.com/',
            });
            
            console.log(`  Navigating to chapter...`);
            await page.goto(chapterUrl, { 
                waitUntil: 'domcontentloaded',
                timeout: 60000,
            });
            
            // Check for Cloudflare
            const pageTitle = await page.title();
            if (pageTitle.includes('Just a moment')) {
                console.log(`  ⚠️ Cloudflare detected, waiting 20 seconds...`);
                await delay(20000);
                
                // Take screenshot for debugging
                await page.screenshot({ path: `cf_debug_${chapterNum}.png` });
                console.log(`  📸 Screenshot saved: cf_debug_${chapterNum}.png`);
            }
            
            // Wait for content
            console.log(`  Waiting for content...`);
            try {
                await page.waitForSelector('#chr-content, .chr-c, .chapter-content', { 
                    timeout: 30000 
                });
            } catch (e) {
                console.log(`  Content selector not found, trying alternative...`);
                await page.waitForFunction(() => {
                    return document.querySelector('body').textContent.length > 500;
                }, { timeout: 20000 });
            }
            
            // Additional wait
            await delay(3000);
            
            // Get content
            const html = await page.content();
            
            // Save to file
            fs.writeFileSync(rawFile, html, 'utf-8');
            console.log(`  ✅ Successfully saved chapter ${chapterNum}`);
            
            await browser.close();
            return true;
            
        } catch (error) {
            retryCount++;
            console.log(`  ✗ Attempt ${retryCount} failed: ${error.message}`);
            
            if (retryCount === maxRetries) {
                console.log(`  ❌ Failed after ${maxRetries} attempts`);
                fs.writeFileSync(rawFile, `<!-- ERROR: ${error.message.replace(/-->/g, '')} -->`, 'utf-8');
                return false;
            }
            
            // Wait before retry
            await delay(5000);
        }
    }
}

function delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}
