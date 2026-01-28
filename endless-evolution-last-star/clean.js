import fs from "fs";
import path from "path";
import * as cheerio from "cheerio";

async function cleanChapters(novelId, chaptersData, endChapter) {
    const RAW_DIR = `./scraped_raw_${novelId}`;
    const CLEAN_DIR = `./clean_chapters_${novelId}`;
    
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
        const chapterNum = parseInt(file.match(/\d+/)[0]);
        if (chapterNum > endChapter) continue;
        
        const chapterInfo = chaptersData[chapterNum - 1];
        const cleanFile = path.join(CLEAN_DIR, `chapter_${chapterNum.toString().padStart(3, '0')}.html`);
        
        try {
            // Read raw HTML
            const html = fs.readFileSync(path.join(RAW_DIR, file), 'utf-8');
            
            // Skip error files
            if (html.includes('ERROR: Failed to scrape') || html.includes('<!-- ERROR:')) {
                console.log(`  [${chapterNum}] ✗ Skipping error file`);
                errorCount++;
                continue;
            }
            
            // Parse with Cheerio
            const $ = cheerio.load(html);
            
            // Extract chapter title
            let title = chapterInfo ? chapterInfo.title : `Chapter ${chapterNum}`;
            
            // Try to get title from page
            if (!title || title === `Chapter ${chapterNum}`) {
                title = $('h3.chr-title').text().trim() || 
                       $('.chr-title').text().trim() ||
                       $('h1').first().text().trim() ||
                       $('title').text().split(' - ')[0] ||
                       `Chapter ${chapterNum}`;
            }
            
            // Clean title
            title = title.replace(/\s+/g, ' ').trim();
            
            // Extract chapter content - try multiple selectors
            let content = null;
            const contentSelectors = [
                '#chr-content',
                '.chr-c',
                '.chapter-content',
                '.content-area',
                '.reading-content',
                '.text-left',
                '.entry-content',
                '.novel-content',
                '.chapter-text',
                '.chapter-body',
                '#novel-content',
                '#chapter-content',
                'article'
            ];
            
            for (const selector of contentSelectors) {
                if ($(selector).length) {
                    content = $(selector).html();
                    if (content && content.length > 500) {
                        console.log(`  [${chapterNum}] Using selector: ${selector}`);
                        break;
                    }
                }
            }
            
            // Fallback: get body content
            if (!content) {
                content = $('body').html();
            }
            
            if (!content || content.length < 500) {
                throw new Error('No substantial content found in HTML');
            }
            
            // Clean content with Cheerio
            const content$ = cheerio.load(content);
            
            // Remove unwanted elements
            const unwantedSelectors = [
                'script',
                'style',
                'iframe',
                '.ads',
                '[class*="ad"]',
                '[id*="ad"]',
                '.comments',
                '.comment',
                '.social-share',
                '.share-buttons',
                '.next-prev',
                '.navigation',
                '.sidebar',
                '.widget',
                '.related-posts',
                '.author-box',
                '.post-tags',
                '.post-meta',
                '.post-footer',
                '.rate-box',
                '.chapter-nav',
                '.chapter-controls',
                '.advertisement',
                '.banner',
                '.sponsored',
                '.promo'
            ];
            
            unwantedSelectors.forEach(selector => {
                content$(selector).remove();
            });
            
            // Remove empty paragraphs and divs
            content$('p, div').each(function() {
                const text = content$(this).text().trim();
                if (text === '' || text.length < 20) {
                    content$(this).remove();
                }
            });
            
            // Get cleaned content
            let cleanedContent = content$('body').html().trim();
            
            // Additional cleaning
            cleanedContent = cleanedContent
                .replace(/\n\s*\n\s*\n/g, '\n\n')
                .replace(/<p>\s*<br>\s*<\/p>/g, '')
                .replace(/<div>\s*<br>\s*<\/div>/g, '')
                .replace(/&nbsp;/g, ' ')
                .trim();
            
            // Create clean HTML with navigation
            const prevChapter = chapterNum > 1 ? 
                `chapter_${(chapterNum - 1).toString().padStart(3, '0')}.html` : null;
            const nextChapter = chapterNum < endChapter ? 
                `chapter_${(chapterNum + 1).toString().padStart(3, '0')}.html` : null;
            
            const cleanHtml = createCleanHtml(chapterNum, title, cleanedContent, prevChapter, nextChapter);
            
            // Save cleaned file
            fs.writeFileSync(cleanFile, cleanHtml, 'utf-8');
            console.log(`  [${chapterNum}] ✓ Cleaned: chapter_${chapterNum.toString().padStart(3, '0')}.html`);
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
        createIndexHtml(novelId, chaptersData.slice(0, endChapter), successCount);
    }
}

