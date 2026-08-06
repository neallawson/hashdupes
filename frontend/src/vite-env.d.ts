/// <reference types="svelte" />
/// <reference types="vite/client" />

import type {
  ScanRequest,
  ReportDTO,
  VerifyRequest,
  VerifyResultDTO,
} from "$lib/types";

// Shape of the Wails-injected bindings for the bound *v1.Service object.
// Methods are generated under window.go[<package>][<struct>].
interface V1Service {
  ChooseFolder(): Promise<string>;
  StartScan(req: ScanRequest): Promise<void>;
  CancelScan(): Promise<void>;
  GetReport(scanId: number): Promise<ReportDTO>;
  VerifyGroup(req: VerifyRequest): Promise<VerifyResultDTO>;
}

interface WailsRuntime {
  EventsOn(event: string, callback: (...data: any[]) => void): () => void;
  EventsOff(event: string): void;
}

declare global {
  interface Window {
    go?: { v1?: { Service?: V1Service } };
    runtime?: WailsRuntime;
  }
}

export {};
