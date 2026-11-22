export namespace service {
	
	export class ChapterRow {
	    number: number;
	    title: string;
	    state: string;
	    error?: string;
	    readerUrl: string;
	    isClean: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChapterRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.title = source["title"];
	        this.state = source["state"];
	        this.error = source["error"];
	        this.readerUrl = source["readerUrl"];
	        this.isClean = source["isClean"];
	    }
	}
	export class DiscoverResult {
	    siteId: string;
	    siteName: string;
	    url: string;
	    title: string;
	    author: string;
	    cover: string;
	    description: string;
	    status?: string;
	    chapters: source.ChapterRef[];
	    tocMode: string;
	    firstChapterUrl?: string;
	    inputKind: string;
	    inputChapter?: number;
	    vars?: Record<string, string>;
	    warnings?: string[];
	    totalChapters: number;
	    existingSlug?: string;
	    downloaded: number;
	
	    static createFrom(source: any = {}) {
	        return new DiscoverResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.siteId = source["siteId"];
	        this.siteName = source["siteName"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.cover = source["cover"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.chapters = this.convertValues(source["chapters"], source.ChapterRef);
	        this.tocMode = source["tocMode"];
	        this.firstChapterUrl = source["firstChapterUrl"];
	        this.inputKind = source["inputKind"];
	        this.inputChapter = source["inputChapter"];
	        this.vars = source["vars"];
	        this.warnings = source["warnings"];
	        this.totalChapters = source["totalChapters"];
	        this.existingSlug = source["existingSlug"];
	        this.downloaded = source["downloaded"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class NovelDetail {
	    slug: string;
	    title: string;
	    author: string;
	    cover: string;
	    description: string;
	    url: string;
	    site: string;
	    totalChapters: number;
	    rawCount: number;
	    cleanCount: number;
	    failedCount: number;
	    lockedCount: number;
	    hasIndex: boolean;
	    catalogUrl: string;
	    sizeBytes: number;
	    sizeMb: string;
	    chapters: ChapterRow[];
	
	    static createFrom(source: any = {}) {
	        return new NovelDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slug = source["slug"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.cover = source["cover"];
	        this.description = source["description"];
	        this.url = source["url"];
	        this.site = source["site"];
	        this.totalChapters = source["totalChapters"];
	        this.rawCount = source["rawCount"];
	        this.cleanCount = source["cleanCount"];
	        this.failedCount = source["failedCount"];
	        this.lockedCount = source["lockedCount"];
	        this.hasIndex = source["hasIndex"];
	        this.catalogUrl = source["catalogUrl"];
	        this.sizeBytes = source["sizeBytes"];
	        this.sizeMb = source["sizeMb"];
	        this.chapters = this.convertValues(source["chapters"], ChapterRow);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class NovelItem {
	    slug: string;
	    title: string;
	    author: string;
	    cover: string;
	    description: string;
	    url: string;
	    site: string;
	    totalChapters: number;
	    rawCount: number;
	    cleanCount: number;
	    failedCount: number;
	    lockedCount: number;
	    hasIndex: boolean;
	    catalogUrl: string;
	    sizeBytes: number;
	    sizeMb: string;
	
	    static createFrom(source: any = {}) {
	        return new NovelItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slug = source["slug"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.cover = source["cover"];
	        this.description = source["description"];
	        this.url = source["url"];
	        this.site = source["site"];
	        this.totalChapters = source["totalChapters"];
	        this.rawCount = source["rawCount"];
	        this.cleanCount = source["cleanCount"];
	        this.failedCount = source["failedCount"];
	        this.lockedCount = source["lockedCount"];
	        this.hasIndex = source["hasIndex"];
	        this.catalogUrl = source["catalogUrl"];
	        this.sizeBytes = source["sizeBytes"];
	        this.sizeMb = source["sizeMb"];
	    }
	}
	export class ScrapeOptions {
	    url: string;
	    startChapter: number;
	    endChapter: number;
	    concurrency: number;
	    force: boolean;
	    skipImages: boolean;
	    fromPasted: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScrapeOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.startChapter = source["startChapter"];
	        this.endChapter = source["endChapter"];
	        this.concurrency = source["concurrency"];
	        this.force = source["force"];
	        this.skipImages = source["skipImages"];
	        this.fromPasted = source["fromPasted"];
	    }
	}
	export class SystemStats {
	    totalNovels: number;
	    totalRawChapters: number;
	    totalCleanChapters: number;
	    totalBytes: number;
	    sizeMb: string;
	    libraryPath: string;
	
	    static createFrom(source: any = {}) {
	        return new SystemStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalNovels = source["totalNovels"];
	        this.totalRawChapters = source["totalRawChapters"];
	        this.totalCleanChapters = source["totalCleanChapters"];
	        this.totalBytes = source["totalBytes"];
	        this.sizeMb = source["sizeMb"];
	        this.libraryPath = source["libraryPath"];
	    }
	}

}

