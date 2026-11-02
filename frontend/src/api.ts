import type {
  DashboardData,
  Finding,
  FindingFilters,
  IssueInput,
  Scan,
  ScanDetail,
  Site,
  SiteDetail,
  SiteInput,
} from './types';

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? 'http://localhost:8080/api';

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(message: string, status = 0, code = 'NETWORK_ERROR') {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE}${path}`, init);
  } catch {
    throw new ApiError('无法连接后端服务，请确认 Go API 与 MySQL 已启动。');
  }
  if (response.status === 204) {
    return undefined as T;
  }
  const text = await response.text();
  let payload: unknown = null;
  if (text) {
    try {
      payload = JSON.parse(text);
    } catch {
      payload = text;
    }
  }
  if (!response.ok) {
    const body = payload as { message?: string; error?: string; code?: string } | null;
    throw new ApiError(body?.message || body?.error || `请求失败（HTTP ${response.status}）`, response.status, body?.code || 'INTERNAL_ERROR');
  }
  return payload as T;
}

function json(method: string, body: unknown): RequestInit {
  return {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  };
}

export function reportUrl(scanId: number): string {
  return `${API_BASE}/scans/${scanId}/report`;
}

export function reportJsonUrl(scanId: number): string {
  return `${API_BASE}/scans/${scanId}/report.json`;
}

function queryString(filters: FindingFilters): string {
  const params = new URLSearchParams();
  if (filters.scanId) params.set('scanId', String(filters.scanId));
  if (filters.severity) params.set('severity', filters.severity);
  if (filters.status) params.set('status', filters.status);
  if (filters.q) params.set('q', filters.q);
  const value = params.toString();
  return value ? `?${value}` : '';
}

export const api = {
  dashboard: () => request<DashboardData>('/dashboard'),
  listSites: () => request<Site[]>('/sites'),
  getSite: (id: number) => request<SiteDetail>(`/sites/${id}`),
  createSite: (input: SiteInput) => request<Site>('/sites', json('POST', input)),
  updateSite: (id: number, input: SiteInput) => request<Site>(`/sites/${id}`, json('PUT', input)),
  deleteSite: (id: number) => request<void>(`/sites/${id}`, { method: 'DELETE' }),
  runScan: (id: number) => request<{ scan: Scan }>(`/sites/${id}/scans`, { method: 'POST' }),
  getSiteFindings: (id: number, filters: FindingFilters = {}) =>
    request<Finding[]>(`/sites/${id}/findings${queryString(filters)}`),
  getScan: (id: number) => request<ScanDetail>(`/scans/${id}`),
  updateIssue: (id: number, input: IssueInput) => request<Finding>(`/issues/${id}`, json('PATCH', input)),
};

export function isTerminalScanStatus(status: string): boolean {
  return status === 'completed' || status === 'failed' || status === 'cancelled';
}

export async function waitForScan(
  scanId: number,
  options: { timeoutMs?: number; intervalMs?: number; onStatus?: (scan: Scan) => void } = {},
): Promise<ScanDetail> {
  const timeoutMs = options.timeoutMs ?? 120000;
  const intervalMs = options.intervalMs ?? 800;
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const detail = await api.getScan(scanId);
    options.onStatus?.(detail.scan);
    if (isTerminalScanStatus(detail.scan.status)) {
      return detail;
    }
    await new Promise((resolve) => window.setTimeout(resolve, intervalMs));
  }
  throw new ApiError('扫描排队或执行时间过长，请稍后在扫描历史中查看结果。', 0, 'SCAN_TIMEOUT');
}
