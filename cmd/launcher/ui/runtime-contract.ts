type RuntimeProviderID = string;
type RuntimeSessionID = string;

interface RuntimeModelRef {
  providerID: RuntimeProviderID;
  modelID: string;
}

interface RuntimeSessionCreateInput {
  parentID?: RuntimeSessionID;
  title?: string;
}

interface RuntimePromptInput {
  text?: string;
  parts?: unknown[];
  agent?: string;
  model?: RuntimeModelRef;
  variant?: string;
  messageID?: string;
  directory?: string;
}

interface RuntimeHostedProviderContract {
  readonly providerID: RuntimeProviderID;
  readonly preferredModels: readonly string[];
  status(): Promise<{ authenticated: boolean; type: string; organizationId: string }>;
  authorize(): Promise<unknown>;
  callback(signal?: AbortSignal): Promise<unknown>;
  disconnect(): Promise<unknown>;
}

interface TLAgentRuntimeContract {
  readonly version: string;
  readonly hosted: RuntimeHostedProviderContract;
  health(): Promise<unknown>;
  agents(): Promise<unknown[]>;
  providerState(): Promise<unknown>;
  sessions: {
    list(options?: { limit?: number; directory?: string }): Promise<unknown>;
    create(input?: RuntimeSessionCreateInput): Promise<unknown>;
    promptAsync(sessionID: RuntimeSessionID, input?: RuntimePromptInput): Promise<unknown>;
    abort(sessionID: RuntimeSessionID, options?: { scope?: string; directory?: string }): Promise<unknown>;
  };
}

declare global {
  interface Window {
    KLU: {
      api?: TLAgentRuntimeContract;
      [key: string]: unknown;
    };
  }
}

export {};
