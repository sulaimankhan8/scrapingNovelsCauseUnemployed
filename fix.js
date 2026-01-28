import fs from "fs";
import path from "path";
import * as cheerio from "cheerio";
import { chromium } from "playwright";

async function fixErrorChapters() {
    console.log("=".repeat(60));
    console.log("🛠️  ERROR CHAPTER FIXER");
    console.log("=".repeat(60));
    
    const RAW_DIR = "./scraped_raw";
    const CLEAN_DIR = "./clean_chapters";
    const ERROR_LOG = "./error_chapters.log";
    const ERROR_REPORT = "./error_reports/error_report.html";
    
    // Step 1: Parse error chapters from HTML report
    const errorChapters = await parseErrorChapters(ERROR_REPORT);
    
    if (errorChapters.length === 0) {
        console.log("No error chapters found in the report!");
        console.log("Checking for error chapters in raw files...");
        await findAndFixFromRawFiles();
        return;
    }
    
    console.log(`\n📋 Found ${errorChapters.length} error chapters to fix:`);
    console.log(`   Chapters: ${errorChapters.join(', ')}`);
    
    // Step 2: Delete error files
    await deleteErrorFiles(errorChapters);
    
    // Step 3: Re-scrape error chapters
    await rescrapeChapters(errorChapters);
    
    // Step 4: Clean newly scraped chapters
    await cleanRescrapedChapters(errorChapters);
    
    console.log("\n" + "=".repeat(60));
    console.log("✅ ERROR FIXING COMPLETED!");
    console.log("=".repeat(60));
    console.log(`Fixed ${errorChapters.length} chapters`);
    console.log(`Next steps:`);
    console.log(`1. Run the cleaner again to update the index`);
    console.log(`2. Check the clean_chapters folder for the new files`);
}

async function parseErrorChapters(errorReportPath) {
    if (!fs.existsSync(errorReportPath)) {
        console.log("Error report not found, checking log file...");
        return parseErrorChaptersFromLog();
    }
    
    try {
        const html = fs.readFileSync(errorReportPath, 'utf-8');
        const $ = cheerio.load(html);
        
        // Extract chapter numbers from the HTML
        const chapterNumbers = [];
        
        // Find all chapter badges in the grid
        $('.chapter-badge').each(function() {
            const chapterText = $(this).text().trim();
            const chapterNum = parseInt(chapterText);
            if (!isNaN(chapterNum) && chapterNum > 0) {
                chapterNumbers.push(chapterNum);
            }
        });
        
        // Also check the table rows
        $('tr[data-type]').each(function() {
            const chapterText = $(this).find('td strong').text().trim();
            const chapterNum = parseInt(chapterText);
            if (!isNaN(chapterNum) && chapterNum > 0 && !chapterNumbers.includes(chapterNum)) {
                chapterNumbers.push(chapterNum);
            }
        });
        
        return [...new Set(chapterNumbers)].sort((a, b) => a - b);
    } catch (error) {
        console.log(`Error parsing HTML report: ${error.message}`);
        return parseErrorChaptersFromLog();
    }
}

async function parseErrorChaptersFromLog() {
    const ERROR_LOG = "./error_chapters.log";
    
    if (!fs.existsSync(ERROR_LOG)) {
        console.log("Error log not found either. Checking raw files for errors...");
        return [];
    }
    
    try {
        const logContent = fs.readFileSync(ERROR_LOG, 'utf-8');
        const chapters = [];
        
        // Extract chapters from log lines
        const lines = logContent.split('\n');
        for (const line of lines) {
            const match = line.match(/Chapter\s+(\d+):/);
            if (match) {
                const chapterNum = parseInt(match[1]);
                if (!isNaN(chapterNum)) {
                    chapters.push(chapterNum);
                }
            }
        }
        
        return [...new Set(chapters)].sort((a, b) => a - b);
    } catch (error) {
        console.log(`Error parsing log file: ${error.message}`);
        return [];
    }
}

