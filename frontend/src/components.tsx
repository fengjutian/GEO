import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { Activity, AlertTriangle, CheckCircle2, Loader2, X } from 'lucide-react';
import type { Finding, Scan, Site, SiteInput } from './types';

export const severityMeta: Record<string, { label: string; badge: string; dot: string; bar: string; text: string }> = {
  critical: { label: '严重', badge: 'bg-rose-50 text-rose-700 border-rose-100', dot: 'bg-rose-500', bar: 'bg-rose-500', text: 'text-rose-600' },
  high: { label: '高', badge: 'bg-orange-50 text-orange-700 border-orange-100', dot: 'bg-orange-500', bar: 'bg-orange-500', text: 'text-orange-600' },
  medium: { label: '中', badge: 'bg-amber-50 text-amber-700 border-amber-100', dot: 'bg-amber-400', bar: 'bg-amber-400', text: 'text-amber-600' },
  low: { label: '低', badge: 'bg-slate-100 text-slate-600 border-slate-200', dot: 'bg-slate-400', bar: 'bg-slate-400', text: 'text-slate-500' },
};

export const issueStatusMeta: Record<string, { label: string; badge: string }> = {
  open: { label: '待处理', badge: 'bg-rose-50 text-rose-700' },
  in_progress: { label: '处理中', badge: 'bg-sky-50 text-sky-700' },
  resolved: { label: '已修复', badge: 'bg-emerald-50 text-emerald-700' },
  ignored: { label: '已忽略', badge: 'bg-slate-100 text-slate-500' },
};

export const issueStatusOptions = ['open', 'in_progress', 'resolved', 'ignored'];

export function scanStatusMeta(status: string): { label: string; badge: string; dot: string } {
  switch (status) {
    case 'queued':
      return { label: '排队中', badge: 'bg-amber-50 text-amber-700', dot: 'bg-amber-400' };
    case 'completed':
      return { label: '已完成', badge: 'bg-emerald-50 text-emerald-700', dot: 'bg-emerald-500' };
    case 'running':
      return { label: '扫描中', badge: 'bg-sky-50 text-sky-700', dot: 'bg-sky-500 animate-pulse' };
    case 'failed':
      return { label: '失败', badge: 'bg-rose-50 text-rose-700', dot: 'bg-rose-500' };
    case 'cancelled':
      return { label: '已取消', badge: 'bg-slate-100 text-slate-500', dot: 'bg-slate-400' };
    default:
      return { label: status || '未知', badge: 'bg-slate-100 text-slate-600', dot: 'bg-slate-400' };
  }
}

export function formatDate(value?: string | null): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getFullYear() < 1970) return '—';
  return date.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
}

export function formatShortDate(value?: string | null): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getFullYear() < 1970) return '—';
  return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' });
}

function scoreTone(score: number): { text: string; ring: string } {
  if (score >= 80) return { text: 'text-emerald-600', ring: '#10b981' };
  if (score >= 60) return { text: 'text-amber-600', ring: '#f59e0b' };
  return { text: 'text-rose-600', ring: '#f43f5e' };
}

export function ScoreRing({ score, size = 68, muted = false }: { score: number; size?: number; muted?: boolean }) {
  const tone = scoreTone(score);
  return (
    <div
      className="relative grid shrink-0 place-items-center rounded-full"
      style={{ width: size, height: size, background: muted ? '#e2e8f0' : `conic-gradient(${tone.ring} ${Math.max(0, Math.min(100, score)) * 3.6}deg, #e2e8f0 0deg)` }}
      aria-label={`评分 ${score}`}
    >
      <div className="grid place-items-center rounded-full bg-white" style={{ width: size - 10, height: size - 10 }}>
        <span className={`font-bold leading-none ${muted ? 'text-slate-400' : tone.text}`} style={{ fontSize: size * 0.3 }}>
          {score}
        </span>
        <span className="mt-0.5 text-[9px] font-semibold text-slate-400">/ 100</span>
      </div>
    </div>
  );
}

export function SeverityBadge({ severity }: { severity: string }) {
  const meta = severityMeta[severity] ?? severityMeta.low;
  return <span className={`pill border ${meta.badge}`}>{meta.label}</span>;
}

