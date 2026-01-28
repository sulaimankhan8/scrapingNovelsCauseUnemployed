const fs = require('fs');
const path = require('path');
const puppeteer = require('puppeteer');

async function convertToPDF() {
    console.log('📚 Converting HTML chapters to PDF...\n');
    
    // Configuration
    const NOVEL_ID = 'endless-evolution-last-star';
    const CLEAN_DIR = `./clean_chapters_${NOVEL_ID}`;
    const OUTPUT_PDF = `./${NOVEL_ID.replace(/-/g, '_')}_complete.pdf`;
    
    // Check if clean directory exists
    if (!fs.existsSync(CLEAN_DIR)) {
        console.error(`❌ Clean directory not found: ${CLEAN_DIR}`);
        console.log('Please run the scraper first to generate clean HTML files.');
        return;
    }
    
    // Get all chapter files in order
    const chapterFiles = fs.readdirSync(CLEAN_DIR)
        .filter(file => file.startsWith('chapter_') && file.endsWith('.html'))
        .sort((a, b) => {
            const numA = parseInt(a.match(/\d+/)[0]);
            const numB = parseInt(b.match(/\d+/)[0]);
            return numA - numB;
        });
    
    if (chapterFiles.length === 0) {
        console.error('❌ No chapter files found in clean directory');
        return;
    }
    
    console.log(`Found ${chapterFiles.length} chapter files`);
    console.log('Starting PDF conversion...\n');
    
    let browser;
    try {
        // Launch browser
        browser = await puppeteer.launch({
            headless: 'new',
            args: ['--no-sandbox', '--disable-setuid-sandbox']
        });
        
        const page = await browser.newPage();
        
        // Combine all chapters into one HTML
        console.log('📝 Combining chapters...');
        
        let combinedHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${NOVEL_ID.replace(/-/g, ' ').toUpperCase()} - Complete Novel</title>
    <style>
        @import url('https://fonts.googleapis.com/css2?family=Merriweather:ital,wght@0,300;0,400;0,700;1,300;1,400&family=Roboto:wght@300;400;500&display=swap');
        
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: 'Merriweather', Georgia, serif;
            line-height: 1.8;
            color: #333;
            background-color: #fff;
            padding: 0;
            margin: 0;
            font-size: 16px;
        }
        
        /* Page break for printing/PDF */
        .chapter {
            page-break-before: always;
            padding: 60px 50px;
            min-height: 100vh;
            position: relative;
        }
        
        .chapter:first-child {
            page-break-before: auto;
        }
        
        /* Table of Contents */
        .toc-page {
            page-break-before: always;
            padding: 80px 50px;
            background-color: #f8f8f8;
        }
        
        .toc-title {
            font-family: 'Roboto', sans-serif;
            font-size: 42px;
            font-weight: 300;
            text-align: center;
            color: #2c3e50;
            margin-bottom: 40px;
            padding-bottom: 20px;
            border-bottom: 2px solid #3498db;
        }
        
        .toc-subtitle {
            font-family: 'Roboto', sans-serif;
            font-size: 18px;
            text-align: center;
            color: #7f8c8d;
            margin-bottom: 60px;
        }
        
        .toc-list {
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            background: white;
            border-radius: 10px;
            box-shadow: 0 5px 15px rgba(0,0,0,0.1);
        }
        
        .toc-item {
            margin-bottom: 15px;
            padding-bottom: 15px;
            border-bottom: 1px solid #eee;
        }
        
        .toc-link {
            display: flex;
            align-items: center;
            text-decoration: none;
            color: #333;
            transition: all 0.3s;
        }
        
        .toc-link:hover {
            color: #3498db;
        }
        
        .toc-number {
            font-family: 'Roboto', sans-serif;
            font-weight: 500;
            min-width: 50px;
            color: #3498db;
        }
        
        .toc-chapter-title {
            flex: 1;
        }
        
        /* Chapter Styles */
        .chapter-header {
            margin-bottom: 60px;
            padding-bottom: 30px;
            border-bottom: 2px solid #e0e0e0;
            text-align: center;
        }
        
        .chapter-title {
            font-family: 'Roboto', sans-serif;
            font-size: 32px;
            font-weight: 400;
            color: #2c3e50;
            margin-bottom: 15px;
            line-height: 1.3;
        }
        
        .chapter-number {
            font-family: 'Roboto', sans-serif;
            font-size: 18px;
            color: #7f8c8d;
            font-weight: normal;
            letter-spacing: 1px;
        }
        
        .chapter-content {
            max-width: 800px;
            margin: 0 auto;
            text-align: justify;
        }
        
        .chapter-content p {
            margin-bottom: 1.8em;
            text-indent: 2em;
            line-height: 1.9;
            font-size: 17px;
            text-align: justify;
        }
        
        .chapter-content p:first-child {
            text-indent: 0;
        }
        
        .chapter-content p:first-child::first-letter {
            font-size: 300%;
            float: left;
            line-height: 1;
            margin-right: 8px;
            margin-top: 5px;
            color: #3498db;
            font-weight: bold;
        }
        
        /* Page footer */
        .page-footer {
            position: absolute;
            bottom: 30px;
            left: 0;
            right: 0;
            text-align: center;
            font-family: 'Roboto', sans-serif;
            font-size: 12px;
            color: #95a5a6;
            border-top: 1px solid #eee;
            padding-top: 10px;
            margin-top: 40px;
        }
        
        .page-number {
            float: right;
            font-weight: 500;
        }
        
        .novel-title {
            float: left;
        }
        
        /* Cover Page */
        .cover-page {
            page-break-before: always;
            min-height: 100vh;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: center;
            color: white;
            padding: 40px;
            text-align: center;
        }
        
        .cover-title {
            font-family: 'Roboto', sans-serif;
            font-size: 48px;
            font-weight: 300;
            margin-bottom: 20px;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.3);
        }
        
        .cover-subtitle {
            font-family: 'Merriweather', serif;
            font-size: 22px;
            font-weight: 300;
            margin-bottom: 40px;
            opacity: 0.9;
        }
        
        .cover-info {
            font-family: 'Roboto', sans-serif;
            font-size: 16px;
            margin-top: 60px;
            opacity: 0.8;
        }
        
        /* Mobile-friendly adjustments */
        @media screen and (max-width: 768px) {
            .chapter {
                padding: 30px 20px;
            }
            
            .toc-page {
                padding: 40px 20px;
            }
            
            .toc-title {
                font-size: 32px;
            }
            
            .chapter-title {
                font-size: 26px;
            }
            
            .chapter-content p {
                font-size: 16px;
            }
            
            .cover-title {
                font-size: 36px;
            }
        }
        
        /* Print-specific styles */
        @media print {
            .cover-page {
                background: white !important;
                color: black !important;
            }
            
            .cover-title {
                text-shadow: none !important;
            }
            
            .toc-page {
                background: white !important;
            }
            
            .page-footer {
                position: fixed;
                bottom: 20px;
            }
        }
    </style>