async function deleteErrorFiles(errorChapters) {
    console.log("\n" + "=".repeat(60));
    console.log("🗑️  DELETING ERROR FILES");
    console.log("=".repeat(60));
    
    const RAW_DIR = "./scraped_raw";
    const CLEAN_DIR = "./clean_chapters";
    
    let deletedRaw = 0;
    let deletedClean = 0;
    
    for (const chapterNum of errorChapters) {
        // Delete raw files
        const rawFiles = [
            `scraped_chapter_${chapterNum}.html`,
            `scraped_chapter_${chapterNum.toString().padStart(3, '0')}.html`
        ];
        
        for (const rawFile of rawFiles) {
            const rawPath = path.join(RAW_DIR, rawFile);
            if (fs.existsSync(rawPath)) {
                try {
                    fs.unlinkSync(rawPath);
                    console.log(`  [${chapterNum}] ✓ Deleted raw file: ${rawFile}`);
                    deletedRaw++;
                } catch (error) {
                    console.log(`  [${chapterNum}] ✗ Error deleting raw file: ${error.message}`);
                }
            }
        }
        
        // Delete clean files
        const cleanFiles = [
            `chapter_${chapterNum}.html`,
            `chapter_${chapterNum.toString().padStart(3, '0')}.html`
        ];
        
        for (const cleanFile of cleanFiles) {
            const cleanPath = path.join(CLEAN_DIR, cleanFile);
            if (fs.existsSync(cleanPath)) {
                try {
                    fs.unlinkSync(cleanPath);
                    console.log(`  [${chapterNum}] ✓ Deleted clean file: ${cleanFile}`);
                    deletedClean++;
                } catch (error) {
                    console.log(`  [${chapterNum}] ✗ Error deleting clean file: ${error.message}`);
                }
            }
        }
    }
    
    console.log(`\n🗑️  Deleted ${deletedRaw} raw files and ${deletedClean} clean files`);
}