// Function to create clean HTML structure
function createCleanHtml(chapterNum, title, content, prevChapter, nextChapter) {
    return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Chapter ${chapterNum}: ${title}</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: 'Georgia', serif;
            line-height: 1.8;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
            color: #333;
        }
        
        .novel-container {
            background-color: white;
            padding: 40px;
            border-radius: 10px;
            box-shadow: 0 5px 20px rgba(0,0,0,0.1);
            min-height: 100vh;
        }
        
        .chapter-header {
            text-align: center;
            margin-bottom: 40px;
            padding-bottom: 20px;
            border-bottom: 2px solid #e0e0e0;
        }
        
        .chapter-title {
            color: #2c3e50;
            font-size: 28px;
            margin-bottom: 10px;
            line-height: 1.3;
        }
        
        .chapter-number {
            color: #7f8c8d;
            font-size: 16px;
            font-weight: normal;
        }
        
        .chapter-content {
            font-size: 18px;
            text-align: justify;
        }
        
        .chapter-content p {
            margin-bottom: 1.5em;
            text-indent: 2em;
            line-height: 1.8;
            text-align: justify;
        }
        
        .chapter-content p:first-child {
            text-indent: 0;
        }
        
        .chapter-navigation {
            display: flex;
            justify-content: space-between;
            margin-top: 50px;
            padding-top: 20px;
            border-top: 1px solid #e0e0e0;
        }
        
        .nav-button {
            display: inline-flex;
            align-items: center;
            padding: 12px 24px;
            background-color: #3498db;
            color: white;
            text-decoration: none;
            border-radius: 5px;
            font-weight: bold;
            transition: all 0.3s ease;
        }
        
        .nav-button:hover {
            background-color: #2980b9;
            transform: translateY(-2px);
            box-shadow: 0 5px 15px rgba(52, 152, 219, 0.3);
        }
        
        .nav-button.prev {
            background-color: #2c3e50;
        }
        
        .nav-button.prev:hover {
            background-color: #1a252f;
        }
        
        .nav-button.disabled {
            background-color: #95a5a6;
            cursor: not-allowed;
            opacity: 0.6;
        }
        
        .nav-button.disabled:hover {
            transform: none;
            box-shadow: none;
            background-color: #95a5a6;
        }
        
        .home-button {
            display: block;
            text-align: center;
            margin: 30px auto;
            padding: 10px 20px;
            background-color: #27ae60;
            color: white;
            text-decoration: none;
            border-radius: 5px;
            width: fit-content;
        }
        
        .chapter-info {
            text-align: center;
            color: #7f8c8d;
            font-style: italic;
            margin-bottom: 30px;
            font-size: 14px;
        }
        
        @media (max-width: 768px) {
            body {
                padding: 10px;
            }
            
            .novel-container {
                padding: 20px;
            }
            
            .chapter-title {
                font-size: 22px;
            }
            
            .chapter-content {
                font-size: 16px;
            }
            
            .chapter-content p {
                text-indent: 1.5em;
            }
            
            .nav-button {
                padding: 10px 15px;
                font-size: 14px;
            }
            
            .chapter-navigation {
                flex-direction: column;
                gap: 15px;
            }
        }
    </style>
