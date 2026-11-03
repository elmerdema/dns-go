import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Activity, Database, Globe2, LogOut, Plus, RefreshCw, Search, Server, ShieldCheck, Trash2 } from "lucide-react";

type RecordType = "A" | "AAAA" | "CNAME" | "TXT" | "MX" | "NS" | "SOA";
type DnsRecord = { name: string; type: RecordType; ttl: number; value: string };
type ApiError = { error?: string };

const recordTypes: RecordType[] = ["A", "AAAA", "CNAME", "TXT", "MX", "NS", "SOA"];
const tokenKey = "dns-go-admin-token";

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className="grid gap-2 text-[11px] font-extrabold tracking-[.03em] text-[#687571]">{label}{children}</label>;
}

function Stat({ icon, tone, label, value, detail }: { icon: React.ReactNode; tone: "blue" | "purple" | "green"; label: string; value: React.ReactNode; detail: string }) {
  const tones = { blue: "bg-[#e8f1f1] text-[#4b7f91]", purple: "bg-[#f0eaf3] text-[#8a6c9e]", green: "bg-[#e7f2e9] text-[#4c9870]" };
  return <div className="flex items-center gap-3.5 rounded-sm border border-[#e1e6dd] bg-white p-5"><div className={`grid h-[38px] w-[38px] place-items-center rounded-sm ${tones[tone]}`}>{icon}</div><div><span className="block text-[11px] text-[#7e8d86]">{label}</span><strong className="mt-1 block text-[25px] font-extrabold tracking-[-.05em] text-[#24312e]">{value}</strong><small className="mt-0.5 block text-[10px] text-[#a6b0a7]">{detail}</small></div></div>;
}

async function request<T>(path: string, token: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: {
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      Authorization: `Bearer ${token}`,
      ...init?.headers,
    },
  });
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as ApiError;
    throw new Error(body.error ?? `Request failed with status ${response.status}`);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

