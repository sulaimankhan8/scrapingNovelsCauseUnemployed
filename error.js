const fs = require("fs");
const path = require("path");
const cheerio = require("cheerio");

(async () => {
    try {
        console.log("🔍 Scanning for Cloudflare-protected/error chapters...");
        console.log("=" .repeat(50));
        
        // Configuration
        const RAW_DIR = "./scraped_raw";
        const CLEAN_DIR = "./clean_chapters";
        
        // Check if directories exist
        if (!fs.existsSync(RAW_DIR)) {
            console.log("✗ No raw files found to scan!");
            return;
        }
        
        // Get all raw files
        const rawFiles = fs.readdirSync(RAW_DIR)
            .filter(file => file.startsWith('scraped_chapter_') && file.endsWith('.html'))
            .sort((a, b) => {
                const numA = parseInt(a.match(/\d+/)[0]);
                const numB = parseInt(b.match(/\d+/)[0]);
                return numA - numB;
            });
        
        console.log(`Found ${rawFiles.length} raw files to scan`);
        
        // Check clean files too
        let cleanFiles = [];
        if (fs.existsSync(CLEAN_DIR)) {
            cleanFiles = fs.readdirSync(CLEAN_DIR)
                .filter(file => file.startsWith('chapter_') && file.endsWith('.html'))
                .sort((a, b) => {
                    const numA = parseInt(a.match(/\d+/)[0]);
                    const numB = parseInt(b.match(/\d+/)[0]);
                    return numA - numB;
                });
            console.log(`Found ${cleanFiles.length} clean files to scan`);
        }
        
        // Scan for Cloudflare protection
        const cloudflareKeywords = [
            'Verifying you are human',
            'novelbin.com needs to review the security',
            'cf-turnstile-response',
            'cf_challenge_response',
            'Enable JavaScript and cookies to continue',
            'Just a moment',
            'Checking your browser',
            'DDOS protection'
        ];
        
        const badRawFiles = [];
        const badCleanFiles = [];
        
        console.log("\n📊 Scanning raw files for Cloudflare protection...");
        for (const file of rawFiles) {
            const filePath = path.join(RAW_DIR, file);
            const chapterNum = file.match(/\d+/)[0];
            
            try {
                const content = fs.readFileSync(filePath, 'utf-8');
                
                // Check for Cloudflare
                const hasCloudflare = cloudflareKeywords.some(keyword => 
                    content.includes(keyword)
                );
                
                // Check for error markers
                const hasError = content.includes('<!-- ERROR:') || 
                                content.includes('ERROR: Failed to scrape');
                
                // Check if content is too small
                const isTooSmall = content.length < 1000;
                
                if (hasCloudflare || hasError || isTooSmall) {
                    badRawFiles.push({
                        file,
                        chapterNum,
                        reason: hasCloudflare ? 'Cloudflare' : 
                               hasError ? 'Scraping Error' : 
                               'Insufficient Content'
                    });
                    console.log(`  [${chapterNum}] ✗ ${hasCloudflare ? 'Cloudflare' : hasError ? 'Error' : 'Small'}: ${file}`);
                } else {
                    console.log(`  [${chapterNum}] ✓ Clean`);
                }
                
            } catch (error) {
                console.log(`  [${chapterNum}] ✗ Read Error: ${error.message}`);
                badRawFiles.push({
                    file,
                    chapterNum,
                    reason: 'File Read Error'
                });
            }
        }
        
        console.log("\n📊 Scanning clean files for Cloudflare protection...");
        for (const file of cleanFiles) {
            const filePath = path.join(CLEAN_DIR, file);
            const chapterNum = file.match(/\d+/)[0];
            
            try {
                const content = fs.readFileSync(filePath, 'utf-8');
                
                // Check for Cloudflare in clean files too
                const hasCloudflare = cloudflareKeywords.some(keyword => 
                    content.includes(keyword)
                );
                
                // Check for generic error indicators
                const hasError = content.includes('Verification successful') && 
                                content.includes('Waiting for novelbin.com to respond');
                
                if (hasCloudflare || hasError) {
                    badCleanFiles.push({
                        file,
                        chapterNum,
                        reason: 'Cloudflare in Clean File'
                    });
                    console.log(`  [${chapterNum}] ✗ Cloudflare detected in clean file: ${file}`);
                } else {
                    console.log(`  [${chapterNum}] ✓ Clean`);
                }
                
            } catch (error) {
                console.log(`  [${chapterNum}] ✗ Read Error: ${error.message}`);
            }
        }
        
        console.log("\n" + "=" .repeat(50));
        console.log("📋 Scan Results:");
        console.log(`Total Raw Files: ${rawFiles.length}`);
        console.log(`Bad Raw Files: ${badRawFiles.length}`);
        console.log(`Total Clean Files: ${cleanFiles.length}`);
        console.log(`Bad Clean Files: ${badCleanFiles.length}`);
        
        if (badRawFiles.length > 0) {
            console.log("\n❌ Problematic Raw Files:");
            badRawFiles.forEach(({file, chapterNum, reason}) => {
                console.log(`  Chapter ${chapterNum}: ${file} (${reason})`);
            });
        }
        
        if (badCleanFiles.length > 0) {
            console.log("\n❌ Problematic Clean Files:");
            badCleanFiles.forEach(({file, chapterNum, reason}) => {
                console.log(`  Chapter ${chapterNum}: ${file} (${reason})`);
            });
        }
        
        // Ask for action
        console.log("\n" + "=" .repeat(50));
        console.log("🛠️  Available Actions:");
        console.log("1. List only (don't delete anything)");
        console.log("2. Delete bad raw files only");
        console.log("3. Delete bad clean files only");
        console.log("4. Delete both bad raw and clean files");
        console.log("5. Create retry list for main.js");
        
        // For simplicity, we'll auto-select option 4
        // In a real app, you'd use readline or prompt
        
        if (badRawFiles.length > 0 || badCleanFiles.length > 0) {
            console.log("\n⚠️  Automatically cleaning bad files...");
            
            // Delete bad raw files
            if (badRawFiles.length > 0) {
                console.log("\n🗑️  Deleting bad raw files:");
                for (const {file, chapterNum} of badRawFiles) {
                    const filePath = path.join(RAW_DIR, file);
                    try {
                        fs.unlinkSync(filePath);
                        console.log(`  ✓ Deleted: ${file}`);
                    } catch (error) {
                        console.log(`  ✗ Failed to delete ${file}: ${error.message}`);
                    }
                }
            }
            
            // Delete bad clean files
            if (badCleanFiles.length > 0) {
                console.log("\n🗑️  Deleting bad clean files:");
                for (const {file, chapterNum} of badCleanFiles) {
                    const filePath = path.join(CLEAN_DIR, file);
                    try {
                        fs.unlinkSync(filePath);
                        console.log(`  ✓ Deleted: ${file}`);
                    } catch (error) {
                        console.log(`  ✗ Failed to delete ${file}: ${error.message}`);
                    }
                }
            }
            
            // Create retry list
            console.log("\n📝 Creating retry list for main.js...");
            const retryChapters = [...badRawFiles.map(f => f.chapterNum)];
            
            if (retryChapters.length > 0) {
                const retryFile = path.join(RAW_DIR, 'retry_chapters.txt');
                fs.writeFileSync(retryFile, retryChapters.join('\n'), 'utf-8');
                console.log(`✓ Created retry list: ${retryFile}`);
                console.log(`Chapters to retry: ${retryChapters.join(', ')}`);
                
                // Also create a simple script to retry these chapters
                createRetryScript(retryChapters);
            }
            
        } else {
            console.log("\n✅ No bad files found! Everything looks clean.");
        }
        
        console.log("\n" + "=" .repeat(50));
        console.log("🎉 Error scan completed!");
        
    } catch (error) {
        console.error(`❌ Fatal error: ${error.message}`);
        console.error(error.stack);
    }
})();

