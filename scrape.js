const fs = require("fs");
const path = require("path");
const { chromium } = require("playwright");
const cheerio = require("cheerio");

(async () => {
    // Configuration
    const BASE_URL = "https://novelbin.com/b/lord-of-the-mysteries/chapter-";
    const START_CHAPTER = 1;
    const END_CHAPTER = 1432;
    const RAW_DIR = "./scraped_raw";
    const CLEAN_DIR = "./clean_chapters";
    
    // Create directories
    [RAW_DIR, CLEAN_DIR].forEach(dir => {
        if (!fs.existsSync(dir)) {
            fs.mkdirSync(dir, { recursive: true });
        }
    });

    console.log(`Starting to scrape chapters ${START_CHAPTER} to ${END_CHAPTER}`);
    console.log("=" .repeat(50));

    for (let chapterNum = START_CHAPTER; chapterNum <= END_CHAPTER; chapterNum++) {
        const chapterUrl = `${BASE_URL}${chapterNum}`;
        const rawFile = path.join(RAW_DIR, `scraped_chapter_${chapterNum}.html`);
        
        console.log(`\n[${chapterNum}/${END_CHAPTER}] Scraping: ${chapterUrl}`);
        
        // Check if already scraped
        if (fs.existsSync(rawFile)) {
            console.log(`  ✓ Already exists, skipping...`);
            continue;
        }

        let retryCount = 0;
        const maxRetries = 3;
        let browser = null;
        let page = null;
        
        while (retryCount < maxRetries) {
            try {
                console.log(`  Attempt ${retryCount + 1}/${maxRetries}...`);
                
                // FRESH BROWSER FOR EACH ATTEMPT
                browser = await chromium.launch({ 
                    headless: false,  // Use visible browser
                    args: [
                        '--disable-blink-features=AutomationControlled',
                        '--disable-dev-shm-usage',
                        '--no-sandbox',
                        '--disable-web-security',
                        '--disable-features=IsolateOrigins,site-per-process',
                        '--window-size=1920,1080',
                        '--start-maximized',
                        '--disable-infobars',
                        '--disable-notifications',
                        '--disable-popup-blocking',
                        '--disable-save-password-bubble',
                        '--disable-translate',
                        '--disable-search-engine-choice-screen',
                        '--disable-component-update'
                    ]
                });

                // Random viewport dimensions
                const viewports = [
                    { width: 1920, height: 1080 },
                    { width: 1366, height: 768 },
                    { width: 1536, height: 864 },
                    { width: 1440, height: 900 }
                ];
                const randomViewport = viewports[Math.floor(Math.random() * viewports.length)];

                // Random user agents
                const userAgents = [
                    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
                    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36',
                    'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0',
                    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36 Edg/118.0.2088.76'
                ];
                const randomUserAgent = userAgents[Math.floor(Math.random() * userAgents.length)];

                const context = await browser.newContext({
                    viewport: randomViewport,
                    userAgent: randomUserAgent,
                    locale: 'en-US',
                    timezoneId: 'America/New_York',
                    permissions: ['geolocation'],
                    geolocation: { latitude: 40.7128, longitude: -74.0060 },
                    colorScheme: 'light'
                });

                // Advanced stealth scripts
                await context.addInitScript(() => {
                    // Overwrite the navigator properties
                    Object.defineProperty(navigator, 'webdriver', {
                        get: () => false,
                    });

                    Object.defineProperty(navigator, 'plugins', {
                        get: () => [1, 2, 3, 4, 5],
                    });

                    Object.defineProperty(navigator, 'languages', {
                        get: () => ['en-US', 'en'],
                    });

                    // Mock Chrome runtime
                    window.chrome = {
                        runtime: {},
                        loadTimes: function() {},
                        csi: function() {},
                        app: { isInstalled: false }
                    };

                    // Spoof platform
                    Object.defineProperty(navigator, 'platform', {
                        get: () => 'Win32',
                    });

                    // Spoof hardware concurrency
                    Object.defineProperty(navigator, 'hardwareConcurrency', {
                        get: () => 8,
                    });

                    // Spoof device memory
                    Object.defineProperty(navigator, 'deviceMemory', {
                        get: () => 8,
                    });

                    // Remove automation flags
                    window.navigator.chrome = {
                        runtime: {},
                    };

                    // Overwrite permissions
                    const originalQuery = window.navigator.permissions.query;
                    window.navigator.permissions.query = (parameters) => (
                        parameters.name === 'notifications' ?
                            Promise.resolve({ state: Notification.permission }) :
                            originalQuery(parameters)
                    );
                });

                page = await context.newPage();
                page.setDefaultTimeout(45000);
                page.setDefaultNavigationTimeout(90000);

                // Set random referrer
                const referrers = [
                    'https://www.google.com/',
                    'https://www.bing.com/',
                    'https://duckduckgo.com/',
                    'https://www.yahoo.com/',
                    'https://novelbin.com/'
                ];
                const randomReferrer = referrers[Math.floor(Math.random() * referrers.length)];
                
                // Set extra HTTP headers
                await page.setExtraHTTPHeaders({
                    'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8',
                    'Accept-Language': 'en-US,en;q=0.9',
                    'Accept-Encoding': 'gzip, deflate, br',
                    'Cache-Control': 'no-cache',
                    'Pragma': 'no-cache',
                    'Referer': randomReferrer,
                    'Upgrade-Insecure-Requests': '1',
                    'Sec-Fetch-Dest': 'document',
                    'Sec-Fetch-Mode': 'navigate',
                    'Sec-Fetch-Site': 'cross-site',
                    'Sec-Fetch-User': '?1',
                    'Sec-CH-UA': '"Not_A Brand";v="8", "Chromium";v="120"',
                    'Sec-CH-UA-Mobile': '?0',
                    'Sec-CH-UA-Platform': '"Windows"'
                });

                // Random delay before navigation (2-4 seconds)
                await delay(Math.random() * 2000 + 2000);

                // Navigate to chapter with random waitUntil
                const waitOptions = ['domcontentloaded', 'load', 'networkidle'];
                const randomWait = waitOptions[Math.floor(Math.random() * waitOptions.length)];
                
                console.log(`  Navigating with ${randomWait}...`);
                await page.goto(chapterUrl, { 
                    waitUntil: randomWait,
                    timeout: 60000,
                    referer: randomReferrer
                });

                // Check for Cloudflare challenge
                const hasCloudflare = await page.evaluate(() => {
                    return document.title.includes('Just a moment') || 
                           document.querySelector('#challenge-form') !== null ||
                           document.body.textContent.includes('Checking your browser');
                });

                if (hasCloudflare) {
                    console.log(`  ⚠️ Cloudflare challenge detected, waiting...`);
                    // Wait longer for manual solving if needed
                    await delay(10000); // 10 seconds
                    
                    // Check if still has challenge
                    const stillHasCloudflare = await page.evaluate(() => {
                        return document.title.includes('Just a moment') || 
                               document.querySelector('#challenge-form') !== null;
                    });
                    
                    if (stillHasCloudflare) {
                        // Take screenshot for debugging
                        await page.screenshot({ path: `cf_challenge_${chapterNum}.png` });
                        console.log(`  📸 Screenshot saved: cf_challenge_${chapterNum}.png`);
                        
                        // Try to wait more
                        console.log(`  ⏳ Waiting longer for challenge to resolve...`);
                        await delay(15000); // 15 more seconds
                    }
                }

                // Wait for content with multiple selectors
                console.log(`  Waiting for content...`);
                try {
                    await page.waitForSelector('#chr-content, .chr-c, .chapter-content, [id*="content"], .content', { 
                        timeout: 30000,
                        state: 'attached'
                    });
                } catch (e) {
                    // Try alternative approach
                    console.log(`  Using alternative content detection...`);
                    await page.waitForFunction(() => {
                        const content = document.querySelector('#chr-content, .chr-c, .chapter-content, [id*="content"]');
                        return content && content.textContent.length > 100;
                    }, { timeout: 20000 });
                }

                // Additional random delay to mimic reading
                await delay(Math.random() * 3000 + 2000);

                // Check if we got the content
                const hasContent = await page.evaluate(() => {
                    const content = document.querySelector('#chr-content, .chr-c, .chapter-content, [id*="content"]');
                    return content && content.textContent.length > 100;
                });
                
                if (!hasContent) {
                    // Try to extract any text content
                    const pageText = await page.evaluate(() => document.body.textContent);
                    if (pageText.length < 500) {
                        throw new Error('No substantial content found on page');
                    }
                }
                
                // Get page HTML
                const html = await page.content();
                
                // Save raw HTML
                fs.writeFileSync(rawFile, html, 'utf-8');
                console.log(`  ✓ Saved raw HTML: scraped_chapter_${chapterNum}.html`);
                
                // Close browser immediately after success
                await browser.close();
                browser = null;
                
                break; // Success, break retry loop
                
            } catch (error) {
                retryCount++;
                console.log(`  ✗ Attempt ${retryCount} failed: ${error.message}`);
                
                // Always close browser on error
                if (browser) {
                    try {
                        await browser.close();
                    } catch (e) {
                        // Ignore close errors
                    }
                    browser = null;
                }
                
                if (retryCount === maxRetries) {
                    console.log(`  ✗ Failed to scrape chapter ${chapterNum} after ${maxRetries} attempts`);
                    // Save error file
                    fs.writeFileSync(rawFile, `<!-- ERROR: ${error.message} -->`, 'utf-8');
                } else {
                    // Wait before retry with increasing delay
                    const waitTime = retryCount * 5000 + Math.random() * 3000;
                    console.log(`  ⏳ Waiting ${Math.round(waitTime/1000)}s before retry...`);
                    await delay(waitTime);
                }
            }
        }
        
        // Random delay between chapters (3-8 seconds)
        if (chapterNum < END_CHAPTER) {
            const delayTime = Math.random() * 5000 + 3000;
            console.log(`  ⏳ Waiting ${Math.round(delayTime/1000)}s before next chapter...`);
            await delay(delayTime);
        }
    }

    console.log("\n" + "=" .repeat(50));
    console.log("✅ Raw scraping completed!");
    console.log(`Raw files saved in: ${path.resolve(RAW_DIR)}`);
    
    // Start cleaning process
    console.log("\nStarting to clean scraped files...");
    await cleanChapters();
})();

