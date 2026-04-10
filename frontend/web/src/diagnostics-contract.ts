import type { AlertAnalysis, ChatTraceEvent } from "@/services/api";

type AssertExtends<T extends U, U> = T;

type AlertAnalysisWithTrace = AssertExtends<
  AlertAnalysis,
  {
    trace?: ChatTraceEvent[];
  }
>;

export const alertAnalysisDiagnosticsContract = (
  analysis: AlertAnalysisWithTrace
): number => analysis.trace?.length ?? 0;