function App() {
  const [token, setToken] = useState(() => sessionStorage.getItem(tokenKey) ?? "");
  const [draftToken, setDraftToken] = useState("");
  const [records, setRecords] = useState<DnsRecord[]>([]);
  const [nameFilter, setNameFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState<"" | RecordType>("");
  const [form, setForm] = useState({ name: "", type: "A" as RecordType, ttl: "300", value: "" });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [loginError, setLoginError] = useState("");
  const [ready, setReady] = useState<boolean | null>(null);

  const query = useMemo(() => {
    const params = new URLSearchParams();
    if (nameFilter.trim()) params.set("name", nameFilter.trim());
    if (typeFilter) params.set("type", typeFilter);
    return params.toString();
  }, [nameFilter, typeFilter]);

  const loadRecords = useCallback(async () => {
    if (!token) return;
    setBusy(true);
    setError("");
    try {
      const response = await request<{ records: DnsRecord[] }>(`/api/v1/records${query ? `?${query}` : ""}`, token);
      setRecords(response.records);
      setReady(true);
    } catch (caught) {
      setReady(false);
      setError(caught instanceof Error ? caught.message : "Could not load records");
    } finally {
      setBusy(false);
    }
  }, [query, token]);

  useEffect(() => { void loadRecords(); }, [loadRecords]);

  async function saveToken(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const candidate = draftToken.trim();
    if (!candidate) return;
    setBusy(true);
    setLoginError("");
    try {
      await request<{ records: DnsRecord[] }>("/api/v1/records", candidate);
      sessionStorage.setItem(tokenKey, candidate);
      setToken(candidate);
      setNotice("Token saved for this browser session.");
    } catch (caught) {
      setLoginError(caught instanceof Error ? caught.message : "Authentication failed");
    } finally {
      setBusy(false);
    }
  }

  function clearToken() {
    sessionStorage.removeItem(tokenKey);
    setToken("");
    setRecords([]);
    setReady(null);
  }

  async function saveRecord(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    setNotice("");
    const params = new URLSearchParams({ name: form.name, type: form.type });
    try {
      await request<DnsRecord>(`/api/v1/records?${params}`, token, {
        method: "PUT",
        body: JSON.stringify({ ttl: Number(form.ttl), value: form.value }),
      });
      setNotice(`${form.type} record saved.`);
      setForm((current) => ({ ...current, name: "", value: "" }));
      await loadRecords();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Could not save record");
      setBusy(false);
    }
  }

  async function deleteRecord(record: DnsRecord) {
    if (!window.confirm(`Delete ${record.type} ${record.name}?`)) return;
    setBusy(true);
    setError("");
    setNotice("");
    const params = new URLSearchParams({ name: record.name, type: record.type });
    try {
      await request<void>(`/api/v1/records?${params}`, token, { method: "DELETE" });
      setNotice(`${record.type} record deleted.`);
      await loadRecords();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Could not delete record");
      setBusy(false);
    }
  }

  if (!token) {
    return (
      <main className="relative grid min-h-screen place-items-center overflow-hidden bg-[#10191c] p-6 text-[#eef0e9] before:absolute before:inset-0 before:bg-[linear-gradient(rgba(220,224,212,.08)_1px,transparent_1px),linear-gradient(90deg,rgba(220,224,212,.08)_1px,transparent_1px)] before:bg-[size:46px_46px] before:opacity-25">
        <div className="absolute -right-52 -top-60 h-[580px] w-[580px] rounded-full border border-[#e195623d]" /><div className="absolute -bottom-44 -left-36 h-80 w-80 rounded-full border border-[#e195623d]" />
        <section className="relative w-full max-w-[470px] rounded-[5px] border border-[#e1e9d922] bg-[#192628eb] p-7 shadow-[0_30px_90px_rgba(0,0,0,.32)] sm:p-[42px]">
          <div className="flex items-center gap-3 text-sm text-[#f0f3e9]"> <div className="grid h-10 w-10 place-items-center rounded-full bg-[#e19a63] text-[#172123]"><Globe2 size={22} /></div><span>dns-go <strong className="font-medium text-[#bdc9bd]">control</strong></span></div>
          <p className="mt-10 font-mono text-[10px] tracking-[.08em] text-[#e19a63]">AUTHENTICATED WORKSPACE</p>
          <h1 className="mt-11 text-[clamp(32px,7vw,48px)] font-extrabold leading-[1.03] tracking-[-.055em]">Keep every<br /><em className="font-serif font-normal text-[#e19a63]">signal</em> accounted for.</h1>
          <p className="mt-3 max-w-[350px] text-[13px] leading-[1.7] text-[#aeb8ae]">Enter the admin token to manage the authoritative records serving your network.</p>
          {loginError && <p role="alert" className="mt-5 rounded-sm border border-[#8e4d4d] bg-[#422b2b] p-3 text-[13px] text-[#f1b1a3]">{loginError}</p>}
          <form onSubmit={saveToken} className="mt-6 grid gap-4">
            <label className="grid gap-2 text-[11px] font-extrabold tracking-[.03em] text-[#c4cdc0]">Admin token<input className="w-full rounded-[3px] border border-[#455354] bg-[#121e20] p-3 text-[#eef0e9] outline-none focus:border-[#cf7139] focus:ring-4 focus:ring-[#cf713922]" aria-label="Admin token" autoFocus type="password" value={draftToken} onChange={(event) => setDraftToken(event.target.value)} placeholder="Paste your bearer token" /></label>
            <button className="inline-flex items-center justify-center gap-2 rounded-[3px] bg-[#e19a63] px-4 py-3 font-extrabold text-[#172123] shadow-[0_8px_18px_rgba(181,91,40,.18)] hover:bg-[#efa875] disabled:cursor-not-allowed disabled:opacity-60" type="submit" disabled={busy}>{busy ? "Checking token…" : <>Enter control plane <span aria-hidden="true">↗</span></>}</button>
          </form>
          <p className="mt-7 flex items-center gap-2 text-[11px] text-[#7f8e87]"><ShieldCheck size={14} /> Your token stays in this browser session.</p>
        </section>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-[#f5f6f2] font-sans text-[#202a35]">
      <header className="flex h-[72px] items-center justify-between border-b border-[#2c3939] bg-[#172326] px-5 sm:px-[4.5%]">
        <div className="flex items-center gap-3 text-[15px] text-[#f0f2e9]"><div className="grid h-8 w-8 place-items-center rounded-full bg-[#e19a63] text-[#172123]"><Globe2 size={20} /></div><span>dns-go <strong className="font-medium text-[#aeb9af] max-sm:hidden">control plane</strong></span></div>
        <div className="flex items-center gap-3"><span className={`flex items-center gap-3 rounded-sm px-3 py-2 font-mono text-[10px] ${ready ? "bg-[#1d3831] text-[#a7d6b1]" : ready === false ? "bg-[#422b2b] text-[#f1b1a3]" : "bg-[#202f30] text-[#c3cdc2]"}`}><span className={`h-1.5 w-1.5 rounded-full ${ready ? "bg-[#8bc69a]" : ready === false ? "bg-[#df8375]" : "bg-[#87918a]"}`} />{ready ? "Redis ready" : ready === false ? "Unavailable" : "Checking"}</span><button className="grid place-items-center rounded-sm p-2 text-[#a8b2aa] hover:bg-[#253536] hover:text-white" onClick={clearToken} title="Sign out" aria-label="Sign out"><LogOut size={17} /></button></div>
      </header>
      <div className="mx-auto w-[91%] max-w-[1400px] py-9 sm:py-[54px]">
        <div className="mb-7 flex flex-col items-start justify-between gap-5 sm:flex-row sm:items-end"><div><p className="font-mono text-[10px] tracking-[.08em] text-[#708078]">AUTHORITATIVE DNS / ZONE VIEW</p><h1 className="my-2 flex items-center gap-4 text-[34px] font-extrabold tracking-[-.06em] sm:text-[38px]">Records <span className="h-1 w-10 bg-[#d6753d]" /></h1><p className="text-[13px] text-[#7d8988]">Manage the answers served by your dns-go instance.</p></div><button className="inline-flex items-center justify-center gap-2 rounded-sm border border-[#ccd5ca] bg-transparent px-4 py-3 font-extrabold text-[#3d4c49] hover:bg-white" onClick={() => void loadRecords()} disabled={busy}><RefreshCw size={16} className={busy ? "animate-spin" : ""} /> Sync records</button></div>
        {error && <div className="mb-5 rounded-sm border border-[#ffd6d9] bg-[#fff0f1] p-3 text-[13px] text-[#a53c48]">{error}</div>}
        {notice && <div className="mb-5 rounded-sm border border-[#c7e9cf] bg-[#ebf7ed] p-3 text-[13px] text-[#28774f]">{notice}</div>}
        <section className="mb-6 grid gap-3.5 md:grid-cols-3"><Stat icon={<Database size={18} />} tone="blue" label="Total records" value={records.length} detail="in active zone" /><Stat icon={<Server size={18} />} tone="purple" label="Record types" value={new Set(records.map((record) => record.type)).size} detail="distinct answers" /><Stat icon={<Activity size={18} />} tone="green" label="API status" value={ready ? "Healthy" : "—"} detail={ready ? "serving requests" : "awaiting check"} /></section>
        <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1.75fr)_minmax(290px,.75fr)]">
          <section className="overflow-hidden rounded-sm border border-[#e1e6dd] bg-white"><div className="flex flex-col items-start justify-between gap-4 border-b border-[#edf0ea] p-5 sm:flex-row sm:items-center"><div><h2 className="text-[15px] font-bold text-[#273530]">DNS records</h2><p className="mt-1 text-[13px] text-[#7d8988]">Current answers in Redis</p></div><div className="flex w-full gap-2 sm:w-auto"><div className="relative flex-1 sm:flex-none"><Search className="absolute left-2.5 top-2.5 text-[#93a09a]" size={14} /><input aria-label="Filter by name" className="w-full rounded-sm border border-[#dfe4dc] py-2 pl-8 pr-2 text-xs outline-none focus:border-[#cf7139] sm:w-[170px]" value={nameFilter} onChange={(event) => setNameFilter(event.target.value)} placeholder="Search names" /></div><select aria-label="Filter by record type" className="w-[105px] rounded-sm border border-[#dfe4dc] p-2 text-xs outline-none focus:border-[#cf7139]" value={typeFilter} onChange={(event) => setTypeFilter(event.target.value as "" | RecordType)}><option value="">All types</option>{recordTypes.map((type) => <option key={type}>{type}</option>)}</select></div></div><div className="overflow-x-auto"><table className="w-full border-collapse text-xs"><thead><tr className="bg-[#fafbf8] text-left font-mono text-[9px] uppercase tracking-[.06em] text-[#87928c]"><th className="p-3.5 px-[18px] font-medium">Name</th><th className="p-3.5 px-[18px] font-medium">Type</th><th className="p-3.5 px-[18px] font-medium">TTL</th><th className="p-3.5 px-[18px] font-medium">Value</th><th /></tr></thead><tbody>{records.length === 0 ? <tr><td colSpan={5} className="p-10 text-center text-[#98a39c]">No records found.</td></tr> : records.map((record) => <tr className="border-b border-[#f0f2ed] last:border-0 hover:bg-[#fcfdf9]" key={`${record.name}-${record.type}`}><td className="whitespace-nowrap p-3.5 px-[18px] font-mono text-[11px] text-[#2a3935]">{record.name}</td><td className="whitespace-nowrap p-3.5 px-[18px]"><span className="rounded-sm bg-[#f9ece2] px-2 py-1 font-mono text-[10px] text-[#b25d2e]">{record.type}</span></td><td className="whitespace-nowrap p-3.5 px-[18px] text-[#687871]">{record.ttl}s</td><td className="max-w-[240px] overflow-hidden text-ellipsis whitespace-nowrap p-3.5 px-[18px] text-[#687871]">{record.value}</td><td className="p-3.5 px-[18px]"><button className="text-[#aeb8b0] hover:text-[#c45d4d]" title="Delete record" aria-label={`Delete ${record.type} ${record.name}`} onClick={() => void deleteRecord(record)} disabled={busy}><Trash2 size={16} /></button></td></tr>)}</tbody></table></div></section>
          <section className="overflow-hidden rounded-sm border border-[#e1e6dd] bg-white pb-5 lg:order-last"><div className="flex items-center justify-between border-b border-[#edf0ea] p-5"><div><h2 className="text-[15px] font-bold text-[#273530]">Add or update</h2><p className="mt-1 text-[13px] text-[#7d8988]">Changes apply immediately.</p></div><div className="grid h-7 w-7 place-items-center rounded-sm bg-[#f9ece2] text-[#c16836]"><Plus size={17} /></div></div><form onSubmit={saveRecord} className="grid gap-4 p-5"><Field label="Name"><input required value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} placeholder="example.com." /></Field><div className="grid grid-cols-2 gap-3"><Field label="Type"><select value={form.type} onChange={(event) => setForm({ ...form, type: event.target.value as RecordType })}>{recordTypes.map((type) => <option key={type}>{type}</option>)}</select></Field><Field label="TTL"><input required min="1" type="number" value={form.ttl} onChange={(event) => setForm({ ...form, ttl: event.target.value })} /></Field></div><Field label="Value"><textarea required rows={4} value={form.value} onChange={(event) => setForm({ ...form, value: event.target.value })} placeholder="192.0.2.10" /></Field><button className="inline-flex items-center justify-center gap-2 rounded-sm bg-[#e19a63] px-4 py-3 font-extrabold text-[#172123] hover:bg-[#efa875]" type="submit" disabled={busy}><Plus size={16} /> Save record</button></form><p className="px-5 text-[11px] leading-relaxed text-[#9aa59e]">Use the format expected by the selected DNS type. Names are normalized automatically.</p></section>
        </div>
      </div>
    </main>
  );
}

export default App;