</head>
<body>
`;

        // Add cover page
        const novelTitle = NOVEL_ID.split('-')
            .map(word => word.charAt(0).toUpperCase() + word.slice(1))
            .join(' ');
        
        combinedHTML += `
    <div class="cover-page">
        <h1 class="cover-title">${novelTitle}</h1>
        <div class="cover-subtitle">Complete Novel Collection</div>
        <div class="cover-info">
            <p>${chapterFiles.length} Chapters</p>
            <p>Generated: ${new Date().toLocaleDateString('en-US', { 
                year: 'numeric', 
                month: 'long', 
                day: 'numeric' 
            })}</p>
            <p>For Offline Reading</p>
        </div>
    </div>
`;

        // Add table of contents
        combinedHTML += `
    <div class="toc-page">
        <h1 class="toc-title">Table of Contents</h1>
        <div class="toc-subtitle">Navigate to any chapter</div>
        <div class="toc-list">
`;

        // Add TOC items
        chapterFiles.forEach((file, index) => {
            const chapterNum = parseInt(file.match(/\d+/)[0]);
            const chapterPath = path.join(CLEAN_DIR, file);
            
            try {
                const html = fs.readFileSync(chapterPath, 'utf-8');
                const titleMatch = html.match(/<h1 class="chapter-title">(.*?)<\/h1>/);
                let title = `Chapter ${chapterNum}`;
                
                if (titleMatch && titleMatch[1]) {
                    title = titleMatch[1];
                }
                
                combinedHTML += `
            <div class="toc-item">
                <a href="#chapter-${chapterNum}" class="toc-link">
                    <span class="toc-number">${chapterNum.toString().padStart(3, '0')}</span>
                    <span class="toc-chapter-title">${title}</span>
                </a>
            </div>
