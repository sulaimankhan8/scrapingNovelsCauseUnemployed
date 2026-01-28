const fs = require("fs");
const path = require("path");
const cheerio = require("cheerio");

(async () => {
    try {
        // Configuration
        const RAW_DIR = "./scraped_raw";
        const CLEAN_DIR = "./clean_chapters";
        
        // Check if raw directory exists
        if (!fs.existsSync(RAW_DIR)) {
            console.log("✗ No raw files found to clean!");
            console.log("ℹ️ Run 'node main.js' first to scrape chapters");
            return;
        }
        
        // Create clean directory
        if (!fs.existsSync(CLEAN_DIR)) {
            fs.mkdirSync(CLEAN_DIR, { recursive: true });
        }
        
        console.log("Starting to clean scraped files...");
        console.log("=" .repeat(50));
        
        // Run the cleaner
        const results = await cleanChapters(RAW_DIR, CLEAN_DIR);
        
        console.log("\n" + "=" .repeat(50));
        console.log("✅ Cleaning completed!");
        console.log(`Success: ${results.successCount} chapters`);
        console.log(`Errors: ${results.errorCount} chapters`);
        console.log(`Clean files saved in: ${path.resolve(CLEAN_DIR)}`);
        
        // Create index.html if we have clean chapters
        if (results.successCount > 0) {
            console.log("\n📚 Creating index.html...");
            await createIndexHtml(CLEAN_DIR, results.successCount);
            console.log(`📄 Open: file://${path.resolve(CLEAN_DIR, "index.html")}`);
        }
        
        console.log("\n🎉 All tasks completed!");
        
    } catch (error) {
        console.error(`Fatal error: ${error.message}`);
        console.error(error.stack);
    }
})();

async function cleanChapters(rawDir, cleanDir) {
    // Get all scraped files
    const files = fs.readdirSync(rawDir)
        .filter(file => file.startsWith('scraped_chapter_') && file.endsWith('.html'))
        .sort((a, b) => {
            const numA = parseInt(a.match(/\d+/)[0]);
            const numB = parseInt(b.match(/\d+/)[0]);
            return numA - numB;
        });
    
    console.log(`Found ${files.length} raw files to clean`);
    
    let successCount = 0;
    let errorCount = 0;
    const cleanedFiles = [];
    
    // Process files in chunks
    const chunkSize = 20;
    for (let i = 0; i < files.length; i += chunkSize) {
        const chunk = files.slice(i, i + chunkSize);
        
        console.log(`\n🧹 Cleaning batch ${Math.floor(i/chunkSize) + 1}/${Math.ceil(files.length/chunkSize)}`);
        
        for (const file of chunk) {
            const chapterNum = file.match(/\d+/)[0];
            const result = await cleanSingleFile(chapterNum, rawDir, cleanDir);
            
            if (result.success) {
                successCount++;
                cleanedFiles.push({
                    number: parseInt(chapterNum),
                    title: result.title,
                    filename: `chapter_${chapterNum.padStart(4, '0')}.html`
                });
            } else {
                errorCount++;
            }
        }
        
        // Clear memory between chunks
        if (global.gc) {
            global.gc();
        }
    }
    
    return { 
        successCount, 
        errorCount, 
        cleanedFiles: cleanedFiles.sort((a, b) => a.number - b.number) 
    };
}

