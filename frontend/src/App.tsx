import { useEffect, useState } from 'react';
import { ChevronRight, Globe2, LayoutDashboard, Plus, ShieldCheck, Sparkles, X } from 'lucide-react';
import { api, waitForScan } from './api';
import type { DashboardData, DashboardSite, SiteInput } from './types';
import { Dashboard } from './Dashboard';
import { SitePage } from './SitePage';
import { SiteFormModal } from './components';

function siteIdFromHash(): number | null {
  const match = window.location.hash.match(/^#\/sites\/(\d+)/);
  if (!match) return null;
  const value = Number(match[1]);
  return Number.isFinite(value) && value > 0 ? value : null;
}

export default function App() {
  const [dashboard, setDashboard] = useState<DashboardData | null>(null);
  const [loadingDashboard, setLoadingDashboard] = useState(true);
  const [selectedSiteId, setSelectedSiteId] = useState<number | null>(() => siteIdFromHash());
  const [showAdd, setShowAdd] = useState(false);
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState('');
  const [scanningSiteId, setScanningSiteId] = useState<number | null>(null);

  const loadDashboard = async (showSpinner = true) => {
    if (showSpinner) setLoadingDashboard(true);
    try {
      setDashboard(await api.dashboard());
    } catch (err) {
      setError(err instanceof Error ? err.message : '无法加载工作台数据');
    } finally {
      if (showSpinner) setLoadingDashboard(false);
    }
  };

  useEffect(() => {
    void loadDashboard();
  }, []);

  useEffect(() => {
    const onHashChange = () => setSelectedSiteId(siteIdFromHash());
    window.addEventListener('hashchange', onHashChange);
    return () => window.removeEventListener('hashchange', onHashChange);
  }, []);

  useEffect(() => {
    if (!error) return;
    const timer = window.setTimeout(() => setError(''), 10000);
    return () => window.clearTimeout(timer);
  }, [error]);

  useEffect(() => {
    const hasActiveScan = dashboard?.sites.some((site) => site.latestScan?.status === 'queued' || site.latestScan?.status === 'running');
    if (!hasActiveScan) return;
    const timer = window.setInterval(() => void loadDashboard(false), 1500);
    return () => window.clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dashboard]);

  const navigate = (siteId: number | null) => {
    setSelectedSiteId(siteId);
    window.location.hash = siteId ? `#/sites/${siteId}` : '#/';
  };

  const addSite = async (input: SiteInput) => {
    setAdding(true);
    try {
      const site = await api.createSite(input);
      await loadDashboard(false);
      setShowAdd(false);
      navigate(site.id);
    } finally {
      setAdding(false);
    }
  };

  const runScan = async (site: DashboardSite) => {
    setScanningSiteId(site.id);
    setError('');
    try {
      const queued = await api.runScan(site.id);
      await loadDashboard(false);
      const detail = await waitForScan(queued.scan.id);
      await loadDashboard(false);
      if (detail.scan.status === 'failed') {
        setError(`扫描未完成：${detail.scan.errorMessage || '目标网站无法访问'}`);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '扫描失败');
    } finally {
      setScanningSiteId(null);
    }
  };

  const deleteSite = async (site: DashboardSite) => {
    if (!window.confirm(`确定删除「${site.name}」及其全部扫描记录、问题与状态历史？此操作不可撤销。`)) return;
    setError('');
    try {
      await api.deleteSite(site.id);
      if (selectedSiteId === site.id) navigate(null);
      await loadDashboard(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除网站失败');
    }
  };

  const selectedSite = dashboard?.sites.find((site) => site.id === selectedSiteId);
  const pageTitle = selectedSiteId ? '网站诊断' : '网站诊断工作台';

  return (
    <div className="min-h-screen bg-slate-50">
      <aside className="fixed inset-y-0 left-0 z-20 hidden w-64 border-r border-slate-200 bg-white lg:flex lg:flex-col">
        <div className="flex h-[76px] items-center gap-3 border-b border-slate-100 px-6">
          <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-indigo-600 text-white"><Sparkles size={19} /></div>
          <div>
            <div className="text-[17px] font-bold tracking-tight">GeoPilot</div>
            <div className="text-[11px] text-slate-400">GEO SITE INTELLIGENCE</div>
          </div>
        </div>
        <div className="px-4 pt-6">
          <div className="px-3 pb-2 text-[11px] font-bold uppercase tracking-[.13em] text-slate-400">工作空间</div>
          <button onClick={() => navigate(null)} className={`mb-1 flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-semibold ${!selectedSiteId ? 'bg-indigo-50 text-indigo-700' : 'text-slate-600 hover:bg-slate-50'}`}>
            <LayoutDashboard size={17} /> 总览
          </button>
          <button onClick={() => setShowAdd(true)} className="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-semibold text-slate-600 hover:bg-slate-50">
            <Globe2 size={17} /> 添加网站 <span className="ml-auto rounded-md bg-slate-100 px-2 py-0.5 text-xs">{dashboard?.stats.siteCount ?? 0}</span>
          </button>
        </div>
        <div className="mt-6 flex-1 overflow-y-auto px-4">
          <div className="flex items-center justify-between px-3 pb-2">
            <span className="text-[11px] font-bold uppercase tracking-[.13em] text-slate-400">我的网站</span>
            <button onClick={() => setShowAdd(true)} className="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-indigo-600" title="添加网站"><Plus size={14} /></button>
          </div>
          {dashboard?.sites.length ? (
            <div className="space-y-1">
              {dashboard.sites.slice(0, 8).map((site) => (
                <button key={site.id} onClick={() => navigate(site.id)} className={`flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm transition ${selectedSiteId === site.id ? 'bg-slate-100 font-semibold text-slate-900' : 'text-slate-600 hover:bg-slate-50'}`}>
                  <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${site.latestScan?.status === 'failed' ? 'bg-rose-500' : site.latestScan?.status === 'completed' ? 'bg-emerald-500' : site.latestScan?.status === 'queued' ? 'bg-amber-400' : site.latestScan?.status === 'running' ? 'bg-sky-500 animate-pulse' : 'bg-slate-300'}`} />
                  <span className="min-w-0 flex-1 truncate">{site.name}</span>
                  {site.latestScan?.status === 'completed' && <span className="text-[11px] font-semibold text-slate-400">{site.latestScan.score}</span>}
                </button>
              ))}
            </div>
          ) : (
            <p className="px-3 text-xs leading-5 text-slate-400">还没有网站。添加第一个目标后，这里会显示快捷入口。</p>
          )}
        </div>
        <div className="border-t border-slate-100 p-5">
          <div className="rounded-xl bg-slate-50 p-3">
            <div className="flex items-center gap-2 text-xs font-semibold text-slate-700"><ShieldCheck size={15} className="text-emerald-600" /> 轻量诊断引擎</div>
            <p className="mt-1.5 text-xs leading-5 text-slate-500">基于可验证的页面证据生成整改建议，问题状态会跨扫描保留。</p>
          </div>
          <div className="mt-4 text-[11px] text-slate-400">GeoPilot MVP · v0.4.0</div>
        </div>
      </aside>

      <main className="lg:pl-64">
        <header className="sticky top-0 z-10 flex h-[76px] items-center justify-between border-b border-slate-200/80 bg-white/90 px-5 backdrop-blur md:px-9">
          <div className="min-w-0">
            <div className="flex items-center gap-1 text-xs text-slate-400">
              <button onClick={() => navigate(null)} className="hover:text-indigo-600">工作空间</button>
              <ChevronRight size={12} />
              <span className="truncate">{selectedSiteId ? selectedSite?.name || '网站详情' : '总览'}</span>
            </div>
            <div className="mt-1 text-lg font-bold">{pageTitle}</div>
          </div>
          <div className="flex items-center gap-2">
            <span className="hidden items-center gap-1.5 rounded-full bg-emerald-50 px-3 py-1.5 text-xs font-semibold text-emerald-700 sm:flex">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" /> 本地工作区
            </span>
            <button onClick={() => setShowAdd(true)} className="btn btn-primary"><Plus size={16} /> 添加网站</button>
          </div>
        </header>

        <div className="mx-auto max-w-[1440px] p-5 md:p-9">
          {error && (
            <div className="mb-5 flex items-start justify-between gap-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
              <span className="leading-6">{error}</span>
              <button onClick={() => setError('')} className="mt-0.5 shrink-0 rounded p-1 hover:bg-amber-100" aria-label="关闭提示"><X size={16} /></button>
            </div>
          )}

          {selectedSiteId ? (
            <SitePage
              key={selectedSiteId}
              siteId={selectedSiteId}
              onBack={() => navigate(null)}
              onError={setError}
              onChanged={() => void loadDashboard(false)}
              onDeleted={() => { navigate(null); void loadDashboard(false); }}
            />
          ) : (
            <Dashboard
              data={dashboard}
              loading={loadingDashboard}
              scanningSiteId={scanningSiteId}
              onAdd={() => setShowAdd(true)}
              onOpenSite={navigate}
              onRunScan={runScan}
              onDeleteSite={deleteSite}
            />
          )}
        </div>
      </main>

      {showAdd && <SiteFormModal submitting={adding} onClose={() => setShowAdd(false)} onSubmit={addSite} />}
    </div>
  );
}