`;
            } catch (error) {
                console.log(`  ⚠️ Error reading chapter ${chapterNum}: ${error.message}`);
                combinedHTML += `
            <div class="toc-item">
                <span class="toc-link">
                    <span class="toc-number">${chapterNum.toString().padStart(3, '0')}</span>
                    <span class="toc-chapter-title">Chapter ${chapterNum}</span>
                </span>
            </div>
`;
            }
        });

        combinedHTML += `
        </div>
    </div>
`;

        // Add chapters
        chapterFiles.forEach((file, index) => {
            const chapterNum = parseInt(file.match(/\d+/)[0]);
            const chapterPath = path.join(CLEAN_DIR, file);
            
            console.log(`  Processing Chapter ${chapterNum}...`);
            
            try {
                const html = fs.readFileSync(chapterPath, 'utf-8');
                
                // Extract chapter title
                const titleMatch = html.match(/<h1 class="chapter-title">(.*?)<\/h1>/);
                let title = `Chapter ${chapterNum}`;
                
                if (titleMatch && titleMatch[1]) {
                    title = titleMatch[1];
                }
                
                // Extract chapter content
                const contentMatch = html.match(/<div class="chapter-content">(.*?)<\/div>/s);
                let content = `<p>Content could not be extracted from Chapter ${chapterNum}</p>`;
                
                if (contentMatch && contentMatch[1]) {
                    content = contentMatch[1];
                    // Clean up content
                    content = content
                        .replace(/<script[\s\S]*?>[\s\S]*?<\/script>/gi, '')
                        .replace(/<style[\s\S]*?>[\s\S]*?<\/style>/gi, '')
                        .replace(/<a[\s\S]*?>[\s\S]*?<\/a>/gi, (match) => {
                            // Convert links to plain text
                            const textMatch = match.match(/>(.*?)</);
                            return textMatch ? textMatch[1] : '';
                        })
                        .replace(/<[^>]*>/g, (tag) => {
                            // Only keep basic tags
                            if (tag.match(/^<(p|br|strong|em|b|i|u|h[1-6]|div|span)(\s|>)/)) {
                                return tag;
                            }
                            return '';
                        });
                }
                
                combinedHTML += `
    <div class="chapter" id="chapter-${chapterNum}">
        <div class="chapter-header">
            <h1 class="chapter-title">${title}</h1>
            <div class="chapter-number">CHAPTER ${chapterNum}</div>
        </div>
        <div class="chapter-content">
            ${content}
        </div>
        <div class="page-footer">
            <span class="novel-title">${novelTitle}</span>
            <span class="page-number">Page <span class="page-number-placeholder"></span></span>
        </div>
    </div>
`;
            } catch (error) {
                console.log(`  ⚠️ Error processing Chapter ${chapterNum}: ${error.message}`);
                combinedHTML += `
    <div class="chapter" id="chapter-${chapterNum}">
        <div class="chapter-header">
            <h1 class="chapter-title">Chapter ${chapterNum}</h1>
            <div class="chapter-number">CHAPTER ${chapterNum}</div>
        </div>
        <div class="chapter-content">
            <p>Error loading this chapter. The content could not be processed.</p>
        </div>
        <div class="page-footer">
            <span class="novel-title">${novelTitle}</span>
            <span class="page-number">Page <span class="page-number-placeholder"></span></span>
        </div>
    </div>
`;
            }
        });

        combinedHTML += `
