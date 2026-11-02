import { useEffect, useMemo, useState, type ReactNode } from 'react';
import {
  Activity,
  ArrowDownRight,
  ArrowLeft,
  ArrowUpRight,
  CheckCircle2,
  CircleAlert,
  Clock3,
  FileJson,
  FileText,
  History,
  Loader2,
  Pencil,
  Plus,
  RefreshCw,
  Search,
  ShieldCheck,
  Trash2,
  Wrench,
} from 'lucide-react';
import { api, reportJsonUrl, reportUrl } from './api';
import type { Finding, Scan, ScanDetail, Site, SiteDetail, SiteInput } from './types';
import {
  EmptyState,
  IssueStatusBadge,
  IssueStatusSelect,
  Modal,
  ScanFailureNote,
  ScanStatusBadge,
  ScoreRing,
  SeverityBadge,
  SeverityDistribution,
  SiteFormModal,
  StatCard,
  formatDate,
  formatShortDate,
  issueStatusMeta,
  issueStatusOptions,
  severityMeta,
} from './components';

type Tab = 'overview' | 'issues' | 'history';

function tabFromHash(): Tab {
  const match = window.location.hash.match(/[?&]tab=(overview|issues|history)/);
  return (match?.[1] as Tab | undefined) ?? 'overview';
}

