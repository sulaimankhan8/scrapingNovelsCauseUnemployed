export namespace archiver {
	
	export class Progress {
	    stage: string;
	    current: number;
	    total: number;
	    scraped: number;
	    cached: number;
	    failed: number;
	    speed: string;
	    eta: string;
	    percentage: number;
	    chapterNum?: number;
	    chapterTitle?: string;
	    status?: string;
	    message?: string;
	    slug?: string;
	    title?: string;
	
	    static createFrom(source: any = {}) {
	        return new Progress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stage = source["stage"];
	        this.current = source["current"];
	        this.total = source["total"];
	        this.scraped = source["scraped"];
	        this.cached = source["cached"];
	        this.failed = source["failed"];
	        this.speed = source["speed"];
	        this.eta = source["eta"];
	        this.percentage = source["percentage"];
	        this.chapterNum = source["chapterNum"];
	        this.chapterTitle = source["chapterTitle"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.slug = source["slug"];
	        this.title = source["title"];
	    }
	}

}

export namespace manga {
	
	export class Job {
	    id: number;
	    kind: string;
	    slug: string;
	    title: string;
	    from?: number;
	    to?: number;
	    keys?: number[];
	    auto?: boolean;
	    message: string;
	    done: number;
	    total: number;
	    failed: number;
	    // Go type: time
	    started: any;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.slug = source["slug"];
	        this.title = source["title"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.keys = source["keys"];
	        this.auto = source["auto"];
	        this.message = source["message"];
	        this.done = source["done"];
	        this.total = source["total"];
	        this.failed = source["failed"];
	        this.started = this.convertValues(source["started"], null);
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
	export class Activity {
	    current?: Job;
	    queue: Job[];
	
	    static createFrom(source: any = {}) {
	        return new Activity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = this.convertValues(source["current"], Job);
	        this.queue = this.convertValues(source["queue"], Job);
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
	export class ChapterView {
	    slug: string;
	    series: string;
	    key: number;
	    number: number;
	    total: number;
	    title: string;
	    pages: string[];
	    dims: number[][];
	    prevKey: number;
	    nextKey: number;
	    nextSaved: boolean;
	    readMode: string;
	    readPage: number;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new ChapterView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slug = source["slug"];
	        this.series = source["series"];
	        this.key = source["key"];
	        this.number = source["number"];
	        this.total = source["total"];
	        this.title = source["title"];
	        this.pages = source["pages"];
	        this.dims = source["dims"];
	        this.prevKey = source["prevKey"];
	        this.nextKey = source["nextKey"];
	        this.nextSaved = source["nextSaved"];
	        this.readMode = source["readMode"];
	        this.readPage = source["readPage"];
	        this.source = source["source"];
	    }
	}
	export class ExportResult {
	    path: string;
	    chapters: number;
	    pages: number;
	    bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new ExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.chapters = source["chapters"];
	        this.pages = source["pages"];
	        this.bytes = source["bytes"];
	    }
	}
	
	export class LibraryItem {
	    slug: string;
	    title: string;
	    type: string;
	    siteName: string;
	    url: string;
	    cover: string;
	    total: number;
	    downloaded: number;
	    failed: number;
	    bytes: number;
	    readNumber: number;
	    readKey: number;
	    firstKey: number;
	    newChapters: number;
	    status: string;
	    // Go type: time
	    addedAt: any;
	    // Go type: time
	    readAt: any;
	
	    static createFrom(source: any = {}) {
	        return new LibraryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slug = source["slug"];
	        this.title = source["title"];
	        this.type = source["type"];
	        this.siteName = source["siteName"];
	        this.url = source["url"];
	        this.cover = source["cover"];
	        this.total = source["total"];
	        this.downloaded = source["downloaded"];
	        this.failed = source["failed"];
	        this.bytes = source["bytes"];
	        this.readNumber = source["readNumber"];
	        this.readKey = source["readKey"];
	        this.firstKey = source["firstKey"];
	        this.newChapters = source["newChapters"];
	        this.status = source["status"];
	        this.addedAt = this.convertValues(source["addedAt"], null);
	        this.readAt = this.convertValues(source["readAt"], null);
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
	export class Chapter {
	    key: number;
	    number: number;
	    title: string;
	    url: string;
	    id?: string;
	    state?: string;
	    error?: string;
	    pages?: number;
	    bytes?: number;
	    // Go type: time
	    savedAt: any;
	    removed?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Chapter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.number = source["number"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.id = source["id"];
	        this.state = source["state"];
	        this.error = source["error"];
	        this.pages = source["pages"];
	        this.bytes = source["bytes"];
	        this.savedAt = this.convertValues(source["savedAt"], null);
	        this.removed = source["removed"];
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
	export class SeriesDetail {
	    slug: string;
	    siteId: string;
	    siteName: string;
	    type: string;
	    url: string;
	    title: string;
	    author: string;
	    description: string;
	    status?: string;
	    coverUrl?: string;
	    coverFile?: string;
	    vars?: Record<string, string>;
	    chapters: Chapter[];
	    nextKey: number;
	    // Go type: time
	    addedAt: any;
	    // Go type: time
	    updatedAt: any;
	    // Go type: time
	    checkedAt: any;
	    readKey?: number;
	    readPage?: number;
	    // Go type: time
	    readAt: any;
	    readMode?: string;
	    newChapters?: number;
	    cover: string;
	    folder: string;
	    effectiveReadMode: string;
	
	    static createFrom(source: any = {}) {
	        return new SeriesDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slug = source["slug"];
	        this.siteId = source["siteId"];
	        this.siteName = source["siteName"];
	        this.type = source["type"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.coverUrl = source["coverUrl"];
	        this.coverFile = source["coverFile"];
	        this.vars = source["vars"];
	        this.chapters = this.convertValues(source["chapters"], Chapter);
	        this.nextKey = source["nextKey"];
	        this.addedAt = this.convertValues(source["addedAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.checkedAt = this.convertValues(source["checkedAt"], null);
	        this.readKey = source["readKey"];
	        this.readPage = source["readPage"];
	        this.readAt = this.convertValues(source["readAt"], null);
	        this.readMode = source["readMode"];
	        this.newChapters = source["newChapters"];
	        this.cover = source["cover"];
	        this.folder = source["folder"];
	        this.effectiveReadMode = source["effectiveReadMode"];
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
	export class ChapterRef {
	    number: number;
	    title: string;
	    url: string;
	    id?: string;
	
	    static createFrom(source: any = {}) {
	        return new ChapterRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.id = source["id"];
	    }
	}
	export class SeriesPreview {
	    siteId: string;
	    siteName: string;
	    type: string;
	    url: string;
	    title: string;
	    author: string;
	    cover: string;
	    description: string;
	    status?: string;
	    chapters: ChapterRef[];
	    vars?: Record<string, string>;
	    warnings?: string[];
	    inputChapter?: number;
	    existingSlug?: string;
	    downloaded: number;
	
	    static createFrom(source: any = {}) {
	        return new SeriesPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.siteId = source["siteId"];
	        this.siteName = source["siteName"];
	        this.type = source["type"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.cover = source["cover"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.chapters = this.convertValues(source["chapters"], ChapterRef);
	        this.vars = source["vars"];
	        this.warnings = source["warnings"];
	        this.inputChapter = source["inputChapter"];
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
	export class Settings {
	    chapterWorkers: number;
	    pageWorkers: number;
	    autoDownloadNew: boolean;
	    checkOnOpen: boolean;
	    readMode: string;
	    notify: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chapterWorkers = source["chapterWorkers"];
	        this.pageWorkers = source["pageWorkers"];
	        this.autoDownloadNew = source["autoDownloadNew"];
	        this.checkOnOpen = source["checkOnOpen"];
	        this.readMode = source["readMode"];
	        this.notify = source["notify"];
	    }
	}
	export class SiteInfo {
	    id: string;
	    name: string;
	    type: string;
	    home: string;
	    status: string;
	    example: string;
	
	    static createFrom(source: any = {}) {
	        return new SiteInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.home = source["home"];
	        this.status = source["status"];
	        this.example = source["example"];
	    }
	}

}

export namespace service {
	
	export class Settings {
	    theme: string;
	    chime: boolean;
	    notify: boolean;
	    speed: string;
	    keepImages: boolean;
	    revise: string;
	    updateOnOpen: boolean;
	    wifiOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.chime = source["chime"];
	        this.notify = source["notify"];
	        this.speed = source["speed"];
	        this.keepImages = source["keepImages"];
	        this.revise = source["revise"];
	        this.updateOnOpen = source["updateOnOpen"];
	        this.wifiOnly = source["wifiOnly"];
	    }
	}
	export class QueueItem {
	    id: number;
	    kind: string;
	    slug?: string;
	    title: string;
	    url: string;
	    cover?: string;
	
	    static createFrom(source: any = {}) {
	        return new QueueItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.slug = source["slug"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.cover = source["cover"];
	    }
	}
	export class Current {
	    id: number;
	    kind: string;
	    slug?: string;
	    title: string;
	    url: string;
	    cover?: string;
	    phase: string;
	    message?: string;
	    progress?: archiver.Progress;
	    challenge?: string;
	    // Go type: time
	    startedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Current(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.slug = source["slug"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.cover = source["cover"];
	        this.phase = source["phase"];
	        this.message = source["message"];
	        this.progress = this.convertValues(source["progress"], archiver.Progress);
	        this.challenge = source["challenge"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
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
	export class Activity {
	    current?: Current;
	    queue: QueueItem[];
	    waiting?: string;
	    settings: Settings;
	
	    static createFrom(source: any = {}) {
	        return new Activity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = this.convertValues(source["current"], Current);
	        this.queue = this.convertValues(source["queue"], QueueItem);
	        this.waiting = source["waiting"];
	        this.settings = this.convertValues(source["settings"], Settings);
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
	export class BackupInfo {
	    app: string;
	    version: number;
	    // Go type: time
	    createdAt: any;
	    novels: number;
	    chapters: number;
	    bytes: number;
	    path?: string;
	    fileName?: string;
	    newHere: number;
	    merged: number;
	    sizeMb?: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.app = source["app"];
	        this.version = source["version"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.novels = source["novels"];
	        this.chapters = source["chapters"];
	        this.bytes = source["bytes"];
	        this.path = source["path"];
	        this.fileName = source["fileName"];
	        this.newHere = source["newHere"];
	        this.merged = source["merged"];
	        this.sizeMb = source["sizeMb"];
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
	export class ChapterRow {
	    number: number;
	    title: string;
	    state: string;
	    error?: string;
	    readerUrl: string;
	    isClean: boolean;
	    words?: number;
	    revised?: string;
	    removed?: boolean;
	
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
	        this.words = source["words"];
	        this.revised = source["revised"];
	        this.removed = source["removed"];
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
	    coverData?: string;
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
	        this.coverData = source["coverData"];
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
	export class ExportResult {
	    path: string;
	    fileName: string;
	    chapters: number;
	    images: number;
	    sizeMb: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.fileName = source["fileName"];
	        this.chapters = source["chapters"];
	        this.images = source["images"];
	        this.sizeMb = source["sizeMb"];
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
	    status?: string;
	    pendingCount: number;
	    removedCount: number;
	    revisedCount: number;
	    lastRead: number;
	    // Go type: time
	    lastReadAt: any;
	    // Go type: time
	    addedAt: any;
	    // Go type: time
	    updatedAt: any;
	    // Go type: time
	    checkedAt: any;
	    lastNew: number;
	    lastNewAt?: string;
	    crawlMode: boolean;
	    imageBytes: number;
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
	        this.status = source["status"];
	        this.pendingCount = source["pendingCount"];
	        this.removedCount = source["removedCount"];
	        this.revisedCount = source["revisedCount"];
	        this.lastRead = source["lastRead"];
	        this.lastReadAt = this.convertValues(source["lastReadAt"], null);
	        this.addedAt = this.convertValues(source["addedAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.checkedAt = this.convertValues(source["checkedAt"], null);
	        this.lastNew = source["lastNew"];
	        this.lastNewAt = source["lastNewAt"];
	        this.crawlMode = source["crawlMode"];
	        this.imageBytes = source["imageBytes"];
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
	    status?: string;
	    pendingCount: number;
	    removedCount: number;
	    revisedCount: number;
	    lastRead: number;
	    // Go type: time
	    lastReadAt: any;
	    // Go type: time
	    addedAt: any;
	    // Go type: time
	    updatedAt: any;
	    // Go type: time
	    checkedAt: any;
	    lastNew: number;
	    lastNewAt?: string;
	    crawlMode: boolean;
	
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
	        this.status = source["status"];
	        this.pendingCount = source["pendingCount"];
	        this.removedCount = source["removedCount"];
	        this.revisedCount = source["revisedCount"];
	        this.lastRead = source["lastRead"];
	        this.lastReadAt = this.convertValues(source["lastReadAt"], null);
	        this.addedAt = this.convertValues(source["addedAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.checkedAt = this.convertValues(source["checkedAt"], null);
	        this.lastNew = source["lastNew"];
	        this.lastNewAt = source["lastNewAt"];
	        this.crawlMode = source["crawlMode"];
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
	
	export class RestoreResult {
	    added: number;
	    merged: number;
	    chapters: number;
	
	    static createFrom(source: any = {}) {
	        return new RestoreResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.merged = source["merged"];
	        this.chapters = source["chapters"];
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
	
	export class SiteInfo {
	    id: string;
	    name: string;
	    domains: string[];
	    home: string;
	    example: string;
	    status: string;
	    fetchMode: string;
	    verified: string;
	
	    static createFrom(source: any = {}) {
	        return new SiteInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.domains = source["domains"];
	        this.home = source["home"];
	        this.example = source["example"];
	        this.status = source["status"];
	        this.fetchMode = source["fetchMode"];
	        this.verified = source["verified"];
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