async function rescrapeChapters(errorChapters) {
    console.log("\n" + "=".repeat(60));
    console.log("🔄 RE-SCRAPING ERROR CHAPTERS");
    console.log("=".repeat(60));
    
    const BASE_URL = "https://novelbin.com/b/lord-of-the-mysteries/chapter-";
    const RAW_DIR = "./scraped_raw";
    
    // Ensure raw directory exists
    if (!fs.existsSync(RAW_DIR)) {
        fs.mkdirSync(RAW_DIR, { recursive: true });
    }
    
    let successCount = 0;
    let failCount = 0;
    
    for (const chapterNum of errorChapters) {
        console.log(`\n[${chapterNum}] Starting re-scrape...`);
        
        let browser = null;
        let retryCount = 0;
        const maxRetries = 3;
        let scrapedSuccessfully = false;
        
        while (retryCount < maxRetries && !scrapedSuccessfully) {
            retryCount++;
            try {
                const chapterUrl = `${BASE_URL}${chapterNum}`;
                
                console.log(`  Attempt ${retryCount}/${maxRetries} for chapter ${chapterNum}...`);
                
                // Launch browser with stealth options
                browser = await chromium.launch({ 
                    headless: false,
                    args: [
                        '--disable-blink-features=AutomationControlled',
                        '--disable-dev-shm-usage',
                        '--no-sandbox',
                        '--window-size=1920,1080'
                    ]
                });
                
                const context = await browser.newContext({
                    viewport: { width: 1920, height: 1080 },
                    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
                });
                
                // Add stealth scripts
                await context.addInitScript(() => {
                    Object.defineProperty(navigator, 'webdriver', { get: () => false });
                });
                
                const page = await context.newPage();
                page.setDefaultTimeout(60000);
                
                // Set headers
                await page.setExtraHTTPHeaders({
                    'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
                    'Accept-Language': 'en-US,en;q=0.9',
                    'Referer': 'https://novelbin.com/'
                });
                
                console.log(`  Navigating to: ${chapterUrl}`);
                
                // Navigate with longer timeout
                await page.goto(chapterUrl, { 
                    waitUntil: 'networkidle',
                    timeout: 90000
                });
                
                // Check for Cloudflare
                const hasCloudflare = await page.evaluate(() => {
                    return document.title.includes('Just a moment') || 
                           document.querySelector('#challenge-form') !== null;
                });
                
                if (hasCloudflare) {
                    console.log(`  ⚠️ Cloudflare detected, waiting 15 seconds...`);
                    await delay(15000);
                    
                    // Check again
                    const stillHasCloudflare = await page.evaluate(() => {
                        return document.title.includes('Just a moment');
                    });
                    
                    if (stillHasCloudflare) {
                        throw new Error('Cloudflare challenge not resolved');
                    }
                }
                
                // Wait for content
                try {
                    await page.waitForSelector('#chr-content, .chr-c, .chapter-content, div[class*="content"]', { 
                        timeout: 30000 
                    });
                } catch (e) {
                    console.log(`  Using fallback content detection...`);
                    await page.waitForFunction(() => {
                        const content = document.querySelector('body');
                        return content && content.textContent.length > 100;
                    }, { timeout: 20000 });
                }
                
                // Additional delay
                await delay(3000);
                
                // Check if content exists
                const hasContent = await page.evaluate(() => {
                    const content = document.querySelector('#chr-content, .chr-c, .chapter-content');
                    return content && content.textContent.length > 100;
                });
                
                if (!hasContent) {
                    throw new Error('No content found on page');
                }
                
                // Get HTML content
                const html = await page.content();
                
                // Save raw HTML
                const rawFile = path.join(RAW_DIR, `scraped_chapter_${chapterNum}.html`);
                fs.writeFileSync(rawFile, html, 'utf-8');
                
                console.log(`  [${chapterNum}] ✓ Successfully re-scraped`);
                successCount++;
                scrapedSuccessfully = true;
                
                await browser.close();
                
            } catch (error) {
                console.log(`  [${chapterNum}] ✗ Attempt ${retryCount} failed: ${error.message}`);
                
                if (browser) {
                    try {
                        await browser.close();
                    } catch (e) {
                        // Ignore close errors
                    }
                }
                
                if (retryCount === maxRetries) {
                    console.log(`  [${chapterNum}] ✗ Failed after ${maxRetries} attempts`);
                    
                    // Save error file
                    const rawFile = path.join(RAW_DIR, `scraped_chapter_${chapterNum}.html`);
                    fs.writeFileSync(rawFile, `<!-- ERROR: ${error.message} -->`, 'utf-8');
                    
                    failCount++;
                } else {
                    // Wait before retry
                    const waitTime = retryCount * 5000;
                    console.log(`  ⏳ Waiting ${waitTime/1000}s before retry...`);
                    await delay(waitTime);
                }
            }
        }
        
        // Random delay between chapters
        if (chapterNum !== errorChapters[errorChapters.length - 1]) {
            const delayTime = Math.random() * 5000 + 3000;
            console.log(`  ⏳ Waiting ${Math.round(delayTime/1000)}s before next chapter...`);
            await delay(delayTime);
        }
    }
    
    console.log(`\n🔄 Re-scraping completed: ${successCount} successful, ${failCount} failed`);
}

