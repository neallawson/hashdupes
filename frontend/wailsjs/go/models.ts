export namespace v1 {
	
	export class FileDTO {
	    id: number;
	    path: string;
	    name: string;
	    size: number;
	    modTime: string;
	    ext: string;
	    isEmpty: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.modTime = source["modTime"];
	        this.ext = source["ext"];
	        this.isEmpty = source["isEmpty"];
	    }
	}
	export class FileGroupDTO {
	    id: string;
	    size: number;
	    headHash: string;
	    files: FileDTO[];
	    reclaimable: number;
	    verified: boolean;
	    withinDupFolder: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileGroupDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.size = source["size"];
	        this.headHash = source["headHash"];
	        this.files = this.convertValues(source["files"], FileDTO);
	        this.reclaimable = source["reclaimable"];
	        this.verified = source["verified"];
	        this.withinDupFolder = source["withinDupFolder"];
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
	export class FolderDTO {
	    id: number;
	    path: string;
	    name: string;
	    folderHash: string;
	    isEmpty: boolean;
	    fileCount: number;
	    totalSize: number;
	
	    static createFrom(source: any = {}) {
	        return new FolderDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.name = source["name"];
	        this.folderHash = source["folderHash"];
	        this.isEmpty = source["isEmpty"];
	        this.fileCount = source["fileCount"];
	        this.totalSize = source["totalSize"];
	    }
	}
	export class FolderGroupDTO {
	    id: string;
	    folderHash: string;
	    folders: FolderDTO[];
	    reclaimable: number;
	
	    static createFrom(source: any = {}) {
	        return new FolderGroupDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.folderHash = source["folderHash"];
	        this.folders = this.convertValues(source["folders"], FolderDTO);
	        this.reclaimable = source["reclaimable"];
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
	export class ReportDTO {
	    scanId: number;
	    folderGroups: FolderGroupDTO[];
	    fileGroups: FileGroupDTO[];
	    allVerified: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ReportDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scanId = source["scanId"];
	        this.folderGroups = this.convertValues(source["folderGroups"], FolderGroupDTO);
	        this.fileGroups = this.convertValues(source["fileGroups"], FileGroupDTO);
	        this.allVerified = source["allVerified"];
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
	export class ScanRequest {
	    rootPath: string;
	    algo: string;
	    headBytes: number;
	    workers: number;
	    followSymlinks: boolean;
	    ignoreHidden: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScanRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rootPath = source["rootPath"];
	        this.algo = source["algo"];
	        this.headBytes = source["headBytes"];
	        this.workers = source["workers"];
	        this.followSymlinks = source["followSymlinks"];
	        this.ignoreHidden = source["ignoreHidden"];
	    }
	}
	export class VerifyRequest {
	    size: number;
	    paths: string[];
	
	    static createFrom(source: any = {}) {
	        return new VerifyRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.size = source["size"];
	        this.paths = source["paths"];
	    }
	}
	export class VerifyResultDTO {
	    groups: FileGroupDTO[];
	    errors?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new VerifyResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groups = this.convertValues(source["groups"], FileGroupDTO);
	        this.errors = source["errors"];
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