</body>
</html>`;

        // Save combined HTML temporarily
        const tempHtmlFile = './temp_combined.html';
        fs.writeFileSync(tempHtmlFile, combinedHTML, 'utf-8');
        console.log('\n✅ Combined HTML created');
        
        // Generate PDF
        console.log('🖨️  Generating PDF...');
        
        await page.goto(`file://${path.resolve(tempHtmlFile)}`, {
            waitUntil: 'networkidle0',
            timeout: 60000
        });
        
        // Add page numbers via JavaScript
        await page.evaluate(() => {
            // Add page numbers
            const pageNumbers = document.querySelectorAll('.page-number-placeholder');
            pageNumbers.forEach((span, index) => {
                span.textContent = index + 1;
            });
            
            // Update TOC links to work in PDF
            const tocLinks = document.querySelectorAll('.toc-link[href^="#"]');
            tocLinks.forEach(link => {
                const href = link.getAttribute('href');
                link.setAttribute('href', '#' + href.substring(1));
            });
        });
        
        // Generate PDF with proper formatting
        const pdfOptions = {
            path: OUTPUT_PDF,
            format: 'A5',
            printBackground: true,
            displayHeaderFooter: false,
            margin: {
                top: '50px',
                right: '40px',
                bottom: '80px', // Extra space for footer
                left: '40px'
            },
            preferCSSPageSize: true,
            timeout: 120000
        };
        
        await page.pdf(pdfOptions);
        
        console.log(`\n✅ PDF successfully created: ${OUTPUT_PDF}`);
        console.log(`📄 File size: ${(fs.statSync(OUTPUT_PDF).size / 1024 / 1024).toFixed(2)} MB`);
        
        // Clean up temp file
        fs.unlinkSync(tempHtmlFile);
        console.log('🧹 Temporary files cleaned up');
        
        // Generate a mobile-friendly version as well
        await generateMobileVersion(page, NOVEL_ID, chapterFiles.length, OUTPUT_PDF);
        
    } catch (error) {
        console.error('❌ Error generating PDF:', error);
    } finally {
        if (browser) {
            await browser.close();
        }
    }
}

async function generateMobileVersion(page, novelId, totalChapters, originalPdf) {
    console.log('\n📱 Creating mobile-friendly version...');
    
    try {
        const mobilePdf = `./${novelId.replace(/-/g, '_')}_mobile.pdf`;
        
        // Reload with mobile settings
        await page.setViewport({
            width: 375, // iPhone width
            height: 667,
            isMobile: true
        });
        
        const mobilePdfOptions = {
            path: mobilePdf,
            format: 'A5',
            printBackground: true,
            displayHeaderFooter: false,
            margin: {
                top: '30px',
                right: '20px',
                bottom: '60px',
                left: '20px'
            },
            scale: 0.8,
            timeout: 120000
        };
        
        await page.pdf(mobilePdfOptions);
        console.log(`✅ Mobile version created: ${mobilePdf}`);
        
        // Create instructions file
        const instructions = `
# 📚 ${novelId.replace(/-/g, ' ').toUpperCase()}

## Reading Instructions

### Files Available:
1. **${path.basename(originalPdf)}** - Standard PDF version (A5 format)
2. **${path.basename(mobilePdf)}** - Mobile-optimized PDF version

### For Best Reading Experience:

#### On iPhone/iPad:
1. Open the **${path.basename(mobilePdf)}** file
2. Use Books app or any PDF reader
3. Enable "Continuous Scroll" mode
4. Adjust brightness for comfortable reading

#### On Android:
1. Open the **${path.basename(mobilePdf)}** file
2. Use Google Play Books or any PDF reader
3. Enable "Night Mode" for evening reading

#### On Computer:
1. Open the **${path.basename(originalPdf)}** file
2. Use Adobe Reader or any PDF viewer
3. Use "Two-Page View" for desktop reading

### Features:
- ✅ Table of Contents with clickable links
- ✅ Chapter numbers and titles
- ✅ Proper page breaks
- ✅ Mobile-optimized text size
- ✅ Clean, readable formatting

### Total Content:
- ${totalChapters} chapters
- Sequential page numbering
- Professional formatting

### Tips:
- Use dark mode in your PDF reader for night reading
- Bookmark your progress
- Adjust font size if needed (some readers allow this)
- The PDF maintains original formatting on all devices

Generated on: ${new Date().toLocaleString()}
`;
        
        fs.writeFileSync('./READING_INSTRUCTIONS.txt', instructions, 'utf-8');
        console.log('📝 Reading instructions created: READING_INSTRUCTIONS.txt');
        
    } catch (error) {
        console.log('⚠️ Could not create mobile version:', error.message);
    }
}

// Run the conversion
convertToPDF().then(() => {
    console.log('\n🎉 Conversion complete!');
    console.log('\n📱 Now you can:');
    console.log('1. Transfer the PDF files to your phone');
    console.log('2. Open with any PDF reader app');
    console.log('3. Enjoy reading offline!');
    console.log('\n💡 Tip: Use "Google Play Books" on Android or "Books" app on iPhone for best experience.');
}).catch(error => {
    console.error('❌ Conversion failed:', error);
});







           