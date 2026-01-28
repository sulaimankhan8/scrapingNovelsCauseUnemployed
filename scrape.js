const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");
const cheerio = require("cheerio");

(async () => {
    try {
        // Configuration
        const BASE_URL = "https://novelbin.com/b/lord-of-the-mysteries/chapter-";
        const START_CHAPTER = 1;
        const END_CHAPTER = 1432;
        const BATCH_SIZE = 10; // Process chapters in batches
        const RAW_DIR = "./scraped_raw";
        const CLEAN_DIR = "./clean_chapters";
        const MAX_CONCURRENT = 2; // Max concurrent scrapers
        
        // Create directories
        [RAW_DIR, CLEAN_DIR].forEach(dir => {
            if (!fs.existsSync(dir)) {
                fs.mkdirSync(dir, { recursive: true });
            }
        });

        console.log(`Starting to scrape chapters ${START_CHAPTER} to ${END_CHAPTER}`);
        console.log("=" .repeat(50));
        
        // Create batches for processing
        const chapters = Array.from(
            { length: END_CHAPTER - START_CHAPTER + 1 },
            (_, i) => START_CHAPTER + i
        );
        
        // Process in batches
        for (let batchStart = 0; batchStart < chapters.length; batchStart += BATCH_SIZE) {
            const batchEnd = Math.min(batchStart + BATCH_SIZE, chapters.length);
            const currentBatch = chapters.slice(batchStart, batchEnd);
            
            console.log(`\n📦 Processing batch ${batchStart/BATCH_SIZE + 1} (Chapters ${currentBatch[0]}-${currentBatch[currentBatch.length - 1]})`);
            
            // Process batch with concurrency
            await processBatch(currentBatch, MAX_CONCURRENT);
            
            // Clean after each batch to free memory
            if (global.gc) {
                global.gc();
            }
            
            // Random delay between batches
            if (batchEnd < chapters.length) {
                const delayTime = Math.random() * 3000 + 2000;
                console.log(`  ⏳ Batch completed. Waiting ${Math.round(delayTime/1000)}s...`);
                await delay(delayTime);
            }
        }

        console.log("\n" + "=" .repeat(50));
        console.log("✅ Raw scraping completed!");
        
        // Start cleaning process
        console.log("\nStarting to clean scraped files...");
        await cleanChapters();
        
    } catch (error) {
        console.error(`Fatal error: ${error.message}`);
        console.error(error.stack);
    }
})();

async function processBatch(chapters, maxConcurrent) {
    const promises = [];
    const active = [];
    
    for (const chapterNum of chapters) {
        // Wait if we have too many concurrent scrapers
        if (active.length >= maxConcurrent) {
            await Promise.race(active);
        }
        
        const promise = scrapeChapter(chapterNum).finally(() => {
            // Remove from active when done
            const index = active.indexOf(promise);
            if (index > -1) {
                active.splice(index, 1);
            }
        });
        
        promises.push(promise);
        active.push(promise);
        
        // Small delay between starting scrapers
        await delay(500);
    }
    
    // Wait for all to complete
    return Promise.allSettled(promises);
}

