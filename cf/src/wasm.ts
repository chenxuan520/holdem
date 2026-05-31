// Isolate-shared Go referee core.
//
// The Go reducer is PURE: all match state travels in/out as JSON, nothing is
// retained in the wasm instance between calls. That lets a single wasm
// instance be shared by the Worker and every Durable Object in the same
// isolate, instead of one per DO. We instantiate lazily, exactly once per
// isolate, and capture the registered global functions into typed handles
// immediately after go.run() so a later (re)assignment can never clobber what
// we hold.

import wasmModule from "./core.wasm";
import "./wasm_exec.js";

// Mirrors the functions backend/cmd/wasmcore registers on globalThis. Each
// takes/returns JSON strings (a single envelope: {record?,pending?,step?,
// body?,decision?,error?}); holdemRetryHint returns a bare string.
export interface GoCore {
  holdemReady: boolean;
  holdemCreateMatch(reqJSON: string, presetsJSON: string, id: string): string;
  holdemDecideNextStep(recordJSON: string): string;
  holdemAdvance(recordJSON: string, seq: number): string;
  holdemApplyHuman(recordJSON: string, action: string, amount: number, seq: number): string;
  holdemApplyAIDecision(
    recordJSON: string,
    presetJSON: string,
    decisionJSON: string,
    aiLogJSON: string,
    seq: number,
  ): string;
  holdemBuildAIRequestBody(
    recordJSON: string,
    seat: number,
    attempt: number,
    lastHint: string,
    presetJSON: string,
  ): string;
  holdemApplyControl(recordJSON: string, action: string): string;
  holdemParseAIResponse(rawBody: string): string;
  holdemRetryHint(rawErr: string): string;
  holdemRecordSummaryFromSnapshot(snapshotJSON: string): string;
  holdemRecordSummaryFromReplay(replayJSON: string): string;
  holdemReplayFromRecord(recordJSON: string): string;
}

interface GoRuntime {
  Go: new () => {
    importObject: WebAssembly.Imports;
    run(instance: WebAssembly.Instance): Promise<void>;
  };
}

let corePromise: Promise<GoCore> | null = null;

function instantiate(): Promise<GoCore> {
  const g = globalThis as unknown as GoRuntime & Record<string, unknown>;
  const go = new g.Go();
  return WebAssembly.instantiate(wasmModule, go.importObject).then((instance) => {
    // go.run starts the Go scheduler; main() registers the globals
    // synchronously and then parks on select{}, so they are ready once run
    // returns control. The returned promise never resolves (the program never
    // exits); we deliberately do not await it.
    void go.run(instance);
    return {
      holdemReady: g.holdemReady as boolean,
      holdemCreateMatch: g.holdemCreateMatch as GoCore["holdemCreateMatch"],
      holdemDecideNextStep: g.holdemDecideNextStep as GoCore["holdemDecideNextStep"],
      holdemAdvance: g.holdemAdvance as GoCore["holdemAdvance"],
      holdemApplyHuman: g.holdemApplyHuman as GoCore["holdemApplyHuman"],
      holdemApplyAIDecision: g.holdemApplyAIDecision as GoCore["holdemApplyAIDecision"],
      holdemBuildAIRequestBody: g.holdemBuildAIRequestBody as GoCore["holdemBuildAIRequestBody"],
      holdemApplyControl: g.holdemApplyControl as GoCore["holdemApplyControl"],
      holdemParseAIResponse: g.holdemParseAIResponse as GoCore["holdemParseAIResponse"],
      holdemRetryHint: g.holdemRetryHint as GoCore["holdemRetryHint"],
      holdemRecordSummaryFromSnapshot: g.holdemRecordSummaryFromSnapshot as GoCore["holdemRecordSummaryFromSnapshot"],
      holdemRecordSummaryFromReplay: g.holdemRecordSummaryFromReplay as GoCore["holdemRecordSummaryFromReplay"],
      holdemReplayFromRecord: g.holdemReplayFromRecord as GoCore["holdemReplayFromRecord"],
    };
  });
}

export function goCore(): Promise<GoCore> {
  if (!corePromise) {
    corePromise = instantiate();
  }
  return corePromise;
}
