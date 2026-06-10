export type Tokens = {
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
  reasoningTokens?: number;
  costUSD: number;
  credits?: number;
};

export type Row = Tokens & {
  key: string;
  source?: string;
  sessionId?: string;
  project?: string;
  start?: string;
  lastActivity?: string;
  modelsUsed: string[];
  modelBreakdowns?: Array<Tokens & { model: string }>;
};

export type Query = {
  Sources?: string[];
  SourcePaths?: Record<string, string[]>;
  Since?: string;
  Until?: string;
  View?: string;
  Sparkline?: string;
  Compare?: string;
  By?: string;
  Project?: string;
  ID?: string;
  Order?: string;
  Top?: number;
  StartOfWeek?: string;
  Mode?: string;
  Speed?: string;
  Instances?: boolean;
  Active?: boolean;
  Recent?: boolean;
  Tolerant?: boolean;
};

export type Result = {
  view: string;
  data: Row[];
  totals: Tokens;
  trend?: {
    metric: string;
    start?: string;
    end?: string;
    series: number[];
    min: number;
    max: number;
    total: number;
  };
  sparklines?: Record<string, number[]>;
  warnings?: string[];
};

type UsageService = {
  GetUsage(query: Query): Promise<Result>;
  ListSources(): Promise<string[]>;
};

declare global {
  interface Window {
    go?: {
      main?: {
        UsageService?: UsageService;
      };
    };
  }
}

function service(): UsageService {
  const svc = window.go?.main?.UsageService;
  if (!svc) {
    throw new Error('Wails UsageService binding is not available.');
  }
  return svc;
}

export async function listSources(): Promise<string[]> {
  return service().ListSources();
}

export async function getUsage(query: Query): Promise<Result> {
  return service().GetUsage(query);
}