async function cleanSingleFile(chapterNum, rawDir, cleanDir) {
    const rawFile = path.join(rawDir, `scraped_chapter_${chapterNum}.html`);
    const cleanFile = path.join(cleanDir, `chapter_${chapterNum.padStart(4, '0')}.html`);
    
    try {
        // Read raw HTML
        const html = fs.readFileSync(rawFile, 'utf-8');
        
        // Skip error files
        if (html.includes('<!-- ERROR:') || html.length < 500) {
            console.log(`  [${chapterNum}] ✗ Skipping error/empty file`);
            return { success: false };
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
        
        // Extract chapter content
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
        
        // Fallback
        if (!content) {
            content = $('body').html();
        }
        
        if (!content || content.length < 500) {
            throw new Error('No substantial content found');
        }
        
        // Fast cleaning using regex
        let cleanedContent = content
            .replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, '')
            .replace(/<style\b[^<]*(?:(?!<\/style>)<[^<]*)*<\/style>/gi, '')
            .replace(/<iframe\b[^<]*(?:(?!<\/iframe>)<[^<]*)*<\/iframe>/gi, '')
            .replace(/class="[^"]*ad[^"]*"/gi, '')
            .replace(/id="[^"]*ad[^"]*"/gi, '')
            .replace(/<div[^>]*class="[^"]*ads?[^"]*"[^>]*>.*?<\/div>/gis, '')
            .replace(/<ins[^>]*>.*?<\/ins>/gis, '')
            .replace(/<a[^>]*rel="[^"]*nofollow[^"]*"[^>]*>.*?<\/a>/gis, '')
            .replace(/<div[^>]*id="[^"]*comments?[^"]*"[^>]*>.*?<\/div>/gis, '')
            .replace(/<div[^>]*class="[^"]*comments?[^"]*"[^>]*>.*?<\/div>/gis, '')
            .replace(/&nbsp;/g, ' ')
            .replace(/\n\s*\n\s*\n/g, '\n\n')
            .replace(/<br\s*\/?>\s*<br\s*\/?>/gi, '</p><p>')
            .trim();
        
        // Create clean HTML
        const cleanHtml = createCleanHtml(chapterNum, title, cleanedContent);
        
        // Save cleaned file
        fs.writeFileSync(cleanFile, cleanHtml, 'utf-8');
        console.log(`  [${chapterNum}] ✓ Cleaned: "${title.substring(0, 50)}..."`);
        return { success: true, title, chapterNum };
        
    } catch (error) {
        console.log(`  [${chapterNum}] ✗ Error: ${error.message}`);
        return { success: false };
    }
}

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
            min-height: 100vh;
        }
        
        .container {
            background-color: white;
            padding: 40px;
            border-radius: 10px;
            box-shadow: 0 5px 20px rgba(0,0,0,0.1);
        }
        
        .chapter-header {
            text-align: center;
            margin-bottom: 40px;
            padding-bottom: 20px;
            border-bottom: 2px solid #e0e0e0;
        }
        
        h1 {
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
            background-color: #95a5a6;
        }
        
        @media (max-width: 768px) {
            body {
                padding: 10px;
            }
            
            .container {
                padding: 20px;
            }
            
            h1 {
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
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="chapter-header">
            <h1>${title}</h1>
            <div class="chapter-number">Chapter ${chapterNum}</div>
        </div>
        
        <div class="chapter-content">
            ${content}
        </div>
        
        <div class="chapter-navigation">
            ${prevChapter ? 
                `<a href="${prevChapter}" class="nav-button prev">← Chapter ${parseInt(chapterNum) - 1}</a>` : 
                `<span class="nav-button disabled">← Previous</span>`
            }
            
            <a href="index.html" class="nav-button">🏠 Chapters List</a>
            
            ${`<a href="${nextChapter}" class="nav-button">Chapter ${parseInt(chapterNum) + 1} →</a>`}
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
                } else if (e.key === 'ArrowRight') {
                    window.location.href = '${nextChapter}';
                } else if (e.key === 'Home' || e.key === 'h') {
                    window.location.href = 'index.html';
                }
            });
            
            // Remove any leftover script tags
            document.querySelectorAll('script').forEach(script => {
                if (!script.src) script.remove();
            });
        });
    </script>