async function scrapeChapter(chapterNum) {
    const BASE_URL = "https://novelbin.com/b/lord-of-the-mysteries/chapter-";
    const RAW_DIR = "./scraped_raw";
    
    const chapterUrl = `${BASE_URL}${chapterNum}`;
    const rawFile = path.join(RAW_DIR, `scraped_chapter_${chapterNum}.html`);
    
    // Check cache first
    if (fs.existsSync(rawFile)) {
        try {
            const fileContent = fs.readFileSync(rawFile, 'utf-8');
            if (!fileContent.includes('<!-- ERROR:') && !fileContent.includes('ERROR: Failed to scrape')) {
                console.log(`  [${chapterNum}] ✓ Already cached`);
                return { chapterNum, success: true };
            }
        } catch (e) {
            // Continue to scrape
        }
    }
    
    let retryCount = 0;
    const maxRetries = 2; // Reduced retries for speed
    let browser = null;
    
    while (retryCount < maxRetries) {
        try {
            console.log(`  [${chapterNum}] Attempt ${retryCount + 1}/${maxRetries}...`);
            
            // Launch browser with minimal settings
            browser = await chromium.launch({ 
                headless: true,  // Headless is faster
                args: [
                    '--disable-blink-features=AutomationControlled',
                    '--disable-dev-shm-usage',
                    '--no-sandbox',
                    '--disable-extensions',
                    '--no-first-run',
                    '--disable-gpu',
                    '--single-process',  // Faster for single pages
                    '--disable-accelerated-2d-canvas',
                    '--disable-setuid-sandbox'
                ]
            });

            const context = await browser.newContext({
                viewport: { width: 1366, height: 768 },
                userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
                locale: 'en-US',
            });

            // Minimal stealth
            await context.addInitScript(() => {
                delete navigator.__proto__.webdriver;
            });

            const page = await context.newPage();
            page.setDefaultTimeout(20000);  // Reduced timeout
            page.setDefaultNavigationTimeout(30000);

            // Simple headers
            await page.setExtraHTTPHeaders({
                'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
                'Accept-Language': 'en-US,en;q=0.9',
                'Referer': 'https://novelbin.com/',
            });

            // Navigate directly
            try {
                await page.goto(chapterUrl, { 
                    waitUntil: 'domcontentloaded',  // Faster than networkidle
                    timeout: 30000,
                });
            } catch (navError) {
                console.log(`  [${chapterNum}] Navigation timeout, trying load...`);
                await page.goto(chapterUrl, { 
                    waitUntil: 'load',
                    timeout: 30000,
                });
            }

            // Quick Cloudflare check
            const pageTitle = await page.title();
            if (pageTitle.includes('Just a moment')) {
                console.log(`  [${chapterNum}] ⚠️ Cloudflare detected, waiting...`);
                await delay(8000);  // Shorter wait
            }

            // Try to get content quickly with multiple selectors
            let content = null;
            const contentSelectors = [
                '#chr-content',
                '.chr-c',
                '.chapter-content',
                '.content-area',
                '.reading-content',
                '.text-left',
                '.entry-content',
                'article',
                'div:has(> p)'
            ];

            for (const selector of contentSelectors) {
                try {
                    const element = await page.$(selector);
                    if (element) {
                        content = await element.innerHTML();
                        if (content && content.length > 500) break;
                    }
                } catch (e) {
                    continue;
                }
            }

            // Fallback: get body content
            if (!content) {
                content = await page.evaluate(() => document.body.innerHTML);
            }

            // Check if we got content
            if (!content || content.length < 500) {
                // Quick check for error page
                const textContent = await page.evaluate(() => document.body.textContent);
                if (textContent.length < 500) {
                    throw new Error('No substantial content found');
                }
            }

            // Save raw HTML
            fs.writeFileSync(rawFile, content, 'utf-8');
            console.log(`  [${chapterNum}] ✓ Scraped successfully`);
            
            // Close browser
            await browser.close();
            return { chapterNum, success: true };
            
        } catch (error) {
            retryCount++;
            console.log(`  [${chapterNum}] ✗ Attempt ${retryCount} failed: ${error.message}`);
            
            // Close browser on error
            if (browser) {
                try {
                    await browser.close();
                } catch (e) {
                    // Ignore
                }
                browser = null;
            }
            
            if (retryCount === maxRetries) {
                console.log(`  [${chapterNum}] ✗ Failed after ${maxRetries} attempts`);
                // Save minimal error file
                fs.writeFileSync(rawFile, `<!-- ERROR: ${error.message.replace(/-->/g, '')} -->`, 'utf-8');
                return { chapterNum, success: false, error: error.message };
            }
            
            // Exponential backoff
            const waitTime = Math.min(5000, 1000 * Math.pow(2, retryCount));
            await delay(waitTime);
        }
    }
}

// Helper function for delays
function delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

