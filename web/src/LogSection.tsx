import { useEffect, useRef, useState } from 'react';
import { appendBoundedLine, parseLogCatalog, parseLogFrame, parseLogStatus, parseLogTarget, readBoundedText, type BoundedLines, type CatalogMode, type LogMode, type LogKind, type LogSource, type LogTarget } from './logs';

const MAX_LINES = 200;
const MAX_BYTES = 256 * 1024;
type State = 'CLOSED' | 'OPENING' | 'STREAMING' | 'PAUSED' | 'ENDED' | 'UNAVAILABLE' | 'LIMIT_REACHED';

export function LogSection() {
  const [expanded, setExpanded] = useState(false);
  const [sources, setSources] = useState<LogSource[]>([]);
  const [catalogMode, setCatalogMode] = useState<CatalogMode | null>(null);
  const [bufferMode, setBufferMode] = useState<LogMode | null>(null);
  const [bufferKind, setBufferKind] = useState<LogKind | null>(null);
  const [bufferTargetLabel, setBufferTargetLabel] = useState<string | null>(null);
  const [activeTarget, setActiveTarget] = useState<LogTarget | null>(null);
  const [sourceId, setSourceId] = useState('');
  const [state, setState] = useState<State>('CLOSED');
  const [buffer, setBuffer] = useState<BoundedLines>({ lines: [], bytes: 0 });
  const [filter, setFilter] = useState('');
  const [follow, setFollow] = useState(true);
  const [warning, setWarning] = useState(false);
  const [message, setMessage] = useState('');
  const logPane = useRef<HTMLOListElement>(null);
  const eventSource = useRef<EventSource | null>(null);
  const abort = useRef<AbortController | null>(null);
  const epoch = useRef(0);
  const seq = useRef(0);
  const session = useRef(0);
  const targetRevision = useRef(0);
  const activeTargetRef = useRef<LogTarget | null>(null);
  const preserveTargetOnResume = useRef(false);

  const closeStream = () => { epoch.current++; eventSource.current?.close(); eventSource.current = null; abort.current?.abort(); abort.current = null; };
  const clearLines = () => { setBuffer({ lines: [], bytes: 0 }); setFilter(''); };
  const loadCatalog = async (expectedEpoch: number): Promise<ReturnType<typeof parseLogCatalog>> => {
    const controller = new AbortController(); abort.current = controller;
    const timeout = window.setTimeout(() => controller.abort(), 2000);
    try {
      const response = await fetch('/api/v1/log-sources', { signal: controller.signal });
      if (!response.ok) throw new Error('Catalog unavailable');
      const raw = await readBoundedText(response, 16 * 1024, controller.signal);
      if (expectedEpoch !== epoch.current || controller.signal.aborted) return null;
      const catalog = parseLogCatalog(raw);
      if (!catalog) throw new Error('Invalid log source catalog');
      setCatalogMode(catalog.mode); setSources(catalog.sources);
      if (!catalog.sources.some(source => source.id === sourceId)) setSourceId(catalog.sources.find(source => source.kind === 'LOADED_MODEL')?.id ?? catalog.sources[0]?.id ?? '');
      setMessage(catalog.mode === 'DISABLED' ? 'Log viewing is not enabled.' : catalog.sources.length ? '' : 'No log sources are configured.');
      return catalog;
    } catch {
      if (expectedEpoch === epoch.current) { setCatalogMode(null); setSources([]); setMessage('Log sources are unavailable.'); }
      return null;
    } finally { window.clearTimeout(timeout); if (abort.current === controller) abort.current = null; }
  };
  const openInspector = () => {
    if (expanded) return;
    setExpanded(true); const id = ++epoch.current; void loadCatalog(id);
  };
  const start = async (resume = false) => {
    if (!sourceId || state === 'OPENING' || state === 'STREAMING') return;
    closeStream();
    if (!resume) { clearLines(); setBufferMode(null); setBufferKind(null); setBufferTargetLabel(null); }
    setActiveTarget(null); activeTargetRef.current = null; targetRevision.current = 0;
    preserveTargetOnResume.current = resume && bufferTargetLabel !== null;
    seq.current = 0; setWarning(false); setMessage('');
    const id = epoch.current; setState('OPENING');
    const freshCatalog = await loadCatalog(id);
    if (!freshCatalog || id !== epoch.current || freshCatalog.mode === 'DISABLED' || !freshCatalog.sources.some(source => source.id === sourceId)) { if (id === epoch.current) setState('UNAVAILABLE'); return; }
    const streamMode: LogMode = freshCatalog.mode;
    const currentSource = sourceId;
    const selectedSource = freshCatalog.sources.find(source => source.id === currentSource)!;
    if (resume && bufferMode !== null && (bufferMode !== streamMode || bufferKind !== selectedSource.kind)) {
      setState('UNAVAILABLE'); setMessage('The source mode changed or source kind changed. Clear the paused buffer and start a new session.'); return;
    }
    setBufferMode(streamMode); setBufferKind(selectedSource.kind);
    const sessionId = ++session.current;
    const automatic = selectedSource.kind === 'LOADED_MODEL';
    const streamEpoch = ++epoch.current;
    const es = new EventSource(`/api/v1/logs/${encodeURIComponent(currentSource)}`);
    eventSource.current = es;
    const onLog = (event: Event) => {
      if (streamEpoch !== epoch.current || eventSource.current !== es) return;
      const frame = parseLogFrame((event as MessageEvent).data, streamMode, currentSource, seq.current);
      if (!frame || (automatic && activeTargetRef.current?.state !== 'FOLLOWING')) { es.close(); eventSource.current = null; setState('UNAVAILABLE'); setMessage(!frame ? 'The log stream returned an invalid frame.' : 'The automatic log stream returned output before a valid loaded target.'); return; }
      seq.current = frame.seq;
      setBuffer(previous => appendBoundedLine(previous, frame, `${sessionId}-${frame.seq}`, MAX_LINES, MAX_BYTES));
      if (automatic && activeTargetRef.current?.model) setBufferTargetLabel(activeTargetRef.current.model);
      setState('STREAMING');
    };
    const onStatus = (event: Event) => {
      if (streamEpoch !== epoch.current || eventSource.current !== es) return;
      const status = parseLogStatus((event as MessageEvent).data, streamMode, currentSource);
      if (!status) { es.close(); eventSource.current = null; setState('UNAVAILABLE'); setMessage('The log stream returned an invalid status.'); return; }
      if (status.state === 'OPENING' || status.state === 'STREAMING') setState(status.state);
      else { es.close(); eventSource.current = null; setState(status.state); setMessage(status.reasonCode === 'EOF' ? 'The log stream ended.' : `Log stream ${status.state.toLowerCase().replace('_', ' ')} (${status.reasonCode}).`); }
    };
    const onTarget = (event: Event) => {
      if (streamEpoch !== epoch.current || eventSource.current !== es || !automatic) return;
      const target = parseLogTarget((event as MessageEvent).data, streamMode, currentSource, targetRevision.current);
      if (!target) { es.close(); eventSource.current = null; setState('UNAVAILABLE'); setMessage('The automatic log stream returned an invalid target.'); return; }
      targetRevision.current = target.revision;
      const sameRetainedTarget = preserveTargetOnResume.current && target.state === 'FOLLOWING' && target.model === bufferTargetLabel;
      preserveTargetOnResume.current = false;
      activeTargetRef.current = target;
      setActiveTarget(target);
      setBufferTargetLabel(target.state === 'FOLLOWING' ? target.model : null);
      if (!sameRetainedTarget) clearLines();
      setMessage('');
    };
    es.addEventListener('log', onLog); es.addEventListener('log-status', onStatus);
    if (automatic) es.addEventListener('log-target', onTarget);
    es.onerror = () => { if (streamEpoch === epoch.current && eventSource.current === es) { es.close(); eventSource.current = null; setState('UNAVAILABLE'); setMessage('The log stream disconnected. Start again to reconnect.'); } };
  };
  const changeSource = (id: string) => { closeStream(); clearLines(); setBufferMode(null); setBufferKind(null); setBufferTargetLabel(null); setActiveTarget(null); activeTargetRef.current = null; targetRevision.current = 0; seq.current = 0; setSourceId(id); setState('CLOSED'); setWarning(false); };
  const pause = () => { closeStream(); setActiveTarget(null); activeTargetRef.current = null; targetRevision.current = 0; setState('PAUSED'); };
  const close = () => { closeStream(); clearLines(); setSources([]); setCatalogMode(null); setSourceId(''); setBufferMode(null); setBufferKind(null); setBufferTargetLabel(null); setActiveTarget(null); activeTargetRef.current = null; targetRevision.current = 0; setState('CLOSED'); setWarning(false); setExpanded(false); setMessage(''); };

  useEffect(() => {
    const pane = logPane.current;
    if (!follow || !pane) return;
    pane.scrollTo?.({ top: pane.scrollHeight, behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' });
  }, [buffer, follow]);
  useEffect(() => () => { closeStream(); }, []);
  const filtered = buffer.lines.filter(({ frame }) => !filter || frame.text.toLocaleLowerCase().includes(filter.toLocaleLowerCase()));
  const loadedSource = sources.find(source => source.id === sourceId)?.kind === 'LOADED_MODEL';
  const displayMode = bufferMode ? `${bufferMode} · ${bufferTargetLabel ? `retained output: ${bufferTargetLabel}` : loadedSource ? 'automatic loaded-model output' : 'retained output'}` : catalogMode ? `${catalogMode} · Manual, ephemeral stream` : 'Manual, ephemeral stream';
  const targetStatus = activeTarget ? activeTarget.state === 'FOLLOWING' ? `Following ${activeTarget.model} · loaded state is not readiness` : `Loaded model ${activeTarget.state.toLowerCase()}` : state === 'PAUSED' && bufferTargetLabel ? `Paused · retained from ${bufferTargetLabel}` : null;

  return <section className="logs-section" aria-labelledby="logs-title">
    <div className="section-heading"><h2 id="logs-title">Logs inspector</h2><span>{displayMode}</span></div>
    {!expanded ? <button type="button" className="logs-open" onClick={openInspector} aria-label="Open logs inspector">Open logs inspector</button> : <>
      <p className="logs-warning">Log text may contain credentials, prompts, paths, addresses, or configuration. It is not guaranteed to be redacted. Start only when you are ready to view it.</p>
      {message && <p role="status" className="logs-message">{message}</p>}
      <div className="logs-controls">
        <label>Log source <select value={sourceId} onChange={event => changeSource(event.target.value)} disabled={!sources.length || state === 'OPENING'}>
          {sources.length ? sources.map(source => <option key={source.id} value={source.id}>{source.label} ({source.kind})</option>) : <option value="">No sources</option>}
        </select></label>
        {state === 'OPENING' ? <button type="button" disabled>Opening…</button> : state === 'STREAMING' ? <button type="button" onClick={pause}>Pause</button> : <button type="button" onClick={() => setWarning(true)} disabled={!sourceId || !sources.length || catalogMode === 'DISABLED'}>{state === 'PAUSED' ? 'Resume' : 'Start'}</button>}
        <button type="button" onClick={() => { clearLines(); setMessage('Buffer cleared.'); }}>Clear</button>
        <button type="button" onClick={close}>Close</button>
      </div>
      {warning && <div className="logs-confirm" role="alert"><p>Logs may include sensitive information and are displayed as plain text only.</p><button type="button" onClick={() => { const resuming = state === 'PAUSED'; setWarning(false); void start(resuming); }}>Start log stream</button><button type="button" onClick={() => setWarning(false)}>Cancel</button></div>}
      <div className="logs-tools"><label>Filter lines <input type="search" maxLength={128} value={filter} onChange={event => setFilter(event.target.value.slice(0, 128))} /></label><label><input type="checkbox" checked={follow} onChange={event => setFollow(event.target.checked)} /> Follow tail</label><span role="status">{state === 'CLOSED' ? 'Ready' : state.replace('_', ' ')}</span>{targetStatus && <span role="status" className="logs-target-status">{targetStatus}</span>}</div>
      <ol ref={logPane} className="logs-lines" aria-label="Log output" aria-live="off">{filtered.map(({ frame: line, key }) => <li key={key}><span className="log-stream">{line.stream}</span><pre>{line.text}{line.truncated ? ' [truncated]' : ''}</pre></li>)}</ol>
    </>}
  </section>;
}
