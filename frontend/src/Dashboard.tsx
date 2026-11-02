import { useMemo, useState } from 'react';
import {
  Activity,
  ArrowUpRight,
  CalendarClock,
  CheckCircle2,
  CircleAlert,
  Globe2,
  LayoutDashboard,
  Plus,
  RefreshCw,
  Search,
  ShieldCheck,
  Sparkles,
  Trash2,
} from 'lucide-react';
import type { DashboardData, DashboardSite } from './types';
import { EmptyState, HealthNote, ScoreRing, ScanStatusBadge, StatCard, formatDate, formatShortDate } from './components';

export function Dashboard({
  data,
  loading,
  scanningSiteId,
  onAdd,
  onOpenSite,
  onRunScan,
  onDeleteSite,
}: {
  data: DashboardData | null;
  loading: boolean;
  scanningSiteId: number | null;
  onAdd: () => void;
  onOpenSite: (id: number) => void;
  onRunScan: (site: DashboardSite) => void;
  onDeleteSite: (site: DashboardSite) => void;
}) {
  const [search, setSearch] = useState('');
  const sites = data?.sites ?? [];
  const filtered = useMemo(
    () => sites.filter((site) => `${site.name} ${site.url}`.toLowerCase().includes(search.toLowerCase())),
    [sites, search],
  );
  const stats = data?.stats;

  return (
    <>
      <div className="mb-8 flex flex-col justify-between gap-4 md:flex-row md:items-end">
        <div>
          <div className="mb-2 inline-flex items-center gap-1.5 rounded-full bg-indigo-50 px-2.5 py-1 text-xs font-semibold text-indigo-700">
            <Sparkles size={13} /> GEO 技术诊断
          </div>
          <h1 className="text-3xl font-bold tracking-tight text-slate-900">让网站更容易被理解</h1>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-500">
            检查网站结构、页面元数据与机器可读性，跟踪问题整改状态，并保留每一次可复核的诊断结果。
          </p>
        </div>
        <button onClick={onAdd} className="btn btn-primary self-start md:self-auto">
          <Plus size={16} /> 添加网站
        </button>
      </div>

      <div className="mb-7 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard icon={<Globe2 size={18} />} label="管理的网站" value={stats?.siteCount ?? '—'} hint="已添加到工作空间" color="indigo" />
        <StatCard icon={<Activity size={18} />} label="累计扫描" value={stats?.scanCount ?? '—'} hint="包含成功与失败记录" color="blue" />
        <StatCard icon={<CircleAlert size={18} />} label="待处理问题" value={stats?.openIssues ?? '—'} hint={`其中高优先级 ${stats?.highPriority ?? 0} 项`} color="amber" />
        <StatCard
          icon={<ShieldCheck size={18} />}
          label="网站平均评分"
          value={stats?.averageScore ? `${stats.averageScore}/100` : '—'}
          hint={stats?.averageScore ? '按各站最近一次成功扫描计算' : '完成扫描后生成'}
          color="green"
        />
      </div>

      <div className="panel overflow-hidden">
        <div className="flex flex-col gap-4 border-b border-slate-100 p-5 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 className="font-bold text-slate-900">我的网站</h2>
            <p className="mt-1 text-xs text-slate-500">管理目标站点、运行诊断并查看最近状态</p>
          </div>
          <div className="relative w-full sm:w-72">
            <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
            <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索网站名称或域名" className="field pl-9" />
          </div>
        </div>

        {loading && sites.length === 0 ? (
          <div className="space-y-3 p-5">
            {[0, 1, 2].map((item) => (
              <div key={item} className="h-16 animate-pulse rounded-xl bg-slate-100" />
            ))}
          </div>
        ) : filtered.length === 0 ? (
          <EmptyState
            icon={<Globe2 size={25} />}
            title={sites.length ? '没有找到匹配的网站' : '还没有添加网站'}
            description={sites.length ? '换一个名称或域名关键词再试。' : '添加一个公开可访问的网站，即可运行第一轮 GEO 技术诊断。'}
            action={!sites.length ? <button onClick={onAdd} className="btn btn-primary"><Plus size={16} /> 添加网站</button> : undefined}
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[760px] text-left text-sm">
              <thead className="bg-slate-50/80 text-xs text-slate-500">
                <tr>
                  <th className="px-5 py-3 font-semibold">网站</th>
                  <th className="px-5 py-3 font-semibold">最近评分</th>
                  <th className="px-5 py-3 font-semibold">状态</th>
                  <th className="px-5 py-3 font-semibold">待处理</th>
                  <th className="px-5 py-3 text-right font-semibold">操作</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((site) => {
                  const scan = site.latestScan;
                  return (
                    <tr key={site.id} className="table-row hover:bg-slate-50/70">
                      <td className="px-5 py-4">
                        <button onClick={() => onOpenSite(site.id)} className="flex items-center gap-3 text-left">
                          <div className="flex h-10 w-10 items-center justify-center rounded-xl border border-slate-100 bg-white text-indigo-600">
                            <Globe2 size={18} />
                          </div>
                          <span>
                            <span className="block font-semibold text-slate-800">{site.name}</span>
                            <span className="mt-1 block max-w-[320px] truncate text-xs text-slate-500">{site.url}</span>
                          </span>
                        </button>
                      </td>
                      <td className="px-5 py-4">
                        {scan?.status === 'completed' ? (
                          <div className="flex items-center gap-3">
                            <ScoreRing score={scan.score} size={46} />
                            <div className="text-xs text-slate-400">{formatShortDate(scan.finishedAt || scan.startedAt || scan.createdAt)}</div>
                          </div>
                        ) : scan ? (
                          <span className={`text-xs font-semibold ${scan.status === 'failed' ? 'text-rose-500' : scan.status === 'queued' ? 'text-amber-600' : 'text-sky-600'}`}>
                            {scan.status === 'failed' ? '扫描失败' : scan.status === 'queued' ? '等待执行' : '扫描中'}
                          </span>
                        ) : (
                          <span className="text-slate-400">尚未诊断</span>
                        )}
                      </td>
                      <td className="px-5 py-4">{scan ? <ScanStatusBadge status={scan.status} /> : <span className="pill bg-slate-100 text-slate-500">待扫描</span>}</td>
                      <td className="px-5 py-4">
                        {site.openIssues > 0 ? (
                          <div>
                            <div className="font-semibold text-slate-800">{site.openIssues}</div>
                            <div className="mt-0.5 text-[11px] text-slate-400">高优先级 {site.highPriority}</div>
                          </div>
                        ) : site.latestScan?.status === 'completed' ? (
                          <span className="inline-flex items-center gap-1 text-xs text-emerald-600"><CheckCircle2 size={13} /> 无待办</span>
                        ) : (
                          <span className="text-xs text-slate-400">—</span>
                        )}
                      </td>
                      <td className="px-5 py-4">
                        <div className="flex justify-end gap-2">
                          <button
                            onClick={() => onRunScan(site)}
                            disabled={scanningSiteId === site.id || scan?.status === 'queued' || scan?.status === 'running'}
                            className="btn btn-secondary !px-3 !py-2"
                          >
                            {scanningSiteId === site.id || scan?.status === 'queued' || scan?.status === 'running' ? <RefreshCw size={14} className="animate-spin" /> : <Activity size={14} />}
                            {scanningSiteId === site.id || scan?.status === 'running' ? '扫描中' : scan?.status === 'queued' ? '排队中' : '诊断'}
                          </button>
                          <button onClick={() => onOpenSite(site.id)} className="btn btn-secondary !px-3 !py-2">详情 <ArrowUpRight size={14} /></button>
                          <button onClick={() => onDeleteSite(site)} className="rounded-lg p-2 text-slate-400 transition hover:bg-rose-50 hover:text-rose-600" title="删除网站"><Trash2 size={15} /></button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="mt-6 grid gap-5 xl:grid-cols-[1.5fr_1fr]">
        <section className="panel p-5">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="font-bold text-slate-900">最近扫描</h2>
              <p className="mt-1 text-xs text-slate-500">跨网站的最新诊断活动</p>
            </div>
            <CalendarClock size={18} className="text-slate-400" />
          </div>
          {!data?.recentScans.length ? (
            <EmptyState icon={<LayoutDashboard size={20} />} title="暂无扫描记录" description="添加网站并运行一次诊断，记录会出现在这里。" />
          ) : (
            <div className="mt-4 divide-y divide-slate-100">
              {data.recentScans.map((scan) => (
                <button key={scan.id} onClick={() => onOpenSite(scan.siteId)} className="flex w-full items-center justify-between gap-3 py-3 text-left transition hover:opacity-80">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-semibold text-slate-800">{scan.siteName}</div>
                    <div className="mt-1 text-xs text-slate-400">{formatDate(scan.finishedAt || scan.startedAt || scan.createdAt)} · 扫描 #{scan.id}</div>
                  </div>
                  <div className="flex shrink-0 items-center gap-3">
                    {scan.status === 'completed' && <span className="text-sm font-bold text-slate-700">{scan.score}<span className="text-[10px] font-normal text-slate-400">/100</span></span>}
                    <ScanStatusBadge status={scan.status} />
                  </div>
                </button>
              ))}
            </div>
          )}
        </section>
        <section className="panel p-5">
          <h2 className="font-bold text-slate-900">诊断范围</h2>
          <p className="mt-1 text-xs leading-5 text-slate-500">当前版本聚焦可验证的首页技术信号，不执行 JavaScript，也不保证内容在 AI 回答中的表现。</p>
          <div className="mt-4 space-y-3 text-sm text-slate-600">
            {['HTTP 可访问性与响应状态', '标题、描述、Canonical 与 H1', 'JSON-LD 结构化数据', 'robots.txt 与 sitemap.xml', '可读正文与重复问题跟踪'].map((item) => (
              <div key={item} className="flex items-center gap-2.5">
                <CheckCircle2 size={15} className="shrink-0 text-emerald-500" /> {item}
              </div>
            ))}
          </div>
          <HealthNote />
        </section>
      </div>

      <div className="mt-4 text-xs text-slate-400">
        <Trash2 size={12} className="mr-1 inline" /> 删除网站会同时移除其扫描记录、问题与状态历史，操作前会再次确认。
      </div>
    </>
  );
}