</head>
<body>
    <div class="novel-container">
        <div class="chapter-header">
            <h1 class="chapter-title">${title}</h1>
            <div class="chapter-number">Chapter ${chapterNum}</div>
        </div>
        
        <div class="chapter-info">
            Downloaded from NovelBin • ${new Date().toLocaleDateString()}
        </div>
        
        <div class="chapter-content">
            ${content}
        </div>
        
        <div class="chapter-navigation">
            ${prevChapter ? 
                `<a href="${prevChapter}" class="nav-button prev">← Chapter ${chapterNum - 1}</a>` : 
                `<span class="nav-button disabled">← Previous</span>`
            }
            
            <a href="index.html" class="nav-button">🏠 Chapters List</a>
            
            ${nextChapter ? 
                `<a href="${nextChapter}" class="nav-button">Chapter ${chapterNum + 1} →</a>` : 
                `<span class="nav-button disabled">Next →</span>`
            }
        </div>
    </div>
    
    <script>
        document.addEventListener('DOMContentLoaded', function() {
            // Add reading progress bar
            const progressBar = document.createElement('div');
            progressBar.style.position = 'fixed';
            progressBar.style.top = '0';
            progressBar.style.left = '0';
            progressBar.style.width = '100%';
            progressBar.style.height = '3px';
            progressBar.style.backgroundColor = '#3498db';
            progressBar.style.transform = 'scaleX(0)';
            progressBar.style.transformOrigin = '0 0';
            progressBar.style.transition = 'transform 0.1s';
            progressBar.style.zIndex = '10000';
            document.body.appendChild(progressBar);
            
            // Update progress
            function updateProgress() {
                const scrollTop = window.pageYOffset || document.documentElement.scrollTop;
                const scrollHeight = document.documentElement.scrollHeight - document.documentElement.clientHeight;
                const progress = scrollHeight > 0 ? (scrollTop / scrollHeight) : 0;
                progressBar.style.transform = 'scaleX(' + progress + ')';
            }
            
            window.addEventListener('scroll', updateProgress);
            updateProgress();
            
            // Keyboard navigation
            document.addEventListener('keydown', function(e) {
                if (e.key === 'ArrowLeft' && ${prevChapter ? 'true' : 'false'}) {
                    window.location.href = '${prevChapter || '#'}';
                } else if (e.key === 'ArrowRight' && ${nextChapter ? 'true' : 'false'}) {
                    window.location.href = '${nextChapter || '#'}';
                } else if (e.key === 'Home' || e.key === 'h') {
                    window.location.href = 'index.html';
                }
            });
            
            // Remove any leftover script tags from original content
            document.querySelectorAll('script').forEach(script => script.remove());
            
            // Smooth scroll to top on load
            window.scrollTo({ top: 0, behavior: 'smooth' });
        });
    </script>
