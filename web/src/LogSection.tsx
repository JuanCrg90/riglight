import { useEffect, useRef, useState } from 'react';
import { parseLogCatalog, parseLogFrame, parseLogStatus, type LogFrame, type LogMode, type LogSource } from './logs';

const encoder = new TextEncoder();
const MAX_LINES = 200;
const MAX_BYTES = 256 * 1024;
type State = 'CLOSED' | 'OPENING' | 'STREAMING' | 'PAUSED' | 'ENDED' | 'UNAVAILABLE' | 'LIMIT_REACHED';

export function LogSection() {
  const [expanded, setExpanded] = useState(false);
  const [sources, setSources] = useState<LogSource[]>([]);
  const [sourceId, setSourceId] = useState('');
  const [state, setState] = useState<State>('CLOSED');
  const [lines, setLines] = useState<LogFrame[]>([]);
  const [filter, setFilter] = useState('');
  const [follow, setFollow] = useState(true);
  const [warning, setWarning] = useState(false);
  const [message, setMessage] = useState('');
  const tail = useRef<HTMLDivElement>(null);
  const eventSource = useRef<EventSource | null>(null);
  const abort = useRef<AbortController | null>(null);
  const epoch = useRef(0);
  const seq = useRef(0);
  const retainedBytes = useRef(0);

  const closeStream = () => { epoch.current++; eventSource.current?.close(); eventSource.current = null; abort.current?.abort(); abort.current = null; };
  const clearLines = () => { setLines([]); retainedBytes.current = 0; setFilter(''); };
  const loadCatalog = async (expectedEpoch: number): Promise<{ mode: LogMode; sources: LogSource[] } | null> => {
    const controller = new AbortController(); abort.current = controller;
    try {
      const response = await fetch('/api/v1/log-sources', { signal: controller.signal });
      if (!response.ok) throw new Error('Catalog unavailable');
      const raw = await response.text();
      if (expectedEpoch !== epoch.current) return null;
      let parsedMode: LogMode | null = null;
      try { const record: unknown = JSON.parse(raw); if (typeof record === 'object' && record !== null && 'mode' in record && (record.mode === 'DEMO' || record.mode === 'LIVE')) parsedMode = record.mode; } catch { /* strict parser below rejects */ }
      const catalog = parseLogCatalog(raw, parsedMode ?? undefined);
      if (!catalog || !parsedMode) throw new Error('Invalid log source catalog');
      setSources(catalog);
      if (!catalog.some(source => source.id === sourceId)) setSourceId(catalog[0]?.id ?? '');
      setMessage(catalog.length ? '' : 'No log sources are configured.');
      return { mode: parsedMode, sources: catalog };
    } catch {
      if (!controller.signal.aborted && expectedEpoch === epoch.current) { setSources([]); setMessage('Log sources are unavailable.'); }
      return null;
    } finally { if (abort.current === controller) abort.current = null; }
  };
  const openInspector = () => {
    if (expanded) return;
    setExpanded(true); const id = ++epoch.current; void loadCatalog(id);
  };
  const start = async () => {
    if (!sourceId || state === 'OPENING' || state === 'STREAMING') return;
    closeStream(); clearLines(); seq.current = 0; setWarning(false); setMessage('');
    const id = epoch.current; setState('OPENING');
    const freshCatalog = await loadCatalog(id);
    if (!freshCatalog || id !== epoch.current || !freshCatalog.sources.some(source => source.id === sourceId)) { if (id === epoch.current) setState('UNAVAILABLE'); return; }
    const streamMode = freshCatalog.mode;
    const currentSource = sourceId;
    const streamEpoch = ++epoch.current;
    const es = new EventSource(`/api/v1/logs/${encodeURIComponent(currentSource)}`);
    eventSource.current = es;
    const onLog = (event: Event) => {
      if (streamEpoch !== epoch.current || eventSource.current !== es) return;
      const frame = parseLogFrame((event as MessageEvent).data, streamMode, currentSource, seq.current);
      if (!frame) { es.close(); eventSource.current = null; setState('UNAVAILABLE'); setMessage('The log stream returned an invalid frame.'); return; }
      seq.current = frame.seq;
      setLines(previous => {
        const next = [...previous, frame]; retainedBytes.current += encoder.encode(frame.text).byteLength;
        while (next.length > MAX_LINES || retainedBytes.current > MAX_BYTES) { const removed = next.shift(); if (removed) retainedBytes.current -= encoder.encode(removed.text).byteLength; }
        return next;
      });
      setState('STREAMING');
    };
    const onStatus = (event: Event) => {
      if (streamEpoch !== epoch.current || eventSource.current !== es) return;
      const status = parseLogStatus((event as MessageEvent).data, streamMode, currentSource);
      if (!status) { es.close(); eventSource.current = null; setState('UNAVAILABLE'); setMessage('The log stream returned an invalid status.'); return; }
      if (status.state === 'OPENING' || status.state === 'STREAMING') setState(status.state);
      else { es.close(); eventSource.current = null; setState(status.state); setMessage(status.reasonCode === 'EOF' ? 'The log stream ended.' : `Log stream ${status.state.toLowerCase().replace('_', ' ')} (${status.reasonCode}).`); }
    };
    es.addEventListener('log', onLog); es.addEventListener('log-status', onStatus);
    es.onerror = () => { if (streamEpoch === epoch.current && eventSource.current === es) { es.close(); eventSource.current = null; setState('UNAVAILABLE'); setMessage('The log stream disconnected. Start again to reconnect.'); } };
  };
  const changeSource = (id: string) => { closeStream(); clearLines(); seq.current = 0; setSourceId(id); setState('CLOSED'); setWarning(false); };
  const pause = () => { closeStream(); setState('PAUSED'); };
  const close = () => { closeStream(); clearLines(); setSources([]); setSourceId(''); setState('CLOSED'); setWarning(false); setExpanded(false); setMessage(''); };

  useEffect(() => { if (follow) tail.current?.scrollIntoView?.({ behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' }); }, [lines, follow]);
  useEffect(() => () => { closeStream(); }, []);
  const filtered = lines.filter(line => !filter || line.text.toLocaleLowerCase().includes(filter.toLocaleLowerCase()));

  return <section className="logs-section" aria-labelledby="logs-title">
    {!expanded ? <button type="button" className="logs-open" onClick={openInspector} aria-label="Open logs inspector">Open logs inspector</button> : <>
      <div className="section-heading"><h2 id="logs-title">Logs inspector</h2><span>Manual, ephemeral stream</span></div>
      <p className="logs-warning">Log text may contain credentials, prompts, paths, addresses, or configuration. It is not guaranteed to be redacted. Start only when you are ready to view it.</p>
      {message && <p role="status" className="logs-message">{message}</p>}
      <div className="logs-controls">
        <label>Log source <select value={sourceId} onChange={event => changeSource(event.target.value)} disabled={!sources.length || state === 'OPENING'}>
          {sources.length ? sources.map(source => <option key={source.id} value={source.id}>{source.label} ({source.kind})</option>) : <option value="">No sources</option>}
        </select></label>
        {state === 'OPENING' ? <button type="button" disabled>Opening…</button> : state === 'STREAMING' ? <button type="button" onClick={pause}>Pause</button> : <button type="button" onClick={() => setWarning(true)} disabled={!sourceId || !sources.length}>{state === 'PAUSED' ? 'Resume' : 'Start'}</button>}
        <button type="button" onClick={() => { clearLines(); setMessage('Buffer cleared.'); }}>Clear</button>
        <button type="button" onClick={close}>Close</button>
      </div>
      {warning && <div className="logs-confirm" role="alert"><p>Logs may include sensitive information and are displayed as plain text only.</p><button type="button" onClick={() => { setWarning(false); void start(); }}>Start log stream</button><button type="button" onClick={() => setWarning(false)}>Cancel</button></div>}
      <div className="logs-tools"><label>Filter lines <input type="search" maxLength={128} value={filter} onChange={event => setFilter(event.target.value.slice(0, 128))} /></label><label><input type="checkbox" checked={follow} onChange={event => setFollow(event.target.checked)} /> Follow tail</label><span role="status">{state === 'CLOSED' ? 'Ready' : state.replace('_', ' ')}</span></div>
      <ol className="logs-lines" aria-label="Log output" aria-live="off">{filtered.map(line => <li key={line.seq}><span className="log-stream">{line.stream}</span><pre>{line.text}{line.truncated ? ' [truncated]' : ''}</pre></li>)}</ol><div ref={tail} />
    </>}
  </section>;
}