async function cleanRescrapedChapters(errorChapters) {
    console.log("\n" + "=".repeat(60));
    console.log("🧹 CLEANING RE-SCRAPED CHAPTERS");
    console.log("=".repeat(60));
    
    const RAW_DIR = "./scraped_raw";
    const CLEAN_DIR = "./clean_chapters";
    
    // Ensure clean directory exists
    if (!fs.existsSync(CLEAN_DIR)) {
        fs.mkdirSync(CLEAN_DIR, { recursive: true });
    }
    
    let cleanedCount = 0;
    let errorCount = 0;
    
    for (const chapterNum of errorChapters) {
        const rawFile = path.join(RAW_DIR, `scraped_chapter_${chapterNum}.html`);
        const cleanFile = path.join(CLEAN_DIR, `chapter_${chapterNum.toString().padStart(3, '0')}.html`);
        
        if (!fs.existsSync(rawFile)) {
            console.log(`  [${chapterNum}] ✗ Raw file not found, skipping`);
            errorCount++;
            continue;
        }
        
        try {
            // Read raw HTML
            const html = fs.readFileSync(rawFile, 'utf-8');
            
            // Skip if it's still an error file
            if (html.includes('<!-- ERROR:')) {
                console.log(`  [${chapterNum}] ✗ Still an error file, skipping`);
                errorCount++;
                continue;
            }
            
            // Parse with Cheerio
            const $ = cheerio.load(html);
            
            // Extract chapter title
            let title = $('h3.chr-title').text().trim() || 
                       $('.chr-title').text().trim() ||
                       $('h2 a').text().trim() ||
                       $('title').text().split(' - ')[0] ||
                       `Chapter ${chapterNum}`;
            
            title = title.replace(/\s+/g, ' ').trim();
            
            // Extract chapter content
            let content = $('#chr-content').html() || 
                         $('.chr-c').html() ||
                         $('.chapter-content').html();
            
            if (!content) {
                // Fallback: try to find any content div
                content = $('div').filter(function() {
                    const text = $(this).text();
                    return text.length > 500;
                }).first().html() || "";
            }
            
            if (!content || content.length < 100) {
                throw new Error('No substantial content found');
            }
            
            // Clean content with Cheerio
            const content$ = cheerio.load(content);
            
            // Remove unwanted elements
            content$('[id^="pf-"]').remove();
            content$('.PUBFUTURE').remove();
            content$('script').remove();
            content$('style').remove();
            content$('iframe').remove();
            content$('.ads').remove();
            content$('[class*="ad"]').remove();
            content$('[id*="ad"]').remove();
            content$('.comments').remove();
            content$('.comment').remove();
            
            // Remove empty paragraphs
            content$('p').each(function() {
                const text = content$(this).text().trim();
                if (text === '' || text.length < 10) {
                    content$(this).remove();
                }
            });
            
            // Remove images that are likely ads
            content$('img').each(function() {
                const src = content$(this).attr('src') || '';
                const alt = content$(this).attr('alt') || '';
                if (src.includes('ad') || alt.includes('Ad') || src.includes('banner')) {
                    content$(this).remove();
                }
            });
            
            // Get cleaned content
            const cleanedContent = content$('body').html()?.trim() || "";
            
            // Create clean HTML
            const cleanHtml = createCleanHtml(title, chapterNum, cleanedContent, errorChapters.length);
            
            // Save cleaned file
            fs.writeFileSync(cleanFile, cleanHtml, 'utf-8');
            console.log(`  [${chapterNum}] ✓ Cleaned successfully`);
            cleanedCount++;
            
        } catch (error) {
            console.log(`  [${chapterNum}] ✗ Cleaning error: ${error.message}`);
            errorCount++;
        }
    }
    
    console.log(`\n🧹 Cleaning completed: ${cleanedCount} cleaned, ${errorCount} errors`);
}