// Cleaner Function - Optimized for speed
async function cleanChapters() {
    const RAW_DIR = "./scraped_raw";
    const CLEAN_DIR = "./clean_chapters";
    
    if (!fs.existsSync(RAW_DIR)) {
        console.log("✗ No raw files found to clean!");
        return;
    }
    
    // Get all scraped files
    const files = fs.readdirSync(RAW_DIR)
        .filter(file => file.startsWith('scraped_chapter_') && file.endsWith('.html'))
        .sort((a, b) => {
            const numA = parseInt(a.match(/\d+/)[0]);
            const numB = parseInt(b.match(/\d+/)[0]);
            return numA - numB;
        });
    
    console.log(`Found ${files.length} raw files to clean`);
    
    let successCount = 0;
    let errorCount = 0;
    
    // Process files in chunks
    const chunkSize = 20;
    for (let i = 0; i < files.length; i += chunkSize) {
        const chunk = files.slice(i, i + chunkSize);
        
        console.log(`\n🧹 Cleaning batch ${Math.floor(i/chunkSize) + 1}/${Math.ceil(files.length/chunkSize)}`);
        
        for (const file of chunk) {
            const chapterNum = file.match(/\d+/)[0];
            const cleanFile = path.join(CLEAN_DIR, `chapter_${chapterNum.padStart(4, '0')}.html`);
            
            try {
                // Read raw HTML
                const html = fs.readFileSync(path.join(RAW_DIR, file), 'utf-8');
                
                // Skip error files
                if (html.includes('<!-- ERROR:') || html.length < 500) {
                    console.log(`  [${chapterNum}] ✗ Skipping error/empty file`);
                    errorCount++;
                    continue;
                }
                
                // Parse with Cheerio
                const $ = cheerio.load(html);
                
                // Extract chapter title
                let title = $('h3.chr-title').text().trim() || 
                           $('.chr-title').text().trim() ||
                           $('h1, h2').first().text().trim() ||
                           $('title').text().split(' - ')[0] ||
                           `Chapter ${chapterNum}`;
                
                // Clean title
                title = title.replace(/\s+/g, ' ').trim().substring(0, 200);
                
                // Extract chapter content - try common selectors
                let content = null;
                const contentSelectors = [
                    '#chr-content',
                    '.chr-c',
                    '.chapter-content',
                    '.content-area',
                    '.reading-content',
                    '.text-left',
                    '.entry-content',
                    'article',
                    'body > div'
                ];
                
                for (const selector of contentSelectors) {
                    const element = $(selector).first();
                    if (element.length && element.text().length > 500) {
                        content = element.html();
                        break;
                    }
                }
                
                // Fallback: get body content
                if (!content) {
                    content = $('body').html();
                }
                
                if (!content || content.length < 500) {
                    throw new Error('No substantial content found');
                }
                
                // Fast cleaning using regex for common issues
                let cleanedContent = content
                    .replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, '')
                    .replace(/<style\b[^<]*(?:(?!<\/style>)<[^<]*)*<\/style>/gi, '')
                    .replace(/<iframe\b[^<]*(?:(?!<\/iframe>)<[^<]*)*<\/iframe>/gi, '')
                    .replace(/class="[^"]*ad[^"]*"/gi, '')
                    .replace(/id="[^"]*ad[^"]*"/gi, '')
                    .replace(/<div[^>]*class="[^"]*ads?[^"]*"[^>]*>.*?<\/div>/gis, '')
                    .replace(/<ins[^>]*>.*?<\/ins>/gis, '')
                    .replace(/&nbsp;/g, ' ')
                    .replace(/\n\s*\n\s*\n/g, '\n\n')
                    .trim();
                
                // Create clean HTML with minimal processing
                const cleanHtml = createCleanHtml(chapterNum, title, cleanedContent);
                
                // Save cleaned file
                fs.writeFileSync(cleanFile, cleanHtml, 'utf-8');
                console.log(`  [${chapterNum}] ✓ Cleaned`);
                successCount++;
                
            } catch (error) {
                console.log(`  [${chapterNum}] ✗ Error: ${error.message}`);
                errorCount++;
            }
        }
    }
    
    console.log("\n" + "=" .repeat(50));
    console.log("✅ Cleaning completed!");
    console.log(`Success: ${successCount} chapters`);
    console.log(`Errors: ${errorCount} chapters`);
    
    // Create index.html if we have clean chapters
    if (successCount > 0) {
        createIndexHtml(successCount);
    }
}