export function IssueStatusBadge({ status }: { status: string }) {
  const meta = issueStatusMeta[status] ?? issueStatusMeta.open;
  return <span className={`pill ${meta.badge}`}>{meta.label}</span>;
}

export function ScanStatusBadge({ status }: { status: string }) {
  const meta = scanStatusMeta(status);
  return (
    <span className={`pill gap-1.5 ${meta.badge}`}>
      <span className={`h-1.5 w-1.5 rounded-full ${meta.dot}`} />
      {meta.label}
    </span>
  );
}

export function StatCard({
  icon,
  label,
  value,
  hint,
  color = 'indigo',
}: {
  icon: ReactNode;
  label: string;
  value: ReactNode;
  hint: string;
  color?: 'indigo' | 'blue' | 'amber' | 'green' | 'red';
}) {
  const colors: Record<string, string> = {
    indigo: 'bg-indigo-50 text-indigo-600',
    blue: 'bg-sky-50 text-sky-600',
    amber: 'bg-amber-50 text-amber-600',
    green: 'bg-emerald-50 text-emerald-600',
    red: 'bg-rose-50 text-rose-600',
  };
  return (
    <div className="panel p-5">
      <div className="flex items-center justify-between">
        <span className="text-sm text-slate-500">{label}</span>
        <span className={`flex h-9 w-9 items-center justify-center rounded-xl ${colors[color]}`}>{icon}</span>
      </div>
      <div className="mt-4 text-2xl font-bold tracking-tight text-slate-900">{value}</div>
      <div className="mt-1 text-xs text-slate-400">{hint}</div>
    </div>
  );
}