</body>
</html>`;
}

// Create index.html
function createIndexHtml(novelId, chaptersData, totalChapters) {
    const CLEAN_DIR = `./clean_chapters_${novelId}`;
    
    // Format novel ID for display
    const displayTitle = novelId.split('-')
        .map(word => word.charAt(0).toUpperCase() + word.slice(1))
        .join(' ');
    
    const indexHtml = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${displayTitle} - Complete Collection</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            line-height: 1.6;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
            color: #333;
        }
        
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background-color: white;
            border-radius: 20px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            overflow: hidden;
        }
        
        header {
            background: linear-gradient(135deg, #2c3e50, #34495e);
            color: white;
            padding: 40px 30px;
            text-align: center;
        }
        
        h1 {
            font-size: 36px;
            margin-bottom: 10px;
        }
        
        .subtitle {
            font-size: 18px;
            opacity: 0.9;
            margin-bottom: 20px;
        }
        
        .stats-container {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin: 30px;
        }
        
        .stat-card {
            background: #ecf0f1;
            padding: 20px;
            border-radius: 10px;
            text-align: center;
            border-left: 5px solid #3498db;
        }
        
        .stat-number {
            font-size: 32px;
            font-weight: bold;
            color: #3498db;
            display: block;
            margin-bottom: 5px;
        }
        
        .stat-label {
            font-size: 14px;
            color: #2c3e50;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        
        .chapters-section {
            padding: 0 30px 40px;
        }
        
        .section-title {
            font-size: 24px;
            color: #2c3e50;
            margin-bottom: 20px;
            padding-bottom: 10px;
            border-bottom: 2px solid #ddd;
        }
        
        .chapter-list {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
            gap: 15px;
            max-height: 600px;
            overflow-y: auto;
            padding: 10px;
            border: 1px solid #ddd;
            border-radius: 10px;
            background: #f9f9f9;
        }
        
        .chapter-item {
            background: white;
            border-radius: 8px;
            overflow: hidden;
            border: 1px solid #ddd;
        }
        
        .chapter-link {
            display: block;
            padding: 15px;
            text-decoration: none;
            color: #333;
            text-align: center;
        }
        
        .chapter-number {
            font-size: 20px;
            font-weight: bold;
            color: #3498db;
            margin-bottom: 5px;
            display: block;
        }
        
        .chapter-title {
            font-size: 14px;
            line-height: 1.4;
            display: block;
        }
        
        .actions {
            display: flex;
            justify-content: center;
            gap: 20px;
            margin: 30px;
            flex-wrap: wrap;
        }
        
        .action-button {
            padding: 12px 30px;
            background: #3498db;
            color: white;
            text-decoration: none;
            border-radius: 50px;
            font-weight: bold;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        
        .action-button:hover {
            background: #2980b9;
        }
        
        .action-button.secondary {
            background: #2c3e50;
        }
        
        .action-button.secondary:hover {
            background: #1a252f;
        }
        
        .search-box {
            margin: 30px;
        }
        
        .search-input {
            width: 100%;
            padding: 12px 20px;
            border: 2px solid #ddd;
            border-radius: 25px;
            font-size: 16px;
        }
        
        footer {
            background: #2c3e50;
            color: white;
            text-align: center;
            padding: 20px;
            margin-top: 40px;
        }
        
        @media (max-width: 768px) {
            .container {
                border-radius: 15px;
            }
            
            header {
                padding: 30px 20px;
            }
            
            .stats-container {
                grid-template-columns: 1fr;
                margin: 20px;
            }
            
            .chapters-section {
                padding: 0 20px 30px;
            }
            
            .chapter-list {
                grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
            }
            
            .actions {
                flex-direction: column;
                align-items: center;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>${displayTitle}</h1>
            <div class="subtitle">Complete Novel Collection • ${totalChapters} Chapters</div>
        </header>
        
        <div class="stats-container">
            <div class="stat-card">
                <span class="stat-number">${totalChapters}</span>
                <span class="stat-label">Total Chapters</span>
            </div>
            <div class="stat-card">
                <span class="stat-number">${new Date().getFullYear()}</span>
                <span class="stat-label">Collection Year</span>
            </div>
        </div>
        
        <div class="search-box">
            <input type="text" id="chapterSearch" class="search-input" placeholder="Search chapters...">
        </div>
        
        <div class="chapters-section">
            <h2 class="section-title">Chapters List</h2>
            <div class="chapter-list" id="chapterList">
                ${chaptersData.map((chapter, index) => `
                    <div class="chapter-item">
                        <a href="chapter_${(index + 1).toString().padStart(3, '0')}.html" class="chapter-link">
                            <span class="chapter-number">${index + 1}</span>
                            <span class="chapter-title">${chapter.title}</span>
                        </a>
                    </div>
                `).join('')}
            </div>
        </div>
        
        <div class="actions">
            <a href="chapter_001.html" class="action-button">
                <span>Start Reading</span>
            </a>
            <a href="https://novelbin.com/" target="_blank" class="action-button secondary">
                <span>Visit Original Site</span>
            </a>
</div>
        
        <footer>
            <p>Offline reading collection for personal use</p>
            <p>Generated on ${new Date().toLocaleString()}</p>
        </footer>
    </div>
    
    <script>
        document.addEventListener('DOMContentLoaded', function() {
            const searchInput = document.getElementById('chapterSearch');
            const chapterItems = document.querySelectorAll('.chapter-item');
            
            searchInput.addEventListener('input', function() {
                const searchTerm = this.value.toLowerCase().trim();
                
                chapterItems.forEach(item => {
                    const text = item.textContent.toLowerCase();
                    item.style.display = text.includes(searchTerm) ? 'block' : 'none';
                });
            });
            
            // Focus search input
            searchInput.focus();
        });
    </script>
</body>
</html>`;
    
    fs.writeFileSync(path.join(CLEAN_DIR, "index.html"), indexHtml, 'utf-8');
    console.log(`\n📚 Created index.html with ${totalChapters} chapters`);
    console.log(`\n🎉 COMPLETED!`);
    console.log(`📁 Clean files: ${path.resolve(CLEAN_DIR)}`);
    console.log(`📄 Open: file://${path.resolve(CLEAN_DIR, "index.html")}`);
}

// MAIN EXECUTION
try {
    // Load chapters from in current directory
    const cleanJsonPath = "./chapters.json"; // Changed from "./endless-evolution-last-star/clean.json"
    
    if (!fs.existsSync(cleanJsonPath)) {
        console.error(`Error: File not found: ${cleanJsonPath}`);
        console.log("Looking for clean.json in current directory...");
        
        // Try to find the file
        const files = fs.readdirSync(".");
        const jsonFiles = files.filter(f => f.includes('.json'));
        console.log("Available JSON files:", jsonFiles);
        
        if (jsonFiles.length > 0) {
            console.log("\nTry using one of these files instead:");
            jsonFiles.forEach(f => console.log(`  - ${f}`));
        }
        
        process.exit(1);
    }
    
    const cleanData = JSON.parse(fs.readFileSync(cleanJsonPath, "utf-8"));
    const novelId = cleanData.novel_id || "endless-evolution-last-star";
    const chaptersData = cleanData.chapters || [];
    const endChapter = 783; // or chaptersData.length if you want all
    
    console.log(`Novel ID: ${novelId}`);
    console.log(`Chapters in JSON: ${chaptersData.length}`);
    console.log(`Will clean up to chapter: ${endChapter}`);
    
    await cleanChapters(novelId, chaptersData, endChapter);
    
} catch (error) {
    console.error(`Fatal error: ${error.message}`);
    console.error(error.stack);
}