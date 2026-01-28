import fs from "fs";
import path from "path";
import * as cheerio from "cheerio";

async function cleanChapters() {
    const RAW_DIR = "./scraped_raw";
    const CLEAN_DIR = "./clean_chapters";
    const ERROR_LOG_FILE = "./error_chapters.log";
    
    if (!fs.existsSync(RAW_DIR)) {
        console.log("✗ No raw files found to clean!");
        return;
    }
    
    // Get all scraped files
    const files = fs.readdirSync(RAW_DIR)
        .filter(file => file.startsWith('scraped_chapter_') && file.endsWith('.html'))
        .sort((a, b) => {
            const numA = parseInt(a.match(/\d+/)?.[0] || 0);
            const numB = parseInt(b.match(/\d+/)?.[0] || 0);
            return numA - numB;
        });
    
    console.log(`Found ${files.length} raw files to clean\n`);
    
    let successCount = 0;
    let errorCount = 0;
    let errorChapters = [];
    
    for (const file of files) {
        const chapterNum = file.match(/\d+/)?.[0] || "0";
        const cleanFile = path.join(CLEAN_DIR, `chapter_${chapterNum.padStart(3, '0')}.html`);
        
        try {
            // Read raw HTML
            const html = fs.readFileSync(path.join(RAW_DIR, file), 'utf-8');
            
            // Check for error files
            if (html.includes('<!-- ERROR:')) {
                console.log(`  [${chapterNum.padStart(3, '0')}] ✗ ERROR FILE`);
                errorCount++;
                
                // Extract error message if available
                const errorMatch = html.match(/<!-- ERROR: (.+?) -->/);
                const errorMsg = errorMatch ? errorMatch[1] : "Unknown error";
                errorChapters.push({
                    chapter: parseInt(chapterNum),
                    file: file,
                    error: errorMsg,
                    type: "Scraping Error"
                });
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
                const contentDiv = $('div').filter(function() {
                    const text = $(this).text();
                    return text.length > 500 && (text.includes('Chapter') || text.includes('Translator'));
                }).first();
                
                content = contentDiv.html();
                
                if (!content) {
                    throw new Error(`No content found in HTML (file: ${file})`);
                }
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
            const cleanedContent = content$('body').html()?.trim() || "";
            
            // Check if cleaned content is too short (potential error)
            const textContent = content$('body').text().trim();
            if (textContent.length < 100) {
                throw new Error(`Content too short (${textContent.length} chars)`);
            }
            
            // Create clean HTML
            const cleanHtml = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${title}</title>
    <style>
        /* Styles remain the same */
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
            ${parseInt(chapterNum) > 1 ? `<a href="chapter_${(parseInt(chapterNum) - 1).toString().padStart(3, '0')}.html" class="nav-button">← Previous Chapter</a>` : `<span class="nav-button disabled">← Previous Chapter</span>`}
            ${parseInt(chapterNum) < files.length ? `<a href="chapter_${(parseInt(chapterNum) + 1).toString().padStart(3, '0')}.html" class="nav-button">Next Chapter →</a>` : `<span class="nav-button disabled">Next Chapter →</span>`}
        </div>
    </div>
</body>
</html>`;
            
            // Ensure clean directory exists
            if (!fs.existsSync(CLEAN_DIR)) {
                fs.mkdirSync(CLEAN_DIR, { recursive: true });
            }
            
            // Save cleaned file
            fs.writeFileSync(cleanFile, cleanHtml, 'utf-8');
            console.log(`  [${chapterNum.padStart(3, '0')}] ✓ Cleaned: chapter_${chapterNum.padStart(3, '0')}.html`);
            successCount++;
            
        } catch (error) {
            console.log(`  [${chapterNum.padStart(3, '0')}] ✗ Processing Error: ${error.message}`);
            errorCount++;
            errorChapters.push({
                chapter: parseInt(chapterNum),
                file: file,
                error: error.message,
                type: "Processing Error"
            });
        }
    }
    
    // Display error report
    displayErrorReport(errorChapters, ERROR_LOG_FILE);
    
    console.log("\n" + "=".repeat(60));
    console.log("✅ CLEANING COMPLETED!");
    console.log("=".repeat(60));
    console.log(`Successfully cleaned: ${successCount} chapters`);
    console.log(`Errors encountered:   ${errorCount} chapters`);
    console.log(`Total files processed: ${files.length} files`);
    console.log(`Clean files saved in: ${path.resolve(CLEAN_DIR)}`);
    
    // Create index.html if we have clean chapters
    if (successCount > 0) {
        createIndexHtml(successCount, errorChapters);
    }
}

function displayErrorReport(errorChapters, logFile) {
    if (errorChapters.length === 0) {
        console.log("\n🎉 No errors encountered! All chapters cleaned successfully.");
        return;
    }
    
    console.log("\n" + "=".repeat(60));
    console.log("⚠️  ERROR REPORT");
    console.log("=".repeat(60));
    
    // Sort errors by chapter number
    errorChapters.sort((a, b) => a.chapter - b.chapter);
    
    // Group errors by type
    const errorByType = {};
    errorChapters.forEach(err => {
        if (!errorByType[err.type]) errorByType[err.type] = [];
        errorByType[err.type].push(err);
    });
    
    // Display summary
    console.log(`\n📊 Summary: ${errorChapters.length} chapters with errors:`);
    Object.entries(errorByType).forEach(([type, errors]) => {
        console.log(`   ${type}: ${errors.length} chapters`);
    });
    
    // Display detailed list
    console.log("\n📋 Error Details:");
    console.log("-".repeat(60));
    
    // Display in groups of 10 for readability
    for (let i = 0; i < errorChapters.length; i += 10) {
        const batch = errorChapters.slice(i, i + 10);
        const chapterList = batch.map(err => 
            `Chapter ${err.chapter.toString().padStart(3, '0')}`
        ).join(", ");
        console.log(chapterList);
    }
    
    // Display specific error types
    console.log("\n🔍 Specific Errors:");
    console.log("-".repeat(60));
    
    const errorMessages = {};
    errorChapters.forEach(err => {
        const shortError = err.error.length > 50 ? err.error.substring(0, 50) + "..." : err.error;
        if (!errorMessages[shortError]) errorMessages[shortError] = [];
        errorMessages[shortError].push(err.chapter);
    });
    
    Object.entries(errorMessages).forEach(([error, chapters]) => {
        const chapterStr = chapters.length <= 5 ? 
            chapters.join(", ") : 
            `${chapters.slice(0, 3).join(", ")}, ... (${chapters.length} total)`;
        console.log(`• "${error}"`);
        console.log(`  Affected chapters: ${chapterStr}\n`);
    });
    
    // Save error log to file
    const logContent = `ERROR CHAPTERS LOG
Generated: ${new Date().toLocaleString()}
Total Errors: ${errorChapters.length}
Total Chapters Processed: ${errorChapters.length + (462 - 42)} // Based on your previous run

ERROR LIST:
${errorChapters.map(err => 
    `Chapter ${err.chapter.toString().padStart(3, '0')}: ${err.type} - ${err.error}`
).join('\n')}

CHAPTERS NEEDING RE-SCRAPING:
${errorChapters.map(err => err.chapter).sort((a, b) => a - b).join(', ')}

ERROR STATISTICS:
${Object.entries(errorByType).map(([type, errors]) => 
    `${type}: ${errors.length} chapters`
).join('\n')}
`;
    
    fs.writeFileSync(logFile, logContent, 'utf-8');
    console.log(`\n📝 Error log saved to: ${path.resolve(logFile)}`);
    
    // Create a simple HTML error report
    createErrorReportHTML(errorChapters);
}

function createErrorReportHTML(errorChapters) {
    const ERROR_REPORT_DIR = "./error_reports";
    if (!fs.existsSync(ERROR_REPORT_DIR)) {
        fs.mkdirSync(ERROR_REPORT_DIR, { recursive: true });
    }
    
    const htmlContent = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Error Report - Super Gene Chapters</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; line-height: 1.6; background: #f5f5f5; padding: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; border-radius: 10px; margin-bottom: 30px; text-align: center; }
        h1 { font-size: 2.5em; margin-bottom: 10px; }
        .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin: 30px 0; }
        .stat-card { background: white; padding: 20px; border-radius: 10px; box-shadow: 0 5px 15px rgba(0,0,0,0.1); text-align: center; }
        .stat-number { font-size: 2em; font-weight: bold; color: #3498db; }
        .error-list { background: white; padding: 30px; border-radius: 10px; box-shadow: 0 5px 15px rgba(0,0,0,0.1); }
        table { width: 100%; border-collapse: collapse; margin-top: 20px; }
        th { background: #3498db; color: white; padding: 15px; text-align: left; }
        td { padding: 12px 15px; border-bottom: 1px solid #ddd; }
        tr:nth-child(even) { background: #f9f9f9; }
        tr:hover { background: #f1f7fd; }
        .badge { padding: 5px 10px; border-radius: 20px; font-size: 0.8em; font-weight: bold; }
        .badge-error { background: #ff6b6b; color: white; }
        .badge-warning { background: #ffd93d; color: #333; }
        .chapter-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(80px, 1fr)); gap: 10px; margin: 20px 0; }
        .chapter-badge { background: #ff6b6b; color: white; padding: 8px; text-align: center; border-radius: 5px; font-weight: bold; }
        .controls { margin: 20px 0; display: flex; gap: 10px; flex-wrap: wrap; }
        .btn { padding: 10px 20px; background: #3498db; color: white; border: none; border-radius: 5px; cursor: pointer; }
        .btn:hover { background: #2980b9; }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Error Report</h1>
            <p>Super Gene Chapter Cleaning - ${new Date().toLocaleDateString()}</p>
        </header>
        
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-number">${errorChapters.length}</div>
                <div>Error Chapters</div>
            </div>
            <div class="stat-card">
                <div class="stat-number">${462 - 42}</div>
                <div>Success Chapters</div>
            </div>
            <div class="stat-card">
                <div class="stat-number">462</div>
                <div>Total Chapters</div>
            </div>
            <div class="stat-card">
                <div class="stat-number">${Math.round((errorChapters.length / 462) * 100)}%</div>
                <div>Error Rate</div>
            </div>
        </div>
        
        <div class="error-list">
            <h2>Error Chapters List</h2>
            
            <div class="controls">
                <button class="btn" onclick="filterErrors('all')">Show All</button>
                <button class="btn" onclick="filterErrors('Scraping Error')">Scraping Errors</button>
                <button class="btn" onclick="filterErrors('Processing Error')">Processing Errors</button>
                <button class="btn" onclick="sortTable('chapter')">Sort by Chapter</button>
                <button class="btn" onclick="sortTable('error')">Sort by Error Type</button>
            </div>
            
            <table id="errorTable">
                <thead>
                    <tr>
                        <th>Chapter</th>
                        <th>File</th>
                        <th>Error Type</th>
                        <th>Error Message</th>
                    </tr>
                </thead>
                <tbody>
                    ${errorChapters.map(err => `
                    <tr data-type="${err.type}">
                        <td><strong>${err.chapter.toString().padStart(3, '0')}</strong></td>
                        <td>${err.file}</td>
                        <td><span class="badge ${err.type === 'Scraping Error' ? 'badge-error' : 'badge-warning'}">${err.type}</span></td>
                        <td>${err.error}</td>
                    </tr>
                    `).join('')}
                </tbody>
            </table>
            
            <h3 style="margin-top: 40px;">Error Chapters Summary</h3>
            <div class="chapter-grid">
                ${errorChapters.map(err => `
                    <div class="chapter-badge" title="${err.error}">
                        ${err.chapter}
                    </div>
                `).join('')}
            </div>
            
            <div style="margin-top: 30px; padding: 20px; background: #f8f9fa; border-radius: 5px;">
                <h4>Recommended Actions:</h4>
                <ul style="margin-left: 20px; margin-top: 10px;">
                    <li>Re-scrape the error chapters (total: ${errorChapters.length})</li>
                    <li>Check if chapters ${errorChapters.map(e => e.chapter).sort((a,b)=>a-b).slice(0,5).join(', ')}${errorChapters.length > 5 ? '...' : ''} are available on the source site</li>
                    <li>Verify network connectivity for scraping</li>
                    <li>Check if the website structure has changed</li>
                </ul>
            </div>
        </div>
    </div>
    
    <script>
        function filterErrors(type) {
            const rows = document.querySelectorAll('#errorTable tbody tr');
            rows.forEach(row => {
                if (type === 'all' || row.getAttribute('data-type') === type) {
                    row.style.display = '';
                } else {
                    row.style.display = 'none';
                }
            });
        }
        
        function sortTable(by) {
            const tbody = document.querySelector('#errorTable tbody');
            const rows = Array.from(tbody.querySelectorAll('tr'));
            
            rows.sort((a, b) => {
                if (by === 'chapter') {
                    const aChap = parseInt(a.cells[0].textContent.trim());
                    const bChap = parseInt(b.cells[0].textContent.trim());
                    return aChap - bChap;
                } else if (by === 'error') {
                    const aError = a.cells[2].textContent.trim();
                    const bError = b.cells[2].textContent.trim();
                    return aError.localeCompare(bError);
                }
                return 0;
            });
            
            // Clear and re-append sorted rows
            rows.forEach(row => tbody.appendChild(row));
        }
        
        // Initialize
        document.addEventListener('DOMContentLoaded', function() {
            console.log('Error report loaded. Total errors:', ${errorChapters.length});
        });
    </script>
</body>
</html>`;
    
    const reportFile = path.join(ERROR_REPORT_DIR, "error_report.html");
    fs.writeFileSync(reportFile, htmlContent, 'utf-8');
    console.log(`📊 HTML error report created: ${path.resolve(reportFile)}`);
}
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
// Run the cleaning process
cleanChapters().catch(console.error);