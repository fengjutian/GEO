export type Site = {
  id: number;
  name: string;
  url: string;
  description: string;
  createdAt: string;
  updatedAt: string;
};

export type Scan = {
  id: number;
  siteId: number;
  status: 'running' | 'completed' | 'failed' | string;
  targetUrl: string;
  startedAt: string;
  finishedAt: string;
  pageCount: number;
  score: number;
  summary: string;
  errorCode?: string;
  errorMessage?: string;
  ruleVersion: string;
  scoreVersion: string;
  createdAt: string;
  issueCount: number;
  highIssueCount: number;
};

export type Finding = {
  id: number;
  siteId: number;
  scanId: number;
  fingerprint: string;
  severity: 'critical' | 'high' | 'medium' | 'low' | string;
  ruleCode: string;
  title: string;
  url: string;
  evidence: string;
  recommendation: string;
  scorePenalty: number;
  status: 'open' | 'in_progress' | 'resolved' | 'ignored' | string;
  firstSeenAt: string;
  lastSeenAt: string;
  createdAt: string;
  updatedAt: string;
};

export type SiteDetail = {
  site: Site;
  scans: Scan[];
  latestScan?: Scan;
};

export type ScanComparison = {
  baseScanId: number;
  newCount: number;
  persistingCount: number;
  resolvedCount: number;
  new: Finding[];
  resolved: Finding[];
};

export type ScanDetail = {
  scan: Scan;
  site: Site;
  findings: Finding[];
  comparison: ScanComparison;
  severityCounts: Record<string, number>;
  statusCounts: Record<string, number>;
};

export type DashboardStats = {
  siteCount: number;
  scanCount: number;
  openIssues: number;
  highPriority: number;
  averageScore: number;
};

export type DashboardSite = Site & {
  latestScan: Scan | null;
  openIssues: number;
  highPriority: number;
};

export type RecentScan = Scan & { siteName: string };

export type DashboardData = {
  stats: DashboardStats;
  sites: DashboardSite[];
  recentScans: RecentScan[];
};

export type SiteInput = {
  name: string;
  url: string;
  description: string;
};

export type IssueInput = {
  status: string;
  note?: string;
};

export type FindingFilters = {
  scanId?: number;
  severity?: string;
  status?: string;
  q?: string;
};