// Helper function for delays
function delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

// Step 2: Cleaner Function (Keep your existing cleaning function)
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
    
    for (const file of files) {
        const chapterNum = file.match(/\d+/)[0];
        const cleanFile = path.join(CLEAN_DIR, `chapter_${chapterNum.padStart(3, '0')}.html`);
        
        try {
            // Read raw HTML
            const html = fs.readFileSync(path.join(RAW_DIR, file), 'utf-8');
            
            // Skip error files
            if (html.includes('<!-- ERROR:')) {
                console.log(`  [${chapterNum}] ✗ Skipping error file`);
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
            
            // Clean title
            title = title.replace(/\s+/g, ' ').trim();
            
            // Extract chapter content
            let content = $('#chr-content').html() || 
                         $('.chr-c').html() ||
                         $('.chapter-content').html();
            
            if (!content) {
                // Fallback: try to find any content div
                content = $('div').filter(function() {
                    const text = $(this).text();
                    return text.length > 500 && (text.includes('Chapter') || text.includes('Translator'));
                }).first().html();
            }
            
            if (!content) {
                throw new Error('No content found in HTML');
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
            content$('#fb-comment-chapter').remove();
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
            const cleanedContent = content$('body').html().trim();
            
            // Create clean HTML
            const cleanHtml = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${title}</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: 'Georgia', 'Times New Roman', serif;
            line-height: 1.8;
            max-width: 800px;
            margin: 0 auto;
            padding: 40px 20px;
            background-color: #f9f9f9;
            color: #333;
        }
        
        .chapter-container {
            background-color: white;
            padding: 40px;
            border-radius: 10px;
            box-shadow: 0 5px 15px rgba(0,0,0,0.1);
        }
        
        h1.chapter-title {
            text-align: center;
            color: #2c3e50;
            border-bottom: 2px solid #3498db;
            padding-bottom: 15px;
            margin-bottom: 30px;
            font-size: 28px;
        }
        
        .chapter-content {
            font-size: 18px;
            text-align: justify;
        }
        
        .chapter-content p {
            margin-bottom: 1.5em;
            text-indent: 2em;
            line-height: 1.8;
        }
        
        .chapter-content br {
            display: block;
            margin: 10px 0;
        }
        
        .navigation {
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid #ddd;
            text-align: center;
        }
        
        .nav-button {
            display: inline-block;
            padding: 10px 20px;
            margin: 0 10px;
            background-color: #3498db;
            color: white;
            text-decoration: none;
            border-radius: 5px;
            font-weight: bold;
            transition: background-color 0.3s;
        }
        
        .nav-button:hover {
            background-color: #2980b9;
        }
        
        .nav-button.disabled {
            background-color: #95a5a6;
            cursor: not-allowed;
        }
        
        .chapter-info {
            text-align: center;
            color: #7f8c8d;
            font-style: italic;
            margin-bottom: 20px;
            font-size: 14px;
        }
        
        @media (max-width: 768px) {
            body {
                padding: 20px 10px;
            }
            
            .chapter-container {
                padding: 20px;
            }
            
            h1.chapter-title {
                font-size: 24px;
            }
            
            .chapter-content {
                font-size: 16px;
            }
        }
    </style>
</head>
<body>
    <div class="chapter-container">
        <h1 class="chapter-title">${title}</h1>
        
        <div class="chapter-info">
            Chapter ${chapterNum} | ${new Date().toLocaleDateString()}
        </div>
        
        <div class="chapter-content">
            ${cleanedContent}
        </div>
        
        <div class="navigation">
            ${parseInt(chapterNum) > 1 ? 
                `<a href="chapter_${(parseInt(chapterNum) - 1).toString().padStart(3, '0')}.html" class="nav-button">← Previous Chapter</a>` : 
                `<span class="nav-button disabled">← Previous Chapter</span>`
            }
            
            ${parseInt(chapterNum) < 50 ? 
                `<a href="chapter_${(parseInt(chapterNum) + 1).toString().padStart(3, '0')}.html" class="nav-button">Next Chapter →</a>` : 
                `<span class="nav-button disabled">Next Chapter →</span>`
            }
        </div>
    </div>
    
    <script>
        // Add smooth scrolling
        document.addEventListener('DOMContentLoaded', function() {
            // Remove any leftover script tags
            document.querySelectorAll('script').forEach(script => script.remove());
            
            // Add chapter navigation with keyboard
            document.addEventListener('keydown', function(e) {
                if (e.key === 'ArrowLeft' && ${parseInt(chapterNum) > 1}) {
                    window.location.href = 'chapter_${(parseInt(chapterNum) - 1).toString().padStart(3, '0')}.html';
                } else if (e.key === 'ArrowRight' && ${parseInt(chapterNum) < 50}) {
                    window.location.href = 'chapter_${(parseInt(chapterNum) + 1).toString().padStart(3, '0')}.html';
                }
            });
            
            // Add reading progress
            const updateProgress = () => {
                const scrollTop = window.pageYOffset || document.documentElement.scrollTop;
                const scrollHeight = document.documentElement.scrollHeight - document.documentElement.clientHeight;
                const progress = scrollHeight > 0 ? Math.round((scrollTop / scrollHeight) * 100) : 0;
                
                let progressBar = document.getElementById('reading-progress');
                if (!progressBar) {
                    progressBar = document.createElement('div');
                    progressBar.id = 'reading-progress';
                    progressBar.style.position = 'fixed';
                    progressBar.style.top = '0';
                    progressBar.style.left = '0';
                    progressBar.style.width = '100%';
                    progressBar.style.height = '3px';
                    progressBar.style.backgroundColor = '#3498db';
                    progressBar.style.zIndex = '1000';
                    progressBar.style.transformOrigin = '0 0';
                    document.body.appendChild(progressBar);
                }
                
                progressBar.style.transform = \`scaleX(\${progress / 100})\`;
            };
            
            window.addEventListener('scroll', updateProgress);
            updateProgress();
        });
    </script>
</body>
</html>`;
            
            // Save cleaned file
            fs.writeFileSync(cleanFile, cleanHtml, 'utf-8');
            console.log(`  [${chapterNum}] ✓ Cleaned: chapter_${chapterNum.padStart(3, '0')}.html`);
            successCount++;
            
        } catch (error) {
            console.log(`  [${chapterNum}] ✗ Error: ${error.message}`);
            errorCount++;
        }
    }
    
    console.log("\n" + "=" .repeat(50));
    console.log("✅ Cleaning completed!");
    console.log(`Success: ${successCount} chapters`);
    console.log(`Errors: ${errorCount} chapters`);
    console.log(`Clean files saved in: ${path.resolve(CLEAN_DIR)}`);
    
    // Create index.html if we have clean chapters
    if (successCount > 0) {
        createIndexHtml(successCount);
    }
}

// Step 3: Create index.html
function createIndexHtml(totalChapters) {
    const CLEAN_DIR = "./clean_chapters";
    
    const indexHtml = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Super Gene - Complete Collection</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            line-height: 1.6;
            max-width: 1000px;
            margin: 0 auto;
            padding: 30px 20px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
        }
        
        .container {
            background-color: white;
            border-radius: 15px;
            padding: 40px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
        }
        
        header {
            text-align: center;
            margin-bottom: 40px;
            padding-bottom: 20px;
            border-bottom: 2px solid #f0f0f0;
        }
        
        h1 {
            color: #2c3e50;
            font-size: 36px;
            margin-bottom: 10px;
        }
        
        .subtitle {
            color: #7f8c8d;
            font-size: 18px;
        }
        
        .stats {
            display: flex;
            justify-content: center;
            gap: 30px;
            margin: 30px 0;
            flex-wrap: wrap;
        }
        
        .stat-box {
            background-color: #f8f9fa;
            padding: 20px;
            border-radius: 10px;
            text-align: center;
            min-width: 150px;
            box-shadow: 0 5px 15px rgba(0,0,0,0.1);
        }
        
        .stat-number {
            font-size: 32px;
            font-weight: bold;
            color: #3498db;
            display: block;
        }
        
        .stat-label {
            font-size: 14px;
            color: #7f8c8d;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        
        .chapter-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
            gap: 15px;
            margin-top: 30px;
        }
        
        .chapter-card {
            background-color: white;
            border: 2px solid #e0e0e0;
            border-radius: 8px;
            padding: 20px 10px;
            text-align: center;
            text-decoration: none;
            color: #2c3e50;
            font-weight: bold;
            transition: all 0.3s ease;
            box-shadow: 0 3px 10px rgba(0,0,0,0.1);
        }
        
        .chapter-card:hover {
            transform: translateY(-5px);
            border-color: #3498db;
            box-shadow: 0 10px 20px rgba(52, 152, 219, 0.2);
            color: #3498db;
        }
        
        .chapter-number {
            font-size: 24px;
            display: block;
            margin-bottom: 5px;
        }
        
        .chapter-label {
            font-size: 12px;
            color: #7f8c8d;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        
        .actions {
            display: flex;
            justify-content: center;
            gap: 20px;
            margin-top: 40px;
            flex-wrap: wrap;
        }
        
        .action-button {
            padding: 12px 30px;
            background-color: #3498db;
            color: white;
            text-decoration: none;
            border-radius: 50px;
            font-weight: bold;
            transition: all 0.3s ease;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        
        .action-button:hover {
            background-color: #2980b9;
            transform: translateY(-2px);
            box-shadow: 0 10px 20px rgba(52, 152, 219, 0.3);
        }
        
        .action-button.secondary {
            background-color: #95a5a6;
        }
        
        .action-button.secondary:hover {
            background-color: #7f8c8d;
        }
        
        footer {
            text-align: center;
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid #eee;
            color: #95a5a6;
            font-size: 14px;
        }
        
        @media (max-width: 768px) {
            .container {
                padding: 20px;
            }
            
            h1 {
                font-size: 28px;
            }
            
            .chapter-grid {
                grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
            }
            
            .actions {
                flex-direction: column;
                align-items: center;
            }
            
            .action-button {
                width: 100%;
                max-width: 300px;
                justify-content: center;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Super Gene</h1>
            <p class="subtitle">Complete Novel Collection - ${totalChapters} Chapters</p>
        </header>
        
        <div class="stats">
            <div class="stat-box">
                <span class="stat-number">${totalChapters}</span>
                <span class="stat-label">Chapters</span>
            </div>
            <div class="stat-box">
                <span class="stat-number">${Math.round(totalChapters * 5)}</span>
                <span class="stat-label">Estimated Pages</span>
            </div>
            <div class="stat-box">
                <span class="stat-number">${new Date().getFullYear()}</span>
                <span class="stat-label">Collection Year</span>
            </div>
        </div>
        
        <h2 style="text-align: center; margin: 30px 0 20px 0; color: #2c3e50;">Chapter List</h2>
        
        <div class="chapter-grid">
            ${Array.from({length: totalChapters}, (_, i) => `
                <a href="chapter_${(i + 1).toString().padStart(3, '0')}.html" class="chapter-card">
                    <span class="chapter-number">${i + 1}</span>
                    <span class="chapter-label">Chapter</span>
                </a>
            `).join('')}
        </div>
        
        <div class="actions">
            <a href="chapter_001.html" class="action-button">
                <span>Start Reading</span>
            </a>
            <a href="https://novelbin.com/b/super-gene" target="_blank" class="action-button secondary">
                <span>Visit Original Site</span>
            </a>
        </div>
        
        <footer>
            <p>This is an offline reading collection. All rights belong to the original author.</p>
            <p>Generated on ${new Date().toLocaleString()}</p>
        </footer>
    </div>
    
    <script>
        // Add search functionality
        document.addEventListener('DOMContentLoaded', function() {
            // Create search box
            const searchBox = document.createElement('div');
            searchBox.style.margin = '20px auto';
            searchBox.style.maxWidth = '400px';
            searchBox.innerHTML = \`
                <input type="text" id="chapterSearch" placeholder="Search for chapter number..." 
                       style="width: 100%; padding: 12px; border: 2px solid #ddd; border-radius: 25px; 
                              font-size: 16px; outline: none; transition: border-color 0.3s;">
            \`;
            
            document.querySelector('.stats').after(searchBox);
            
            const searchInput = document.getElementById('chapterSearch');
            const chapterCards = document.querySelectorAll('.chapter-card');
            
            searchInput.addEventListener('input', function() {
                const searchTerm = this.value.toLowerCase().trim();
                
                chapterCards.forEach(card => {
                    const chapterNum = card.querySelector('.chapter-number').textContent;
                    const matches = chapterNum.includes(searchTerm);
                    card.style.display = matches ? 'block' : 'none';
                });
            });
            
            // Add keyboard shortcuts
            document.addEventListener('keydown', function(e) {
                if (e.ctrlKey && e.key === 'f') {
                    e.preventDefault();
                    searchInput.focus();
                }
                
                // Number keys to navigate to chapters
                if (e.key >= '1' && e.key <= '9' && e.ctrlKey) {
                    const chapterNum = parseInt(e.key);
                    if (chapterNum <= ${totalChapters}) {
                        window.location.href = \`chapter_\${chapterNum.toString().padStart(3, '0')}.html\`;
                    }
                }
            });
            
            // Add smooth scrolling for chapter navigation
            const links = document.querySelectorAll('a[href^="chapter_"]');
            links.forEach(link => {
                link.addEventListener('click', function(e) {
                    if (this.getAttribute('href').startsWith('chapter_')) {
                        e.preventDefault();
                        document.body.style.opacity = '0.7';
                        document.body.style.transition = 'opacity 0.3s';
                        setTimeout(() => {
                            window.location.href = this.getAttribute('href');
                        }, 300);
                    }
                });
            });
        });
    </script>
</body>
</html>`;
    
    fs.writeFileSync(path.join(CLEAN_DIR, "index.html"), indexHtml, 'utf-8');
    console.log(`\n📚 Created index.html with ${totalChapters} chapters`);
    console.log(`Open: file://${path.resolve(CLEAN_DIR, "index.html")}`);
}