// Function to create clean HTML structure
function createCleanHtml(chapterNum, title, content) {
    const prevChapter = parseInt(chapterNum) > 1 ? 
        `chapter_${(parseInt(chapterNum) - 1).toString().padStart(4, '0')}.html` : null;
    const nextChapter = `chapter_${(parseInt(chapterNum) + 1).toString().padStart(4, '0')}.html`;
    
    return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${title} - Chapter ${chapterNum}</title>
    <style>
        body {
            font-family: 'Georgia', serif;
            line-height: 1.8;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
            color: #333;
        }
        .container {
            background-color: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 {
            text-align: center;
            color: #2c3e50;
            border-bottom: 2px solid #3498db;
            padding-bottom: 10px;
            margin-bottom: 30px;
        }
        .content {
            font-size: 18px;
            text-align: justify;
        }
        .content p {
            margin-bottom: 1.5em;
            text-indent: 2em;
            line-height: 1.8;
        }
        .nav {
            display: flex;
            justify-content: space-between;
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid #ddd;
        }
        .nav a {
            padding: 10px 20px;
            background-color: #3498db;
            color: white;
            text-decoration: none;
            border-radius: 5px;
        }
        .nav a:hover {
            background-color: #2980b9;
        }
        @media (max-width: 768px) {
            body {
                padding: 10px;
            }
            .container {
                padding: 15px;
            }
            .content {
                font-size: 16px;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Chapter ${chapterNum}: ${title}</h1>
        <div class="content">
            ${content}
        </div>
        <div class="nav">
            ${prevChapter ? `<a href="${prevChapter}">← Previous</a>` : '<span></span>'}
            <a href="index.html">🏠 Chapters</a>
            <a href="${nextChapter}">Next →</a>
        </div>
    </div>
</body>
</html>`;
}

// Create index.html
function createIndexHtml(totalChapters) {
    const CLEAN_DIR = "./clean_chapters";
    
    const chapters = Array.from({ length: totalChapters }, (_, i) => i + 1);
    
    const indexHtml = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Lord of the Mysteries - Complete Collection</title>
    <style>
        body {
            font-family: 'Segoe UI', Tahoma, sans-serif;
            line-height: 1.6;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
        }
        .container {
            background-color: white;
            border-radius: 15px;
            padding: 30px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.2);
        }
        header {
            text-align: center;
            margin-bottom: 30px;
            padding-bottom: 20px;
            border-bottom: 2px solid #f0f0f0;
        }
        h1 {
            color: #2c3e50;
            font-size: 32px;
            margin-bottom: 10px;
        }
        .stats {
            display: flex;
            justify-content: center;
            gap: 30px;
            margin: 30px 0;
            flex-wrap: wrap;
        }
        .stat {
            background-color: #f8f9fa;
            padding: 15px 30px;
            border-radius: 8px;
            text-align: center;
            min-width: 120px;
        }
        .chapters {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
            gap: 15px;
            margin-top: 20px;
        }
        .chapter {
            background-color: white;
            border: 1px solid #ddd;
            border-radius: 8px;
            padding: 15px;
            text-align: center;
        }
        .chapter a {
            text-decoration: none;
            color: #3498db;
            font-weight: bold;
            display: block;
        }
        .chapter:hover {
            border-color: #3498db;
            transform: translateY(-2px);
            box-shadow: 0 5px 15px rgba(52, 152, 219, 0.2);
        }
        @media (max-width: 768px) {
            .chapters {
                grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Lord of the Mysteries</h1>
            <p>Complete Collection - ${totalChapters} Chapters</p>
        </header>
        
        <div class="stats">
            <div class="stat">
                <strong>${totalChapters}</strong><br>Chapters
            </div>
            <div class="stat">
                <strong>${new Date().getFullYear()}</strong><br>Year
            </div>
        </div>
        
        <h2>Chapters:</h2>
        <div class="chapters">
            ${chapters.map(num => `
                <div class="chapter">
                    <a href="chapter_${num.toString().padStart(4, '0')}.html">
                        Chapter ${num}
                    </a>
                </div>
            `).join('')}
        </div>
    </div>
</body>
</html>`;
    
    fs.writeFileSync(path.join(CLEAN_DIR, "index.html"), indexHtml, 'utf-8');
    console.log(`\n📚 Created index.html with ${totalChapters} chapters`);
    console.log(`\n🎉 COMPLETED!`);
    console.log(`📁 Clean files: ${path.resolve(CLEAN_DIR)}`);
    console.log(`📄 Open: file://${path.resolve(CLEAN_DIR, "index.html")}`);
}