function createRetryScript(retryChapters) {
    const scriptContent = `const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");

// Retry specific chapters
const CHAPTERS_TO_RETRY = [${retryChapters.join(', ')}];
const BASE_URL = "https://novelbin.com/b/lord-of-the-mysteries/chapter-";
const RAW_DIR = "./scraped_raw";

(async () => {
    console.log(\`🔁 Retrying ${retryChapters.length} chapters...\`);
    console.log("Chapters:", CHAPTERS_TO_RETRY.join(', '));
    console.log("=" .repeat(50));
    
    for (const chapterNum of CHAPTERS_TO_RETRY) {
        console.log(\`\\n[${"${chapterNum}"}] Starting retry...\`);
        await retryChapter(chapterNum);
    }
    
    console.log("\\n✅ All retries completed!");
})();

async function retryChapter(chapterNum) {
    const chapterUrl = \`\${BASE_URL}\${chapterNum}\`;
    const rawFile = path.join(RAW_DIR, \`scraped_chapter_\${chapterNum}.html\`);
    
    let retryCount = 0;
    const maxRetries = 3;
    
    while (retryCount < maxRetries) {
        try {
            console.log(\`  Attempt \${retryCount + 1}/\${maxRetries}...\`);
            
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
            
            console.log(\`  Navigating to chapter...\`);
            await page.goto(chapterUrl, { 
                waitUntil: 'domcontentloaded',
                timeout: 60000,
            });
            
            // Check for Cloudflare
            const pageTitle = await page.title();
            if (pageTitle.includes('Just a moment')) {
                console.log(\`  ⚠️ Cloudflare detected, waiting 20 seconds...\`);
                await delay(20000);
                
                // Take screenshot for debugging
                await page.screenshot({ path: \`cf_debug_\${chapterNum}.png\` });
                console.log(\`  📸 Screenshot saved: cf_debug_\${chapterNum}.png\`);
            }
            
            // Wait for content
            console.log(\`  Waiting for content...\`);
            try {
                await page.waitForSelector('#chr-content, .chr-c, .chapter-content', { 
                    timeout: 30000 
                });
            } catch (e) {
                console.log(\`  Content selector not found, trying alternative...\`);
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
            console.log(\`  ✅ Successfully saved chapter \${chapterNum}\`);
            
            await browser.close();
            return true;
            
        } catch (error) {
            retryCount++;
            console.log(\`  ✗ Attempt \${retryCount} failed: \${error.message}\`);
            
            if (retryCount === maxRetries) {
                console.log(\`  ❌ Failed after \${maxRetries} attempts\`);
                fs.writeFileSync(rawFile, \`<!-- ERROR: \${error.message.replace(/-->/g, '')} -->\`, 'utf-8');
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
`;
    
    const scriptPath = path.join(process.cwd(), 'retry_failed.js');
    fs.writeFileSync(scriptPath, scriptContent, 'utf-8');
    console.log(`📄 Created retry script: ${scriptPath}`);
    console.log(`💡 Run: node retry_failed.js`);
}