export function EmptyState({ icon, title, description, action }: { icon?: ReactNode; title: string; description: string; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center px-5 py-14 text-center">
      <div className="mb-3 rounded-xl bg-slate-100 p-3 text-slate-400">{icon ?? <Activity size={20} />}</div>
      <div className="text-sm font-semibold text-slate-700">{title}</div>
      <div className="mt-1 max-w-sm text-xs leading-5 text-slate-400">{description}</div>
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}

export function SeverityDistribution({ counts, total }: { counts: Record<string, number>; total: number }) {
  if (total === 0) {
    return <div className="rounded-xl bg-emerald-50 p-4 text-sm text-emerald-700">本次扫描未发现规则命中的问题。</div>;
  }
  return (
    <div className="space-y-3">
      {(['critical', 'high', 'medium', 'low'] as const).map((severity) => {
        const meta = severityMeta[severity];
        const count = counts[severity] ?? 0;
        return (
          <div key={severity} className="flex items-center gap-3">
            <span className={`w-8 text-xs font-semibold ${meta.text}`}>{meta.label}</span>
            <div className="h-2 flex-1 overflow-hidden rounded-full bg-slate-100">
              <div className={`h-full rounded-full ${meta.bar}`} style={{ width: `${(count / Math.max(total, 1)) * 100}%` }} />
            </div>
            <span className="w-6 text-right text-xs font-semibold text-slate-500">{count}</span>
          </div>
        );
      })}
    </div>
  );
}

export function Modal({
  title,
  subtitle,
  children,
  onClose,
  wide = false,
}: {
  title: string;
  subtitle?: string;
  children: ReactNode;
  onClose: () => void;
  wide?: boolean;
}) {
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.style.overflow = previous;
    };
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/35 p-4 backdrop-blur-[2px]"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className={`max-h-[92vh] w-full ${wide ? 'max-w-4xl' : 'max-w-lg'} overflow-y-auto rounded-2xl bg-white shadow-2xl`}>
        <div className="sticky top-0 z-10 flex items-start justify-between gap-4 border-b border-slate-100 bg-white/95 px-6 py-5 backdrop-blur">
          <div>
            <h2 className="text-xl font-bold text-slate-900">{title}</h2>
            {subtitle && <p className="mt-1 text-sm text-slate-500">{subtitle}</p>}
          </div>
          <button type="button" onClick={onClose} className="rounded-lg p-2 text-slate-400 transition hover:bg-slate-100 hover:text-slate-600" aria-label="关闭">
            <X size={18} />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

export function SiteFormModal({
  initial,
  submitting,
  onClose,
  onSubmit,
}: {
  initial?: Site;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (input: SiteInput) => Promise<void>;
}) {
  const [form, setForm] = useState<SiteInput>({
    name: initial?.name ?? '',
    url: initial?.url ?? '',
    description: initial?.description ?? '',
  });
  const [error, setError] = useState('');

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError('');
    if (!form.name.trim() || !form.url.trim()) {
      setError('请填写网站名称和地址。');
      return;
    }
    try {
      await onSubmit({ name: form.name.trim(), url: form.url.trim(), description: form.description.trim() });
    } catch (err) {
      setError(err instanceof Error ? err.message : '保存失败');
    }
  };

  return (
    <Modal title={initial ? '编辑网站' : '添加网站'} subtitle={initial ? '修改网站信息，不会删除已有扫描历史。' : '添加可公开访问的网站作为诊断目标。'} onClose={onClose}>
      <form onSubmit={submit} className="space-y-4 p-6">
        <div>
          <label className="label" htmlFor="site-name">网站名称 *</label>
          <input id="site-name" required className="field" placeholder="例如：公司官网" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} />
        </div>
        <div>
          <label className="label" htmlFor="site-url">网站地址 *</label>
          <input id="site-url" required className="field" placeholder="https://example.com" value={form.url} onChange={(event) => setForm({ ...form, url: event.target.value })} />
          <p className="mt-1.5 text-xs text-slate-400">仅支持 HTTP/HTTPS 的 80、443 端口；本机、内网与云元数据地址会被拦截。</p>
        </div>
        <div>
          <label className="label" htmlFor="site-description">备注（可选）</label>
          <textarea id="site-description" className="field min-h-20 resize-y" placeholder="网站用途、业务说明…" value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} />
        </div>
        {error && <div className="rounded-xl border border-rose-100 bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</div>}
        <div className="flex justify-end gap-2 border-t border-slate-100 pt-4">
          <button type="button" onClick={onClose} className="btn btn-secondary">取消</button>
          <button type="submit" disabled={submitting} className="btn btn-primary">
            {submitting && <Loader2 size={15} className="animate-spin" />}
            {submitting ? '提交中…' : initial ? '保存修改' : '创建网站'}
          </button>
        </div>
      </form>
    </Modal>
  );
}

export function ScanFailureNote({ scan }: { scan: Scan }) {
  if (scan.status !== 'failed') return null;
  return (
    <div className="flex items-start gap-3 rounded-xl border border-rose-100 bg-rose-50 p-4 text-sm text-rose-800">
      <AlertTriangle size={18} className="mt-0.5 shrink-0" />
      <div>
        <div className="font-semibold">本次扫描未能完成</div>
        <p className="mt-1 leading-6">{scan.errorMessage || '目标页面无法访问，请检查地址、证书与访问限制。'}</p>
        {scan.errorCode && <div className="mt-1 text-xs text-rose-500">错误代码：{scan.errorCode}</div>}
      </div>
    </div>
  );
}

export function IssueStatusSelect({
  finding,
  disabled,
  onChange,
}: {
  finding: Finding;
  disabled: boolean;
  onChange: (status: string) => void;
}) {
  return (
    <select
      className="rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-xs font-semibold text-slate-600 outline-none transition focus:border-indigo-400"
      value={finding.status}
      disabled={disabled}
      onChange={(event) => onChange(event.target.value)}
      aria-label="更新问题状态"
    >
      {issueStatusOptions.map((status) => (
        <option key={status} value={status}>{issueStatusMeta[status].label}</option>
      ))}
    </select>
  );
}

export function HealthNote() {
  return (
    <div className="mt-5 flex items-start gap-2 text-xs leading-5 text-slate-400">
      <CheckCircle2 size={15} className="mt-0.5 shrink-0" />
      诊断结果来自确定性规则检查，主要帮助发现技术与内容可读性问题，不保证特定 AI 平台收录或引用。
    </div>
  );
}