</body>
</html>`;
}

async function createIndexHtml(cleanDir, totalChapters) {
    // Read existing cleaned files to get actual count and titles
    const files = fs.readdirSync(cleanDir)
        .filter(file => file.startsWith('chapter_') && file.endsWith('.html'))
        .sort();
    
    const chapters = [];
    
    // Extract titles from cleaned files
    for (const file of files.slice(0, totalChapters)) {
        const chapterNum = file.match(/\d+/)[0];
        const filePath = path.join(cleanDir, file);
        
        try {
            const html = fs.readFileSync(filePath, 'utf-8');
            const $ = cheerio.load(html);
            const title = $('h1').text().trim() || `Chapter ${chapterNum}`;
            
            chapters.push({
                number: parseInt(chapterNum),
                title: title.replace(/Chapter \d+[:-\s]*/, '').trim() || `Chapter ${chapterNum}`,
                filename: file
            });
        } catch (e) {
            chapters.push({
                number: parseInt(chapterNum),
                title: `Chapter ${chapterNum}`,
                filename: file
            });
        }
    }
    
    // Sort by chapter number
    chapters.sort((a, b) => a.number - b.number);
    
    const indexHtml = generateIndexHtml(chapters);
    fs.writeFileSync(path.join(cleanDir, "index.html"), indexHtml, 'utf-8');
}

function generateIndexHtml(chapters) {
    const novelTitle = "Lord of the Mysteries";
    const totalChapters = chapters.length;
    
    return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${novelTitle} - Complete Collection</title>
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
            font-weight: 700;
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
            transition: transform 0.3s ease;
        }
        
        .stat-card:hover {
            transform: translateY(-5px);
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
        
        .search-container {
            margin: 20px 30px;
            position: relative;
        }
        
        .search-input {
            width: 100%;
            padding: 15px 20px 15px 50px;
            border: 2px solid #ddd;
            border-radius: 25px;
            font-size: 16px;
            transition: border-color 0.3s;
        }
        
        .search-input:focus {
            outline: none;
            border-color: #3498db;
        }
        
        .search-icon {
            position: absolute;
            left: 20px;
            top: 50%;
            transform: translateY(-50%);
            color: #7f8c8d;
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
        
        .chapter-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
            gap: 20px;
            max-height: 600px;
            overflow-y: auto;
            padding: 10px;
        }
        
        .chapter-card {
            background: white;
            border-radius: 10px;
            overflow: hidden;
            border: 1px solid #e0e0e0;
            transition: all 0.3s ease;
        }
        
        .chapter-card:hover {
            border-color: #3498db;
            transform: translateY(-5px);
            box-shadow: 0 10px 25px rgba(52, 152, 219, 0.2);
        }
        
        .chapter-link {
            display: flex;
            padding: 20px;
            text-decoration: none;
            color: #333;
            align-items: center;
            gap: 15px;
        }
        
        .chapter-number {
            font-size: 24px;
            font-weight: bold;
            color: #3498db;
            min-width: 50px;
        }
        
        .chapter-info {
            flex: 1;
        }
        
        .chapter-title {
            font-size: 16px;
            line-height: 1.4;
            margin-bottom: 5px;
            font-weight: 600;
        }
        
        .chapter-subtitle {
            font-size: 14px;
            color: #7f8c8d;
        }
        
        .actions {
            display: flex;
            justify-content: center;
            gap: 20px;
            margin: 40px 30px;
            flex-wrap: wrap;
        }
        
        .action-button {
            padding: 15px 40px;
            background: #3498db;
            color: white;
            text-decoration: none;
            border-radius: 50px;
            font-weight: bold;
            display: flex;
            align-items: center;
            gap: 10px;
            transition: all 0.3s ease;
        }
        
        .action-button:hover {
            background: #2980b9;
            transform: translateY(-3px);
            box-shadow: 0 10px 20px rgba(52, 152, 219, 0.3);
        }
        
        .action-button.secondary {
            background: #2c3e50;
        }
        
        .action-button.secondary:hover {
            background: #1a252f;
        }
        
        footer {
            background: #2c3e50;
            color: white;
            text-align: center;
            padding: 20px;
            margin-top: 40px;
        }
        
        .footer-text {
            font-size: 14px;
            opacity: 0.8;
        }
        
        .no-results {
            text-align: center;
            padding: 40px;
            color: #7f8c8d;
            font-style: italic;
            grid-column: 1 / -1;
        }
        
        @media (max-width: 768px) {
            .container {
                border-radius: 15px;
            }
            
            header {
                padding: 30px 20px;
            }
            
            h1 {
                font-size: 28px;
            }
            
            .stats-container {
                grid-template-columns: 1fr;
                margin: 20px;
            }
            
            .chapter-grid {
                grid-template-columns: 1fr;
            }
            
            .chapters-section {
                padding: 0 20px 30px;
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
            <h1>${novelTitle}</h1>
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
            <div class="stat-card">
                <span class="stat-number">${Math.round(totalChapters * 0.7)}</span>
                <span class="stat-label">Estimated Hours</span>
            </div>
        </div>
        
        <div class="search-container">
            <div class="search-icon">🔍</div>
            <input type="text" id="chapterSearch" class="search-input" placeholder="Search chapters by number or title...">
        </div>
        
        <div class="chapters-section">
            <h2 class="section-title">Chapters List</h2>
            <div class="chapter-grid" id="chapterGrid">
                ${chapters.map(chapter => `
                    <div class="chapter-card" data-chapter="${chapter.number}" data-title="${chapter.title.toLowerCase()}">
                        <a href="${chapter.filename}" class="chapter-link">
                            <div class="chapter-number">${chapter.number}</div>
                            <div class="chapter-info">
                                <div class="chapter-title">${chapter.title.substring(0, 60)}${chapter.title.length > 60 ? '...' : ''}</div>
                                <div class="chapter-subtitle">Chapter ${chapter.number}</div>
                            </div>
                        </a>
                    </div>
                `).join('')}
            </div>
        </div>
        
        <div class="actions">
            <a href="${chapters[0]?.filename || '#'}" class="action-button">
                <span>📖 Start Reading</span>
            </a>
            <a href="https://novelbin.com/b/lord-of-the-mysteries" target="_blank" class="action-button secondary">
                <span>🌐 Visit Original Site</span>
            </a>
        </div>
        
        <footer>
            <div class="footer-text">
                <p>Offline reading collection for personal use</p>
                <p>All rights belong to the original author and publisher</p>
                <p>Generated on ${new Date().toLocaleString()}</p>
            </div>
        </footer>
    </div>
    
    <script>
        document.addEventListener('DOMContentLoaded', function() {
            const searchInput = document.getElementById('chapterSearch');
            const chapterCards = document.querySelectorAll('.chapter-card');
            const chapterGrid = document.getElementById('chapterGrid');
            
            searchInput.addEventListener('input', function() {
                const searchTerm = this.value.toLowerCase().trim();
                
                let visibleCount = 0;
                chapterCards.forEach(card => {
                    const chapterNum = card.dataset.chapter;
                    const chapterTitle = card.dataset.title;
                    const matches = chapterNum.includes(searchTerm) || 
                                   chapterTitle.includes(searchTerm);
                    
                    card.style.display = matches ? 'block' : 'none';
                    if (matches) visibleCount++;
                });
                
                // Show no results message
                let noResults = document.getElementById('noResults');
                if (visibleCount === 0) {
                    if (!noResults) {
                        noResults = document.createElement('div');
                        noResults.id = 'noResults';
                        noResults.className = 'no-results';
                        noResults.textContent = 'No chapters found matching your search.';
                        chapterGrid.appendChild(noResults);
                    }
                } else if (noResults) {
                    noResults.remove();
                }
                
                // Scroll to top when searching
                chapterGrid.scrollTop = 0;
            });
            
            // Focus search input
            searchInput.focus();
            
            // Keyboard shortcuts
            document.addEventListener('keydown', function(e) {
                if (e.ctrlKey && e.key === 'f') {
                    e.preventDefault();
                    searchInput.focus();
                    searchInput.select();
                }
                
                // Number keys to navigate
                if (e.key >= '1' && e.key <= '9' && e.ctrlKey) {
                    const chapterNum = parseInt(e.key);
                    const chapter = document.querySelector(\`.chapter-card[data-chapter="\${chapterNum}"] a\`);
                    if (chapter) {
                        window.location.href = chapter.href;
                    }
                }
            });
            
            // Smooth scroll for chapter cards
            document.querySelectorAll('.chapter-link').forEach(link => {
                link.addEventListener('click', function(e) {
                    e.preventDefault();
                    const target = this.getAttribute('href');
                    
                    // Add loading animation
                    document.body.style.opacity = '0.7';
                    document.body.style.transition = 'opacity 0.3s';
                    
                    setTimeout(() => {
                        window.location.href = target;
                    }, 300);
                });
            });
            
            // Add scroll to top button
            const scrollTopBtn = document.createElement('button');
            scrollTopBtn.innerHTML = '↑';
            scrollTopBtn.style.position = 'fixed';
            scrollTopBtn.style.bottom = '20px';
            scrollTopBtn.style.right = '20px';
            scrollTopBtn.style.width = '50px';
            scrollTopBtn.style.height = '50px';
            scrollTopBtn.style.borderRadius = '50%';
            scrollTopBtn.style.background = '#3498db';
            scrollTopBtn.style.color = 'white';
            scrollTopBtn.style.border = 'none';
            scrollTopBtn.style.fontSize = '20px';
            scrollTopBtn.style.cursor = 'pointer';
            scrollTopBtn.style.boxShadow = '0 4px 10px rgba(0,0,0,0.3)';
            scrollTopBtn.style.display = 'none';
            scrollTopBtn.style.zIndex = '1000';
            scrollTopBtn.style.transition = 'all 0.3s';
            
            scrollTopBtn.addEventListener('click', () => {
                window.scrollTo({ top: 0, behavior: 'smooth' });
            });
            
            document.body.appendChild(scrollTopBtn);
            
            // Show/hide scroll button
            window.addEventListener('scroll', () => {
                if (window.pageYOffset > 300) {
                    scrollTopBtn.style.display = 'block';
                } else {
                    scrollTopBtn.style.display = 'none';
                }
            });
            
            // Add filter by chapter range
            const filterContainer = document.createElement('div');
            filterContainer.style.margin = '10px 30px';
            filterContainer.style.display = 'flex';
            filterContainer.style.gap = '10px';
            filterContainer.style.alignItems = 'center';
            filterContainer.style.flexWrap = 'wrap';
            
            filterContainer.innerHTML = \`
                <span style="font-weight: bold;">Filter by range:</span>
                <input type="number" id="minChapter" placeholder="Min" min="1" max="\${totalChapters}" style="padding: 8px; border: 1px solid #ddd; border-radius: 4px; width: 80px;">
                <span>to</span>
                <input type="number" id="maxChapter" placeholder="Max" min="1" max="\${totalChapters}" style="padding: 8px; border: 1px solid #ddd; border-radius: 4px; width: 80px;">
                <button id="applyFilter" style="padding: 8px 16px; background: #27ae60; color: white; border: none; border-radius: 4px; cursor: pointer;">Apply</button>
                <button id="clearFilter" style="padding: 8px 16px; background: #e74c3c; color: white; border: none; border-radius: 4px; cursor: pointer;">Clear</button>
            \`;
            
            document.querySelector('.chapters-section').insertBefore(filterContainer, document.getElementById('chapterGrid'));
            
            // Filter by chapter range
            document.getElementById('applyFilter').addEventListener('click', function() {
                const min = parseInt(document.getElementById('minChapter').value) || 1;
                const max = parseInt(document.getElementById('maxChapter').value) || totalChapters;
                
                chapterCards.forEach(card => {
                    const chapterNum = parseInt(card.dataset.chapter);
                    card.style.display = (chapterNum >= min && chapterNum <= max) ? 'block' : 'none';
                });
                
                // Update visible count
                let visibleCount = 0;
                chapterCards.forEach(card => {
                    if (card.style.display !== 'none') visibleCount++;
                });
                
                // Update search to work with filtered results
                searchInput.dispatchEvent(new Event('input'));
            });
            
            document.getElementById('clearFilter').addEventListener('click', function() {
                document.getElementById('minChapter').value = '';
                document.getElementById('maxChapter').value = '';
                chapterCards.forEach(card => {
                    card.style.display = 'block';
                });
                searchInput.dispatchEvent(new Event('input'));
            });
            
            // Auto-scroll to current chapter if in URL
            const urlParams = new URLSearchParams(window.location.search);
            const scrollToChapter = urlParams.get('chapter');
            if (scrollToChapter) {
                const targetCard = document.querySelector(\`.chapter-card[data-chapter="\${scrollToChapter}"]\`);
                if (targetCard) {
                    setTimeout(() => {
                        targetCard.scrollIntoView({ behavior: 'smooth', block: 'center' });
                        targetCard.style.animation = 'highlight 2s';
                    }, 500);
                }
            }
            
            // Add highlight animation
            const style = document.createElement('style');
            style.textContent = \`
                @keyframes highlight {
                    0% { background-color: #fffacd; }
                    100% { background-color: white; }
                }
                .chapter-card.highlight {
                    animation: highlight 2s;
                }
            \`;
            document.head.appendChild(style);
        });
    </script>
</body>
</html>`;
}