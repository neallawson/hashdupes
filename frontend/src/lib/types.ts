// Mirrors of the Go internal/api/v1 DTOs. Keep field names in sync with the
// JSON tags on the Go side.

export type HashAlgo = "blake3" | "sha256";

export interface ScanRequest {
  rootPath: string;
  algo?: HashAlgo | "";
  headBytes?: number;
  workers?: number;
  followSymlinks?: boolean;
  ignoreHidden?: boolean;
}

export interface ScanProgress {
  filesSeen: number;
  filesHashed: number;
  bytesHashed: number;
  skipped: number;
  currentPath: string;
  done: boolean;
}

export interface ScanDone {
  scanId: number;
  filesHashed: number;
  foldersSeen: number;
  skipped: number;
}

export interface ScanError {
  message: string;
}

export interface FileDTO {
  id: number;
  path: string;
  name: string;
  size: number;
  modTime: string;
  ext: string;
  isEmpty: boolean;
}

export interface FileGroupDTO {
  id: string;
  size: number;
  headHash: string;
  files: FileDTO[];
  reclaimable: number;
  verified: boolean;
  withinDupFolder: boolean;
}

export interface FolderDTO {
  id: number;
  path: string;
  name: string;
  folderHash: string;
  isEmpty: boolean;
  fileCount: number;
  totalSize: number;
}

export interface FolderGroupDTO {
  id: string;
  folderHash: string;
  folders: FolderDTO[];
  reclaimable: number;
}

export interface ReportDTO {
  scanId: number;
  folderGroups: FolderGroupDTO[];
  fileGroups: FileGroupDTO[];
  allVerified: boolean;
}

export interface VerifyRequest {
  size: number;
  paths: string[];
}

export interface VerifyResultDTO {
  groups: FileGroupDTO[];
  errors?: Record<string, string>;
}
