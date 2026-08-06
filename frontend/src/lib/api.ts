import type {
  ReportDTO,
  ScanRequest,
  VerifyRequest,
  VerifyResultDTO,
} from "$lib/types";

/** isWailsAvailable reports whether the Go bindings have been injected. */
export function isWailsAvailable(): boolean {
  return !!window.go?.v1?.Service;
}

function service() {
  const s = window.go?.v1?.Service;
  if (!s) {
    throw new Error(
      "Wails bindings are not available. Run the app with `wails dev` or a built binary.",
    );
  }
  return s;
}

export const api = {
  chooseFolder(): Promise<string> {
    return service().ChooseFolder();
  },
  startScan(req: ScanRequest): Promise<void> {
    return service().StartScan(req);
  },
  cancelScan(): Promise<void> {
    return service().CancelScan();
  },
  getReport(scanId: number): Promise<ReportDTO> {
    return service().GetReport(scanId);
  },
  verifyGroup(req: VerifyRequest): Promise<VerifyResultDTO> {
    return service().VerifyGroup(req);
  },
};

/**
 * onEvent subscribes to a Wails runtime event, returning an unsubscribe
 * function. It is a no-op (returning a no-op unsubscriber) when the runtime is
 * unavailable, so components can mount safely outside the desktop shell.
 */
export function onEvent(
  event: string,
  callback: (...data: any[]) => void,
): () => void {
  const rt = window.runtime;
  if (!rt) return () => {};
  return rt.EventsOn(event, callback);
}