export function SitePage({
  siteId,
  onBack,
  onError,
  onChanged,
  onDeleted,
}: {
  siteId: number;
  onBack: () => void;
  onError: (message: string) => void;
  onChanged: () => void;
  onDeleted: () => void;
}) {
  const [detail, setDetail] = useState<SiteDetail | null>(null);
  const [findings, setFindings] = useState<Finding[]>([]);
  const [latestDetail, setLatestDetail] = useState<ScanDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<Tab>(() => tabFromHash());
  const [scanning, setScanning] = useState(false);
  const [scanStartedAt, setScanStartedAt] = useState<number | null>(null);
  const [elapsed, setElapsed] = useState(0);
  const [editing, setEditing] = useState(false);
  const [savingSite, setSavingSite] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [scanModalOpen, setScanModalOpen] = useState(false);
  const [scanModalData, setScanModalData] = useState<ScanDetail | null>(null);
  const [scanModalLoading, setScanModalLoading] = useState(false);
  const [issueUpdating, setIssueUpdating] = useState<number | null>(null);

  const loadAll = async (showSpinner: boolean) => {
    if (showSpinner) setLoading(true);
    try {
      const [siteData, siteFindings] = await Promise.all([api.getSite(siteId), api.getSiteFindings(siteId)]);
      setDetail(siteData);
      setFindings(siteFindings);
      const completed = siteData.scans.find((scan) => scan.status === 'completed');
      if (completed) {
        api.getScan(completed.id).then(setLatestDetail).catch(() => setLatestDetail(null));
      } else {
        setLatestDetail(null);
      }
    } catch (error) {
      onError(error instanceof Error ? error.message : '加载网站详情失败');
    } finally {
      if (showSpinner) setLoading(false);
    }
  };

  useEffect(() => {
    void loadAll(true);
    setLatestDetail(null);
    setScanModalData(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [siteId]);

  useEffect(() => {
    const nextHash = `#/sites/${siteId}?tab=${tab}`;
    if (window.location.hash !== nextHash) {
      window.history.replaceState(null, '', nextHash);
    }
  }, [siteId, tab]);

  useEffect(() => {
    if (scanStartedAt === null) return;
    setElapsed(0);
    const timer = window.setInterval(() => setElapsed(Math.floor((Date.now() - scanStartedAt) / 1000)), 1000);
    return () => window.clearInterval(timer);
  }, [scanStartedAt]);

  const latestCompleted = detail?.scans.find((scan) => scan.status === 'completed');
  const activeFindings = findings.filter((finding) => finding.status === 'open' || finding.status === 'in_progress');
  const highPriority = activeFindings.filter((finding) => finding.severity === 'critical' || finding.severity === 'high');
  const statusCounts = useMemo(() => {
    const counts: Record<string, number> = { open: 0, in_progress: 0, resolved: 0, ignored: 0 };
    findings.forEach((finding) => {
      counts[finding.status] = (counts[finding.status] ?? 0) + 1;
    });
    return counts;
  }, [findings]);

  const runScan = async () => {
    setScanning(true);
    setScanStartedAt(Date.now());
    try {
      const result = await api.runScan(siteId);
      setScanStartedAt(null);
      await loadAll(false);
      setTab('overview');
      onChanged();
      if (result.scan.status === 'failed') {
        onError(`扫描未完成：${result.scan.errorMessage || '目标网站无法访问'}`);
      }
    } catch (error) {
      setScanStartedAt(null);
      onError(error instanceof Error ? error.message : '扫描失败');
    } finally {
      setScanning(false);
    }
  };

  const saveSite = async (input: SiteInput) => {
    setSavingSite(true);
    try {
      const updated = await api.updateSite(siteId, input);
      setDetail((current) => (current ? { ...current, site: updated } : current));
      setEditing(false);
      onChanged();
    } catch (error) {
      throw error;
    } finally {
      setSavingSite(false);
    }
  };

  const deleteSite = async () => {
    if (!window.confirm('确定删除该网站及其全部扫描记录、问题与状态历史？此操作不可撤销。')) return;
    setDeleting(true);
    try {
      await api.deleteSite(siteId);
      onDeleted();
    } catch (error) {
      onError(error instanceof Error ? error.message : '删除网站失败');
    } finally {
      setDeleting(false);
    }
  };

  const openScan = async (scan: Scan) => {
    setScanModalOpen(true);
    setScanModalLoading(true);
    setScanModalData(null);
    try {
      setScanModalData(await api.getScan(scan.id));
    } catch (error) {
      onError(error instanceof Error ? error.message : '加载扫描详情失败');
      setScanModalOpen(false);
    } finally {
      setScanModalLoading(false);
    }
  };

  const updateIssue = async (finding: Finding, status: string) => {
    setIssueUpdating(finding.id);
    try {
      const updated = await api.updateIssue(finding.id, { status });
      setFindings((current) => current.map((item) => (item.id === finding.id ? updated : item)));
      setLatestDetail((current) => (current ? { ...current, findings: current.findings.map((item) => (item.id === finding.id ? updated : item)) } : current));
      setScanModalData((current) => (current ? { ...current, findings: current.findings.map((item) => (item.id === finding.id ? updated : item)) } : current));
      onChanged();
    } catch (error) {
      onError(error instanceof Error ? error.message : '更新问题状态失败');
    } finally {
      setIssueUpdating(null);
    }
  };

  if (loading && !detail) {
    return (
      <div className="space-y-4">
        <div className="h-8 w-48 animate-pulse rounded-lg bg-slate-200" />
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {[0, 1, 2, 3].map((item) => <div key={item} className="h-32 animate-pulse rounded-2xl bg-slate-100" />)}
        </div>
        <div className="h-80 animate-pulse rounded-2xl bg-slate-100" />
      </div>
    );
  }

  if (!detail) {
    return <EmptyState title="网站不存在" description="它可能已经被删除，请返回工作台。" action={<button onClick={onBack} className="btn btn-primary"><ArrowLeft size={15} /> 返回工作台</button>} />;
  }

  const { site, scans } = detail;

  return (
    <>
      <div className="mb-6 flex flex-col justify-between gap-4 xl:flex-row xl:items-start">
        <div className="min-w-0">
          <button onClick={onBack} className="mb-3 inline-flex items-center gap-1 text-xs font-semibold text-indigo-600 hover:text-indigo-800">
            <ArrowLeft size={13} /> 返回网站列表
          </button>
          <div className="flex items-center gap-3">
            <h1 className="truncate text-2xl font-bold text-slate-900">{site.name}</h1>
            <button onClick={() => setEditing(true)} className="rounded-lg p-1.5 text-slate-400 transition hover:bg-slate-100 hover:text-slate-700" title="编辑网站">
              <Pencil size={15} />
            </button>
          </div>
          <a href={site.url} target="_blank" rel="noreferrer" className="mt-1 inline-flex max-w-full items-center gap-1 truncate text-sm text-slate-500 hover:text-indigo-600">
            <span className="truncate">{site.url}</span> <ArrowUpRight size={13} className="shrink-0" />
          </a>
          {site.description && <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-500">{site.description}</p>}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {latestCompleted && (
            <a href={reportUrl(latestCompleted.id)} target="_blank" rel="noreferrer" className="btn btn-secondary">
              <FileText size={15} /> HTML 报告
            </a>
          )}
          <button onClick={() => setEditing(true)} className="btn btn-secondary">
            <Pencil size={15} /> 编辑
          </button>
          <button onClick={runScan} disabled={scanning} className="btn btn-primary">
            {scanning ? <RefreshCw size={15} className="animate-spin" /> : <Activity size={15} />}
            {scanning ? `扫描中${elapsed ? ` ${elapsed}s` : ''}` : '运行诊断'}
          </button>
          <button onClick={deleteSite} disabled={deleting} className="rounded-lg border border-slate-200 p-2.5 text-slate-400 transition hover:border-rose-200 hover:bg-rose-50 hover:text-rose-600" title="删除网站">
            {deleting ? <Loader2 size={15} className="animate-spin" /> : <Trash2 size={15} />}
          </button>
        </div>
      </div>

      {scanning && (
        <div className="mb-5 flex items-center gap-3 rounded-xl border border-sky-100 bg-sky-50 px-4 py-3 text-sm text-sky-800">
          <Loader2 size={17} className="animate-spin" />
          <div>
            <div className="font-semibold">正在抓取并分析首页</div>
            <div className="mt-0.5 text-xs text-sky-600">已用时 {elapsed}s。扫描包含首页、robots.txt 与 sitemap.xml，通常需要数秒。</div>
          </div>
        </div>
      )}

      <div className="mb-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard
          icon={<Activity size={18} />}
          label="最近评分"
          value={latestCompleted ? `${latestCompleted.score}/100` : '—'}
          hint={latestCompleted ? `扫描 #${latestCompleted.id} · ${formatShortDate(latestCompleted.finishedAt)}` : '运行扫描后生成'}
          color="indigo"
        />
        <StatCard icon={<CircleAlert size={18} />} label="待处理问题" value={activeFindings.length} hint="待处理与处理中" color="amber" />
        <StatCard icon={<ShieldCheck size={18} />} label="高优先级" value={highPriority.length} hint="严重与高优先级问题" color="red" />
        <StatCard icon={<History size={18} />} label="扫描记录" value={scans.length} hint="保留最近 20 次扫描" color="blue" />
      </div>

      <div className="mb-5 flex gap-1 overflow-x-auto border-b border-slate-200">
        <TabButton active={tab === 'overview'} onClick={() => setTab('overview')} icon={<Activity size={15} />} label="诊断概览" />
        <TabButton active={tab === 'issues'} onClick={() => setTab('issues')} icon={<CircleAlert size={15} />} label="问题清单" count={findings.length} />
        <TabButton active={tab === 'history'} onClick={() => setTab('history')} icon={<History size={15} />} label="扫描历史" count={scans.length} />
      </div>

      {tab === 'overview' && (
        <Overview
          detail={detail}
          latestDetail={latestDetail}
          findings={findings}
          highPriority={highPriority}
          statusCounts={statusCounts}
          scanning={scanning}
          onRunScan={runScan}
          onOpenIssues={() => setTab('issues')}
          onOpenScan={openScan}
        />
      )}

      {tab === 'issues' && (
        <IssuesPanel
          siteId={siteId}
          findings={findings}
          updating={issueUpdating}
          onUpdate={updateIssue}
          onRunScan={runScan}
          scanning={scanning}
        />
      )}

      {tab === 'history' && (
        <HistoryPanel scans={scans} onOpenScan={openScan} onRunScan={runScan} scanning={scanning} />
      )}

      {editing && (
        <SiteFormModal initial={site} submitting={savingSite} onClose={() => setEditing(false)} onSubmit={saveSite} />
      )}

      {scanModalOpen && (
        <ScanDetailModal
          data={scanModalData}
          loading={scanModalLoading}
          updating={issueUpdating}
          onClose={() => setScanModalOpen(false)}
          onUpdate={updateIssue}
        />
      )}
    </>
  );
}

function TabButton({ active, onClick, icon, label, count }: { active: boolean; onClick: () => void; icon: ReactNode; label: string; count?: number }) {
  return (
    <button onClick={onClick} className={`flex shrink-0 items-center gap-2 border-b-2 px-4 py-3 text-sm font-semibold transition ${active ? 'border-indigo-600 text-indigo-700' : 'border-transparent text-slate-500 hover:text-slate-700'}`}>
      {icon} {label}
      {count !== undefined && <span className="rounded-md bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500">{count}</span>}
    </button>
  );
}

function Overview({
  detail,
  latestDetail,
  findings,
  highPriority,
  statusCounts,
  scanning,
  onRunScan,
  onOpenIssues,
  onOpenScan,
}: {
  detail: SiteDetail;
  latestDetail: ScanDetail | null;
  findings: Finding[];
  highPriority: Finding[];
  statusCounts: Record<string, number>;
  scanning: boolean;
  onRunScan: () => void;
  onOpenIssues: () => void;
  onOpenScan: (scan: Scan) => void;
}) {
  const latest = detail.scans[0];
  if (!latest) {
    return (
      <section className="panel">
        <EmptyState
          icon={<Activity size={24} />}
          title="还没有扫描记录"
          description="运行一次诊断，检查首页 HTTP 状态、元数据、结构化数据与基础文件。"
          action={<button onClick={onRunScan} disabled={scanning} className="btn btn-primary">{scanning ? <RefreshCw size={15} className="animate-spin" /> : <Activity size={15} />} 运行第一次诊断</button>}
        />
      </section>
    );
  }
  const completed = detail.scans.find((scan) => scan.status === 'completed');
  const severityCounts = latestDetail?.severityCounts ?? { critical: 0, high: 0, medium: 0, low: 0 };
  const totalSeverity = Object.values(severityCounts).reduce((sum, value) => sum + value, 0);
  const comparison = latestDetail?.comparison;
  return (
    <div className="space-y-5">
      {latest.status === 'failed' && <ScanFailureNote scan={latest} />}

      <div className="grid gap-5 xl:grid-cols-[1.35fr_1fr]">
        <section className="panel p-5">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h2 className="font-bold text-slate-900">最近扫描</h2>
              <p className="mt-1 text-xs text-slate-500">扫描 #{latest.id} · {formatDate(latest.finishedAt || latest.startedAt)}</p>
            </div>
            <div className="flex items-center gap-2">
              <ScanStatusBadge status={latest.status} />
              <button onClick={() => onOpenScan(latest)} className="text-xs font-semibold text-indigo-600 hover:text-indigo-800">详情</button>
            </div>
          </div>
          {latest.status === 'completed' ? (
            <>
              <div className="mt-5 flex items-center gap-5 rounded-2xl bg-slate-50 p-4">
                <ScoreRing score={latest.score} size={86} />
                <div className="min-w-0">
                  <div className="text-sm font-semibold text-slate-800">GeoPilot 规则评分</div>
                  <p className="mt-1 text-xs leading-5 text-slate-500">{latest.summary}</p>
                  <div className="mt-2 flex flex-wrap gap-1.5 text-[11px] text-slate-400">
                    <span>规则 {latest.ruleVersion || '—'}</span>
                    <span>·</span>
                    <span>评分 {latest.scoreVersion || '—'}</span>
                    <span>·</span>
                    <span>{latest.pageCount} 页</span>
                  </div>
                </div>
              </div>
              <div className="mt-5">
                <div className="mb-3 flex items-center justify-between">
                  <h3 className="text-sm font-semibold text-slate-700">问题分布</h3>
                  <button onClick={onOpenIssues} className="text-xs font-semibold text-indigo-600 hover:text-indigo-800">查看问题 →</button>
                </div>
                <SeverityDistribution counts={severityCounts} total={totalSeverity} />
              </div>
              {comparison && comparison.baseScanId > 0 && (
                <div className="mt-5 grid grid-cols-3 gap-2 rounded-xl border border-slate-100 p-3 text-center">
                  <CompareMetric label="本次新增" value={comparison.newCount} tone="text-rose-600" />
                  <CompareMetric label="持续存在" value={comparison.persistingCount} tone="text-amber-600" />
                  <CompareMetric label="未再出现" value={comparison.resolvedCount} tone="text-emerald-600" />
                </div>
              )}
            </>
          ) : (
            <div className="mt-5 rounded-xl border border-slate-100 bg-slate-50 p-4 text-sm text-slate-600">
              {latest.errorMessage || '该次扫描未产生可用结果，请重新运行诊断。'}
            </div>
          )}
        </section>

        <section className="panel p-5">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="font-bold text-slate-900">整改进度</h2>
              <p className="mt-1 text-xs text-slate-500">基于最近一次成功扫描的问题状态</p>
            </div>
            <Wrench size={18} className="text-slate-400" />
          </div>
          <div className="mt-5 space-y-3">
            {issueStatusOptions.map((status) => {
              const count = statusCounts[status] ?? 0;
              const total = Math.max(findings.length, 1);
              const meta = issueStatusMeta[status];
              const color = status === 'resolved' ? 'bg-emerald-500' : status === 'in_progress' ? 'bg-sky-500' : status === 'ignored' ? 'bg-slate-300' : 'bg-rose-500';
              return (
                <div key={status} className="flex items-center gap-3">
                  <span className="w-14 text-xs font-semibold text-slate-500">{meta.label}</span>
                  <div className="h-2 flex-1 overflow-hidden rounded-full bg-slate-100">
                    <div className={`h-full rounded-full ${color}`} style={{ width: `${(count / total) * 100}%` }} />
                  </div>
                  <span className="w-6 text-right text-xs font-semibold text-slate-600">{count}</span>
                </div>
              );
            })}
          </div>
          <div className="mt-5 rounded-xl bg-slate-50 p-3 text-xs leading-5 text-slate-500">
            标记“已修复”代表人工确认，不等同于系统已验证。重新扫描仍发现同一问题时，系统会自动重新打开并记录。
          </div>
        </section>
      </div>

      <div className="grid gap-5 xl:grid-cols-[1.35fr_1fr]">
        <section className="panel p-5">
          <h2 className="font-bold text-slate-900">优先整改</h2>
          <p className="mt-1 text-xs text-slate-500">先处理阻断抓取与理解的问题</p>
          {highPriority.length === 0 ? (
            <EmptyState icon={<CheckCircle2 size={20} />} title="暂无高优先级待办" description="当前扫描没有严重或高优先级问题，仍建议定期复查。" />
          ) : (
            <div className="mt-4 space-y-3">
              {highPriority.slice(0, 5).map((finding) => (
                <div key={finding.id} className="rounded-xl border border-slate-100 p-3">
                  <div className="flex items-center gap-2">
                    <SeverityBadge severity={finding.severity} />
                    <span className="text-xs text-slate-400">{finding.ruleCode}</span>
                  </div>
                  <div className="mt-2 text-sm font-semibold text-slate-800">{finding.title}</div>
                  <div className="mt-1 text-xs leading-5 text-slate-500">{finding.recommendation}</div>
                </div>
              ))}
            </div>
          )}
        </section>

        <section className="panel p-5">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="font-bold text-slate-900">扫描轨迹</h2>
              <p className="mt-1 text-xs text-slate-500">最近 5 次扫描的评分变化</p>
            </div>
            <History size={18} className="text-slate-400" />
          </div>
          <div className="mt-4 space-y-3">
            {detail.scans.slice(0, 5).map((scan, index) => {
              const older = detail.scans[index + 1];
              const delta = older && older.status === 'completed' && scan.status === 'completed' ? scan.score - older.score : null;
              return (
                <button key={scan.id} onClick={() => onOpenScan(scan)} className="flex w-full items-center justify-between gap-3 rounded-xl px-2 py-2 text-left transition hover:bg-slate-50">
                  <div>
                    <div className="text-sm font-semibold text-slate-700">扫描 #{scan.id}</div>
                    <div className="mt-0.5 text-xs text-slate-400">{formatDate(scan.finishedAt || scan.startedAt)}</div>
                  </div>
                  <div className="flex items-center gap-2">
                    {delta !== null && delta !== 0 && (
                      <span className={`inline-flex items-center text-xs font-semibold ${delta > 0 ? 'text-emerald-600' : 'text-rose-600'}`}>
                        {delta > 0 ? <ArrowUpRight size={13} /> : <ArrowDownRight size={13} />}{Math.abs(delta)}
                      </span>
                    )}
                    {scan.status === 'completed' ? <span className="text-sm font-bold text-slate-700">{scan.score}</span> : <ScanStatusBadge status={scan.status} />}
                  </div>
                </button>
              );
            })}
          </div>
          {completed && (
            <a href={reportUrl(completed.id)} target="_blank" rel="noreferrer" className="btn btn-secondary mt-4 w-full">
              <FileText size={15} /> 打开最近一次 HTML 报告
            </a>
          )}
        </section>
      </div>
    </div>
  );
}

function CompareMetric({ label, value, tone }: { label: string; value: number; tone: string }) {
  return (
    <div>
      <div className={`text-lg font-bold ${tone}`}>{value}</div>
      <div className="mt-0.5 text-[11px] text-slate-400">{label}</div>
    </div>
  );
}

function IssuesPanel({
  siteId,
  findings,
  updating,
  onUpdate,
  onRunScan,
  scanning,
}: {
  siteId: number;
  findings: Finding[];
  updating: number | null;
  onUpdate: (finding: Finding, status: string) => void;
  onRunScan: () => void;
  scanning: boolean;
}) {
  const [severity, setSeverity] = useState('all');
  const [status, setStatus] = useState('all');
  const [search, setSearch] = useState('');
  const filtered = useMemo(
    () =>
      findings.filter((finding) => {
        if (severity !== 'all' && finding.severity !== severity) return false;
        if (status !== 'all' && finding.status !== status) return false;
        if (search) {
          const haystack = `${finding.title} ${finding.ruleCode} ${finding.evidence}`.toLowerCase();
          if (!haystack.includes(search.toLowerCase())) return false;
        }
        return true;
      }),
    [findings, severity, status, search],
  );

  if (findings.length === 0) {
    return (
      <section className="panel">
        <EmptyState
          icon={<CheckCircle2 size={24} />}
          title="暂无问题记录"
          description="运行一次诊断以生成问题清单；如果扫描失败，请检查目标是否可访问。"
          action={<button onClick={onRunScan} disabled={scanning} className="btn btn-primary"><RefreshCw size={15} /> 运行诊断</button>}
        />
      </section>
    );
  }

  return (
    <section className="panel overflow-hidden">
      <div className="border-b border-slate-100 p-5">
        <div className="flex flex-col justify-between gap-3 lg:flex-row lg:items-center">
          <div>
            <h2 className="font-bold text-slate-900">诊断问题</h2>
            <p className="mt-1 text-xs text-slate-500">显示最近一次成功扫描的问题，并保留跨扫描的首次发现时间。网站 #{siteId}</p>
          </div>
          <div className="flex flex-col gap-2 sm:flex-row">
            <div className="relative">
              <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
              <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索标题、规则或证据" className="field !py-2 pl-9 text-sm sm:w-56" />
            </div>
            <select value={severity} onChange={(event) => setSeverity(event.target.value)} className="field !w-auto !py-2 text-sm">
              <option value="all">全部级别</option>
              <option value="critical">严重</option>
              <option value="high">高</option>
              <option value="medium">中</option>
              <option value="low">低</option>
            </select>
            <select value={status} onChange={(event) => setStatus(event.target.value)} className="field !w-auto !py-2 text-sm">
              <option value="all">全部状态</option>
              {issueStatusOptions.map((value) => <option key={value} value={value}>{issueStatusMeta[value].label}</option>)}
            </select>
          </div>
        </div>
        <div className="mt-3 text-xs text-slate-400">显示 {filtered.length} / {findings.length} 项</div>
      </div>
      {filtered.length === 0 ? (
        <EmptyState title="没有匹配的问题" description="调整筛选条件或搜索关键词后再试。" />
      ) : (
        <div className="divide-y divide-slate-100">
          {filtered.map((finding) => (
            <IssueCard key={finding.id} finding={finding} updating={updating === finding.id} onUpdate={onUpdate} />
          ))}
        </div>
      )}
    </section>
  );
}

function IssueCard({ finding, updating, onUpdate }: { finding: Finding; updating: boolean; onUpdate: (finding: Finding, status: string) => void }) {
  const meta = severityMeta[finding.severity] ?? severityMeta.low;
  return (
    <article className="p-5">
      <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-start">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <SeverityBadge severity={finding.severity} />
            <IssueStatusBadge status={finding.status} />
            <span className="text-xs text-slate-400">规则 {finding.ruleCode}</span>
            <span className="text-xs text-slate-400">扣分 {finding.scorePenalty}</span>
          </div>
          <h3 className="mt-2 font-semibold text-slate-900">{finding.title}</h3>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {updating && <Loader2 size={14} className="animate-spin text-slate-400" />}
          <IssueStatusSelect finding={finding} disabled={updating} onChange={(status) => onUpdate(finding, status)} />
        </div>
      </div>
      <div className="mt-3 grid gap-3 lg:grid-cols-2">
        <div className={`rounded-xl border p-3 ${meta.badge}`}>
          <div className="text-[11px] font-bold uppercase tracking-wide opacity-70">证据</div>
          <p className="mt-1 break-words text-sm leading-6">{finding.evidence || '—'}</p>
        </div>
        <div className="rounded-xl border border-slate-100 bg-slate-50 p-3">
          <div className="text-[11px] font-bold uppercase tracking-wide text-slate-400">整改建议</div>
          <p className="mt-1 text-sm leading-6 text-slate-600">{finding.recommendation || '—'}</p>
        </div>
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px] text-slate-400">
        <span className="inline-flex items-center gap-1"><Clock3 size={12} /> 首次发现 {formatDate(finding.firstSeenAt)}</span>
        <span>最近发现 {formatDate(finding.lastSeenAt)}</span>
        <span className="truncate">页面 {finding.url}</span>
      </div>
    </article>
  );
}