function createCleanHtml(title, chapterNum, cleanedContent, totalChapters) {
    return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${title}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: 'Georgia', serif; line-height: 1.8; max-width: 800px; margin: 0 auto; padding: 40px 20px; background-color: #f9f9f9; color: #333; }
        .chapter-container { background-color: white; padding: 40px; border-radius: 10px; box-shadow: 0 5px 15px rgba(0,0,0,0.1); }
        h1.chapter-title { text-align: center; color: #2c3e50; border-bottom: 2px solid #3498db; padding-bottom: 15px; margin-bottom: 30px; font-size: 28px; }
        .chapter-content { font-size: 18px; text-align: justify; }
        .chapter-content p { margin-bottom: 1.5em; text-indent: 2em; line-height: 1.8; }
        .navigation { margin-top: 40px; padding-top: 20px; border-top: 1px solid #ddd; text-align: center; }
        .nav-button { display: inline-block; padding: 10px 20px; margin: 0 10px; background-color: #3498db; color: white; text-decoration: none; border-radius: 5px; font-weight: bold; transition: background-color 0.3s; }
        .nav-button:hover { background-color: #2980b9; }
        .nav-button.disabled { background-color: #95a5a6; cursor: not-allowed; }
        .chapter-info { text-align: center; color: #7f8c8d; font-style: italic; margin-bottom: 20px; font-size: 14px; }
        @media (max-width: 768px) { body { padding: 20px 10px; } .chapter-container { padding: 20px; } h1.chapter-title { font-size: 24px; } .chapter-content { font-size: 16px; } }
    </style>
</head>
<body>
    <div class="chapter-container">
        <h1 class="chapter-title">${title}</h1>
        <div class="chapter-info">Chapter ${chapterNum} | ${new Date().toLocaleDateString()}</div>
        <div class="chapter-content">${cleanedContent}</div>
        <div class="navigation">
            ${chapterNum > 1 ? `<a href="chapter_${(chapterNum - 1).toString().padStart(3, '0')}.html" class="nav-button">← Previous Chapter</a>` : `<span class="nav-button disabled">← Previous Chapter</span>`}
            ${chapterNum < 462 ? `<a href="chapter_${(chapterNum + 1).toString().padStart(3, '0')}.html" class="nav-button">Next Chapter →</a>` : `<span class="nav-button disabled">Next Chapter →</span>`}
        </div>
    </div>
</body>
</html>`;
}

async function findAndFixFromRawFiles() {
    console.log("\n" + "=".repeat(60));
    console.log("🔍 CHECKING RAW FILES FOR ERRORS");
    console.log("=".repeat(60));
    
    const RAW_DIR = "./scraped_raw";
    
    if (!fs.existsSync(RAW_DIR)) {
        console.log("Raw directory not found!");
        return;
    }
    
    const files = fs.readdirSync(RAW_DIR)
        .filter(file => file.startsWith('scraped_chapter_') && file.endsWith('.html'))
        .sort((a, b) => {
            const numA = parseInt(a.match(/\d+/)?.[0] || 0);
            const numB = parseInt(b.match(/\d+/)?.[0] || 0);
            return numA - numB;
        });
    
    const errorChapters = [];
    
    console.log(`Scanning ${files.length} raw files...`);
    
    for (const file of files) {
        const chapterNum = file.match(/\d+/)?.[0];
        if (!chapterNum) continue;
        
        try {
            const html = fs.readFileSync(path.join(RAW_DIR, file), 'utf-8');
            
            if (html.includes('<!-- ERROR:')) {
                console.log(`  [${chapterNum}] ✗ Error file detected`);
                errorChapters.push(parseInt(chapterNum));
            }
        } catch (error) {
            console.log(`  [${chapterNum}] ✗ Error reading file: ${error.message}`);
        }
    }
    
    if (errorChapters.length > 0) {
        console.log(`\nFound ${errorChapters.length} error chapters: ${errorChapters.join(', ')}`);
        
        const response = await askQuestion(`\nDo you want to fix these ${errorChapters.length} error chapters? (yes/no): `);
        
        if (response.toLowerCase() === 'yes') {
            // Delete error files
            await deleteErrorFiles(errorChapters);
            
            // Re-scrape
            await rescrapeChapters(errorChapters);
            
            // Clean
            await cleanRescrapedChapters(errorChapters);
            
            console.log("\n✅ Done! Error chapters have been fixed.");
        } else {
            console.log("Skipping fix operation.");
        }
    } else {
        console.log("🎉 No error chapters found in raw files!");
    }
}

function delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

function askQuestion(question) {
    return new Promise((resolve) => {
        const readline = require('readline').createInterface({
            input: process.stdin,
            output: process.stdout
        });
        
        readline.question(question, (answer) => {
            readline.close();
            resolve(answer);
        });
    });
}

// Run the fixer
fixErrorChapters().catch(console.error);