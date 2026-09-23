export namespace classify {
	
	export class Status {
	    clipAvailable: boolean;
	    reason: string;
	    modelDir: string;
	    libPath: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clipAvailable = source["clipAvailable"];
	        this.reason = source["reason"];
	        this.modelDir = source["modelDir"];
	        this.libPath = source["libPath"];
	    }
	}

}

export namespace config {
	
	export class Settings {
	    recentSources: string[];
	    lastDestination: string;
	    load: string;
	    verifyAfterCopy: boolean;
	    bytesPerSecLimit: number;
	    modelDir: string;
	    clipThreshold: number;
	    otherArchives: string[];
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recentSources = source["recentSources"];
	        this.lastDestination = source["lastDestination"];
	        this.load = source["load"];
	        this.verifyAfterCopy = source["verifyAfterCopy"];
	        this.bytesPerSecLimit = source["bytesPerSecLimit"];
	        this.modelDir = source["modelDir"];
	        this.clipThreshold = source["clipThreshold"];
	        this.otherArchives = source["otherArchives"];
	    }
	}

}

export namespace drives {
	
	export class Drive {
	    path: string;
	    label: string;
	    freeBytes: number;
	    totalBytes: number;
	    removable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Drive(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.label = source["label"];
	        this.freeBytes = source["freeBytes"];
	        this.totalBytes = source["totalBytes"];
	        this.removable = source["removable"];
	    }
	}

}

export namespace main {
	
	export class ArchiveRequest {
	    root: string;
	    dryRun: boolean;
	    verify: boolean;
	    load: string;
	
	    static createFrom(source: any = {}) {
	        return new ArchiveRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.dryRun = source["dryRun"];
	        this.verify = source["verify"];
	        this.load = source["load"];
	    }
	}
	export class DestinationInfo {
	    root: string;
	    exists: boolean;
	    isArchive: boolean;
	    freeBytes: number;
	    // Go type: index
	    stats: any;
	    needsReindex: boolean;
	    error?: string;
	    neededBytes: number;
	    enough: boolean;
	    insideSource: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DestinationInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.exists = source["exists"];
	        this.isArchive = source["isArchive"];
	        this.freeBytes = source["freeBytes"];
	        this.stats = this.convertValues(source["stats"], null);
	        this.needsReindex = source["needsReindex"];
	        this.error = source["error"];
	        this.neededBytes = source["neededBytes"];
	        this.enough = source["enough"];
	        this.insideSource = source["insideSource"];
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

}