function HistoryPanel({ scans, onOpenScan, onRunScan, scanning }: { scans: Scan[]; onOpenScan: (scan: Scan) => void; onRunScan: () => void; scanning: boolean }) {
  if (scans.length === 0) {
    return (
      <section className="panel">
        <EmptyState title="还没有扫描历史" description="每次运行诊断都会生成独立记录，不会覆盖旧结果。" action={<button onClick={onRunScan} disabled={scanning} className="btn btn-primary"><Activity size={15} /> 运行诊断</button>} />
      </section>
    );
  }
  return (
    <section className="panel overflow-hidden">
      <div className="flex flex-col justify-between gap-3 border-b border-slate-100 p-5 sm:flex-row sm:items-center">
        <div>
          <h2 className="font-bold text-slate-900">扫描历史</h2>
          <p className="mt-1 text-xs text-slate-500">最多显示最近 20 次扫描，旧结果不会被新扫描覆盖</p>
        </div>
        <button onClick={onRunScan} disabled={scanning} className="btn btn-secondary self-start sm:self-auto"><RefreshCw size={14} className={scanning ? 'animate-spin' : ''} /> 新扫描</button>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[820px] text-left text-sm">
          <thead className="bg-slate-50/80 text-xs text-slate-500">
            <tr>
              <th className="px-5 py-3 font-semibold">扫描</th>
              <th className="px-5 py-3 font-semibold">时间</th>
              <th className="px-5 py-3 font-semibold">评分</th>
              <th className="px-5 py-3 font-semibold">问题</th>
              <th className="px-5 py-3 font-semibold">状态</th>
              <th className="px-5 py-3 text-right font-semibold">操作</th>
            </tr>
          </thead>
          <tbody>
            {scans.map((scan) => (
              <tr key={scan.id} className="table-row hover:bg-slate-50/70">
                <td className="px-5 py-4 font-semibold text-slate-800">#{scan.id}</td>
                <td className="px-5 py-4 text-xs text-slate-500">{formatDate(scan.finishedAt || scan.startedAt)}</td>
                <td className="px-5 py-4">{scan.status === 'completed' ? <span className="font-bold text-slate-800">{scan.score}<span className="text-[10px] font-normal text-slate-400">/100</span></span> : <span className="text-slate-400">—</span>}</td>
                <td className="px-5 py-4 text-xs text-slate-500">
                  {scan.status === 'completed' ? <>{scan.issueCount} 项{scan.highIssueCount > 0 && <span className="ml-1 text-rose-500">（高 {scan.highIssueCount}）</span>}</> : scan.errorMessage ? <span className="line-clamp-1 block max-w-[220px] truncate text-rose-500">{scan.errorMessage}</span> : '—'}
                </td>
                <td className="px-5 py-4"><ScanStatusBadge status={scan.status} /></td>
                <td className="px-5 py-4">
                  <div className="flex justify-end gap-2">
                    <button onClick={() => onOpenScan(scan)} className="btn btn-secondary !px-3 !py-2">详情</button>
                    {scan.status === 'completed' && (
                      <>
                        <a href={reportUrl(scan.id)} target="_blank" rel="noreferrer" className="rounded-lg p-2 text-slate-400 hover:bg-slate-100 hover:text-indigo-600" title="HTML 报告"><FileText size={15} /></a>
                        <a href={reportJsonUrl(scan.id)} className="rounded-lg p-2 text-slate-400 hover:bg-slate-100 hover:text-indigo-600" title="JSON 导出"><FileJson size={15} /></a>
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function ScanDetailModal({
  data,
  loading,
  updating,
  onClose,
  onUpdate,
}: {
  data: ScanDetail | null;
  loading: boolean;
  updating: number | null;
  onClose: () => void;
  onUpdate: (finding: Finding, status: string) => void;
}) {
  return (
    <Modal title={data ? `扫描 #${data.scan.id}` : '扫描详情'} subtitle={data ? `${data.site.name} · ${formatDate(data.scan.finishedAt || data.scan.startedAt)}` : '正在加载扫描结果…'} onClose={onClose} wide>
      {loading || !data ? (
        <div className="space-y-3 p-6">
          {[0, 1, 2].map((item) => <div key={item} className="h-20 animate-pulse rounded-xl bg-slate-100" />)}
        </div>
      ) : (
        <div className="space-y-5 p-6">
          {data.scan.status === 'failed' ? (
            <ScanFailureNote scan={data.scan} />
          ) : (
            <div className="flex flex-col gap-5 rounded-2xl bg-slate-50 p-5 sm:flex-row sm:items-center">
              <ScoreRing score={data.scan.score} size={84} />
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <ScanStatusBadge status={data.scan.status} />
                  <span className="text-xs text-slate-400">规则 {data.scan.ruleVersion || '—'}</span>
                  <span className="text-xs text-slate-400">评分算法 {data.scan.scoreVersion || '—'}</span>
                </div>
                <p className="mt-2 text-sm leading-6 text-slate-600">{data.scan.summary}</p>
                {(data.scan.finalUrl || data.scan.httpStatus) && (
                  <div className="mt-2 flex flex-wrap items-center gap-2 text-[11px] text-slate-400">
                    {data.scan.httpStatus ? <span className="rounded-md bg-white px-2 py-1 font-semibold text-slate-600">HTTP {data.scan.httpStatus}</span> : null}
                    {data.scan.finalUrl && <span className="max-w-full truncate">最终 URL {data.scan.finalUrl}</span>}
                  </div>
                )}
                <div className="mt-3 flex flex-wrap gap-2">
                  <a href={reportUrl(data.scan.id)} target="_blank" rel="noreferrer" className="btn btn-secondary !px-3 !py-2"><FileText size={14} /> HTML 报告</a>
                  <a href={reportJsonUrl(data.scan.id)} className="btn btn-secondary !px-3 !py-2"><FileJson size={14} /> JSON 导出</a>
                </div>
              </div>
            </div>
          )}

          {data.scan.status === 'completed' && (
            <>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                {(Object.keys(severityMeta) as string[]).map((severity) => (
                  <div key={severity} className="rounded-xl border border-slate-100 p-3">
                    <div className={`text-xs font-semibold ${severityMeta[severity].text}`}>{severityMeta[severity].label}</div>
                    <div className="mt-1 text-xl font-bold text-slate-800">{data.severityCounts[severity] ?? 0}</div>
                  </div>
                ))}
              </div>
              {data.comparison.baseScanId > 0 && (
                <div className="flex flex-wrap items-center gap-2 rounded-xl border border-slate-100 p-3 text-xs text-slate-600">
                  <span className="font-semibold text-slate-700">对比扫描 #{data.comparison.baseScanId}</span>
                  <span className="rounded-md bg-rose-50 px-2 py-1 text-rose-700">新增 {data.comparison.newCount}</span>
                  <span className="rounded-md bg-amber-50 px-2 py-1 text-amber-700">持续 {data.comparison.persistingCount}</span>
                  <span className="rounded-md bg-emerald-50 px-2 py-1 text-emerald-700">未再出现 {data.comparison.resolvedCount}</span>
                </div>
              )}
              <div>
                <h3 className="mb-3 text-sm font-semibold text-slate-700">问题明细（{data.findings.length}）</h3>
                {data.findings.length === 0 ? (
                  <div className="rounded-xl bg-emerald-50 p-4 text-sm text-emerald-700">本次扫描未发现规则命中的问题。</div>
                ) : (
                  <div className="space-y-2">
                    {data.findings.map((finding) => (
                      <div key={finding.id} className="rounded-xl border border-slate-100 p-3">
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <div className="flex flex-wrap items-center gap-2">
                            <SeverityBadge severity={finding.severity} />
                            <span className="text-xs text-slate-400">{finding.ruleCode}</span>
                            <span className="text-sm font-semibold text-slate-800">{finding.title}</span>
                          </div>
                          <IssueStatusSelect finding={finding} disabled={updating === finding.id} onChange={(status) => onUpdate(finding, status)} />
                        </div>
                        <p className="mt-2 text-xs leading-5 text-slate-500">{finding.evidence}</p>
                        <p className="mt-1 text-xs leading-5 text-slate-600"><span className="font-semibold">建议：</span>{finding.recommendation}</p>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </>
          )}

          {data.comparison.resolved.length > 0 && (
            <div>
              <h3 className="mb-2 text-sm font-semibold text-slate-700">较上次未再出现（{data.comparison.resolved.length}）</h3>
              <div className="space-y-1.5">
                {data.comparison.resolved.map((finding) => (
                  <div key={finding.id} className="flex items-center gap-2 rounded-lg bg-emerald-50/70 px-3 py-2 text-xs text-emerald-800">
                    <CheckCircle2 size={13} className="shrink-0" />
                    <span className="font-semibold">{finding.title}</span>
                    <span className="text-emerald-600">{finding.ruleCode}</span>
                  </div>
                ))}
              </div>
              <p className="mt-2 text-[11px] text-slate-400">“未再出现”仅表示本次抓取未命中同一规则，仍需结合页面改版与规则版本人工确认。</p>
            </div>
          )}

          <div className="flex items-start gap-2 rounded-xl bg-slate-50 p-3 text-[11px] leading-5 text-slate-500">
            <Plus size={13} className="mt-0.5 shrink-0" />
            扫描只读取公开 HTTP 响应，不执行 JavaScript。外部内容已转义；报告不构成任何 AI 平台收录或引用保证。
          </div>
        </div>
      )}
    </Modal>
  );
}
