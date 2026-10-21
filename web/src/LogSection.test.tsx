import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { LogSection } from './LogSection';

const sources = (mode = 'DEMO', entries = [{ id: 'model.example-model', label: 'Example', kind: 'MODEL' }]) => JSON.stringify({ schemaVersion: 1, mode, sources: entries });
class FakeEventSource {
  static created: FakeEventSource[] = [];
  close = vi.fn();
  listeners = new Map<string, EventListener>();
  onerror: ((event: Event) => void) | null = null;
  constructor(readonly url: string) { FakeEventSource.created.push(this); }
  addEventListener(type: string, listener: EventListener) { this.listeners.set(type, listener); }
  emit(type: string, data: unknown) { this.listeners.get(type)?.({ data: JSON.stringify(data) } as MessageEvent); }
}
const frame = (seq: number, text = `line ${seq}`, sourceId = 'model.example-model', mode = 'DEMO') => ({ schemaVersion: 1, mode, sourceId, seq, text, stream: 'STDOUT', truncated: false });
const target = (revision: number, state: string, model: string | null) => ({ schemaVersion: 1, mode: 'DEMO', sourceId: 'model.loaded', revision, state, model });
function setup(catalog = sources()) {
  const fetchMock = vi.fn().mockImplementation(async () => new Response(catalog));
  FakeEventSource.created = [];
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal('EventSource', FakeEventSource);
  return { fetchMock };
}
async function startViewer() {
  fireEvent.click(screen.getByRole('button', { name: /open logs/i }));
  await screen.findByLabelText('Log source');
  fireEvent.click(screen.getByRole('button', { name: 'Start' }));
  fireEvent.click(screen.getByRole('button', { name: 'Start log stream' }));
  await waitFor(() => expect(FakeEventSource.created).toHaveLength(1));
  return FakeEventSource.created[0]!;
}

afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.useRealTimers(); });

describe('LogSection', () => {
  it('does not request sources until opened and starts only after explicit confirmation', async () => {
    const { fetchMock } = setup();
    render(<LogSection />);
    expect(fetchMock).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: /open logs/i }));
    await screen.findByLabelText('Log source');
    expect(FakeEventSource.created).toHaveLength(0);
    fireEvent.click(screen.getByRole('button', { name: 'Start' }));
    expect(screen.getByText(/may contain credentials/i)).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: 'Start log stream' }));
    await waitFor(() => expect(FakeEventSource.created).toHaveLength(1));
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(FakeEventSource.created[0]!.close).not.toHaveBeenCalled();
    act(() => FakeEventSource.created[0]!.emit('log', frame(1, '<img src=x>')));
    expect(screen.getByText('<img src=x>')).toBeVisible();
    expect(document.querySelector('img')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Pause' }));
    expect(FakeEventSource.created[0]!.close).toHaveBeenCalled();
    expect(screen.getByText('<img src=x>')).toBeVisible();
  });

  it('prefers the loaded source, follows target changes on one EventSource, clears per-target output and preserves sequence', async () => {
    const entries = [{ id: 'model.loaded', label: 'Currently loaded model', kind: 'LOADED_MODEL' }, { id: 'model.example', label: 'Fixed', kind: 'MODEL' }];
    setup(sources('DEMO', entries)); render(<LogSection />);
    fireEvent.click(screen.getByRole('button', { name: /open logs/i }));
    await screen.findByLabelText('Log source');
    expect((screen.getByLabelText('Log source') as HTMLSelectElement).value).toBe('model.loaded');
    fireEvent.click(screen.getByRole('button', { name: 'Start' })); fireEvent.click(screen.getByRole('button', { name: 'Start log stream' }));
    await waitFor(() => expect(FakeEventSource.created).toHaveLength(1));
    const es = FakeEventSource.created[0]!;
    act(() => es.emit('log', frame(1, 'too early', 'model.loaded')));
    expect(es.close).toHaveBeenCalled();
    cleanup();
    setup(sources('DEMO', entries)); render(<LogSection />); const stream = await startViewer();
    act(() => { stream.emit('log-target', target(1, 'FOLLOWING', 'model-a')); stream.emit('log', frame(1, 'A line', 'model.loaded')); });
    expect(screen.getByText('A line')).toBeVisible();
    expect(screen.getByText(/Following model-a/)).toBeVisible();
    fireEvent.change(screen.getByLabelText('Filter lines'), { target: { value: 'A line' } });
    act(() => stream.emit('log-target', target(2, 'WAITING', null)));
    expect(screen.queryByText('A line')).not.toBeInTheDocument();
    expect(screen.queryByText('blocked')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Filter lines')).toHaveValue('');
    expect(screen.getByText(/Loaded model waiting/)).toBeVisible();
    act(() => { stream.emit('log-target', target(3, 'FOLLOWING', 'model-b')); stream.emit('log', frame(2, 'B line', 'model.loaded')); });
    expect(screen.queryByText('A line')).not.toBeInTheDocument();
    expect(screen.getByText('B line')).toBeVisible();
    expect(FakeEventSource.created).toHaveLength(1);
  });

  it('keeps same-model paused history across Resume with unique session keys', async () => {
    const entries = [{ id: 'model.loaded', label: 'Currently loaded model', kind: 'LOADED_MODEL' }];
    setup(sources('DEMO', entries)); render(<LogSection />); const first = await startViewer();
    act(() => { first.emit('log-target', target(1, 'FOLLOWING', 'model-a')); first.emit('log', frame(1, 'first session', 'model.loaded')); });
    fireEvent.click(screen.getByRole('button', { name: 'Pause' }));
    fireEvent.click(screen.getByRole('button', { name: 'Resume' })); fireEvent.click(screen.getByRole('button', { name: 'Start log stream' }));
    await waitFor(() => expect(FakeEventSource.created).toHaveLength(2));
    const second = FakeEventSource.created[1]!;
    act(() => second.emit('log-target', target(1, 'FOLLOWING', 'model-a')));
    act(() => second.emit('log', frame(1, 'second session', 'model.loaded')));
    expect(screen.getByText('first session')).toBeVisible();
    expect(screen.getByText('second session')).toBeVisible();
    expect(screen.getByRole('list', { name: 'Log output' }).querySelectorAll('li')).toHaveLength(2);
  });

  it('retains the paused model label but clears history for a different resumed target', async () => {
    const entries = [{ id: 'model.loaded', label: 'Currently loaded model', kind: 'LOADED_MODEL' }];
    const { fetchMock } = setup(sources('DEMO', entries));
    render(<LogSection />); const first = await startViewer();
    act(() => { first.emit('log-target', target(1, 'FOLLOWING', 'model-a')); first.emit('log', frame(1, 'A history', 'model.loaded')); });
    fireEvent.click(screen.getByRole('button', { name: 'Pause' }));
    expect(screen.getByText(/retained from model-a/)).toBeVisible();
    fetchMock.mockImplementation(async () => new Response(sources('DEMO', entries)));
    fireEvent.click(screen.getByRole('button', { name: 'Resume' })); fireEvent.click(screen.getByRole('button', { name: 'Start log stream' }));
    await waitFor(() => expect(FakeEventSource.created).toHaveLength(2));
    const second = FakeEventSource.created[1]!;
    act(() => second.emit('log-target', target(1, 'FOLLOWING', 'model-b')));
    expect(screen.queryByText('A history')).not.toBeInTheDocument();
    act(() => second.emit('log', frame(1, 'B history', 'model.loaded')));
    expect(screen.getByText('B history')).toBeVisible();
    expect(FakeEventSource.created).toHaveLength(2);
  });

  it('keeps a stable accessible heading while collapsed and treats DISABLED separately from empty LIVE', async () => {
    setup(sources('DISABLED', []));
    render(<LogSection />);
    expect(screen.getByRole('heading', { name: 'Logs inspector' })).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: /open logs/i }));
    expect(await screen.findByText('Log viewing is not enabled.')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Start' })).toBeDisabled();
    cleanup();
    setup(sources('LIVE', []));
    render(<LogSection />);
    fireEvent.click(screen.getByRole('button', { name: /open logs/i }));
    expect(await screen.findByText('No log sources are configured.')).toBeVisible();
  });

  it('maintains both retention bounds for a large Unicode stream under StrictMode', async () => {
    setup();
    render(<StrictMode><LogSection /></StrictMode>);
    const es = await startViewer();
    act(() => { for (let seq = 1; seq <= 205; seq++) es.emit('log', frame(seq, '😀'.repeat(1024))); });
    const items = screen.getByRole('list', { name: 'Log output' }).querySelectorAll('li');
    expect(items.length).toBeLessThanOrEqual(200);
    const retained = [...screen.getByRole('list', { name: 'Log output' }).querySelectorAll('pre')].map(node => node.textContent ?? '');
    expect(retained.reduce((sum, text) => sum + new TextEncoder().encode(text).byteLength, 0)).toBeLessThanOrEqual(256 * 1024);
    expect(retained.length).toBe(64);
  });

  it('retains paused history on Resume and gives repeated session sequences distinct keys', async () => {
    setup(); render(<LogSection />); const first = await startViewer();
    act(() => first.emit('log', frame(1, 'before pause')));
    fireEvent.click(screen.getByRole('button', { name: 'Pause' }));
    fireEvent.click(screen.getByRole('button', { name: 'Resume' }));
    fireEvent.click(screen.getByRole('button', { name: 'Start log stream' }));
    await waitFor(() => expect(FakeEventSource.created).toHaveLength(2));
    act(() => FakeEventSource.created[1]!.emit('log', frame(1, 'after resume')));
    expect(screen.getByText('before pause')).toBeVisible();
    expect(screen.getByText('after resume')).toBeVisible();
    expect(screen.getByRole('list', { name: 'Log output' }).querySelectorAll('li')).toHaveLength(2);
  });

  it('does not mix retained output when catalog mode changes across Resume', async () => {
    const { fetchMock } = setup();
    fetchMock.mockImplementationOnce(async () => new Response(sources('DEMO')))
      .mockImplementationOnce(async () => new Response(sources('DEMO')))
      .mockImplementationOnce(async () => new Response(sources('LIVE')));
    render(<LogSection />); const first = await startViewer();
    act(() => first.emit('log', frame(1, 'DEMO only')));
    fireEvent.click(screen.getByRole('button', { name: 'Pause' }));
    fireEvent.click(screen.getByRole('button', { name: 'Resume' }));
    fireEvent.click(screen.getByRole('button', { name: 'Start log stream' }));
    await screen.findByText(/source mode changed/i);
    expect(FakeEventSource.created).toHaveLength(1);
    expect(screen.getByText('DEMO only')).toBeVisible();
    expect(screen.getByText('DEMO · retained output')).toBeVisible();
  });

  it('clears and isolates source changes, requiring another explicit Start', async () => {
    const entries = [{ id: 'model.example-model', label: 'Example', kind: 'MODEL' }, { id: 'docker.example-api', label: 'API', kind: 'DOCKER' }];
    setup(sources('DEMO', entries)); render(<LogSection />); const first = await startViewer();
    act(() => first.emit('log', frame(1, 'model line')));
    fireEvent.change(screen.getByLabelText('Log source'), { target: { value: 'docker.example-api' } });
    expect(first.close).toHaveBeenCalled();
    expect(screen.queryByText('model line')).not.toBeInTheDocument();
    expect(FakeEventSource.created).toHaveLength(1);
    act(() => first.emit('log', frame(2, 'stale callback')));
    expect(screen.queryByText('stale callback')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Start' }));
    fireEvent.click(screen.getByRole('button', { name: 'Start log stream' }));
    await waitFor(() => expect(FakeEventSource.created).toHaveLength(2));
    expect(FakeEventSource.created[1]!.url).toContain('docker.example-api');
  });

  it('filters retained text locally and closes on EventSource errors', async () => {
    const { fetchMock } = setup(); render(<LogSection />); const es = await startViewer();
    act(() => { es.emit('log', frame(1, 'needle')); es.emit('log', frame(2, 'other')); });
    fireEvent.change(screen.getByLabelText('Filter lines'), { target: { value: 'needle' } });
    expect(screen.getByText('needle')).toBeVisible();
    expect(screen.queryByText('other')).not.toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    act(() => es.onerror?.(new Event('error')));
    expect(es.close).toHaveBeenCalled();
    expect(FakeEventSource.created).toHaveLength(1);
  });

  it('follows only within the log pane and uses instant scrolling for reduced motion', async () => {
    const paneScroll = vi.fn();
    const pageScroll = vi.fn();
    const oldDescriptor = Object.getOwnPropertyDescriptor(HTMLOListElement.prototype, 'scrollTo');
    Object.defineProperty(HTMLOListElement.prototype, 'scrollTo', { configurable: true, value: paneScroll });
    vi.stubGlobal('scrollTo', pageScroll);
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: true })));
    try {
      setup(); render(<LogSection />); const es = await startViewer();
      paneScroll.mockClear();
      act(() => es.emit('log', frame(1, 'tail')));
      const pane = screen.getByRole('list', { name: 'Log output' });
      expect(paneScroll).toHaveBeenCalledWith({ top: pane.scrollHeight, behavior: 'auto' });
      expect(pageScroll).not.toHaveBeenCalled();
    } finally {
      if (oldDescriptor) Object.defineProperty(HTMLOListElement.prototype, 'scrollTo', oldDescriptor);
      else delete (HTMLOListElement.prototype as Partial<HTMLOListElement>).scrollTo;
    }
  });

  it('preserves the sequence guard after Clear and closes invalid/misbinding frames', async () => {
    setup(); render(<LogSection />); const es = await startViewer();
    act(() => es.emit('log', frame(1, 'safe')));
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }));
    act(() => es.emit('log', frame(1, 'replay')));
    expect(es.close).toHaveBeenCalled();
    expect(screen.queryByText('replay')).not.toBeInTheDocument();
  });

  it('closes terminal, malformed and error streams without automatic reconnect', async () => {
    setup(); render(<LogSection />); const es = await startViewer();
    act(() => es.emit('log-status', { schemaVersion: 1, mode: 'DEMO', sourceId: 'model.example-model', state: 'ENDED', reasonCode: 'EOF' }));
    expect(es.close).toHaveBeenCalledTimes(1);
    expect(FakeEventSource.created).toHaveLength(1);
  });

  it('aborts pending catalog reads on close and unmount and ignores late responses', async () => {
    let signal: AbortSignal | undefined;
    const fetchMock = vi.fn((_url: string, options: { signal: AbortSignal }) => { signal = options.signal; return new Promise<Response>(() => {}); });
    FakeEventSource.created = [];
    vi.stubGlobal('fetch', fetchMock); vi.stubGlobal('EventSource', FakeEventSource);
    const { unmount } = render(<LogSection />);
    fireEvent.click(screen.getByRole('button', { name: /open logs/i }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    expect(signal?.aborted).toBe(true);
    fireEvent.click(screen.getByRole('button', { name: /open logs/i }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    unmount();
    expect(signal?.aborted).toBe(true);
    expect(FakeEventSource.created).toHaveLength(0);
  });

  it('aborts stalled catalog fetches at the absolute deadline', async () => {
    vi.useFakeTimers();
    const fetchMock = vi.fn((_url: string, options: { signal: AbortSignal }) => new Promise<Response>((_resolve, reject) => options.signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')), { once: true })));
    vi.stubGlobal('fetch', fetchMock); FakeEventSource.created = []; vi.stubGlobal('EventSource', FakeEventSource);
    render(<LogSection />); fireEvent.click(screen.getByRole('button', { name: /open logs/i }));
    act(() => { vi.advanceTimersByTime(2000); });
    await act(async () => { await Promise.resolve(); });
    expect(screen.getByText('Log sources are unavailable.')).toBeVisible();
    expect(FakeEventSource.created).toHaveLength(0);
  });

  it('rejects oversized catalogs before creating an EventSource', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async () => new Response(' '.repeat(16 * 1024 + 1))));
    FakeEventSource.created = []; vi.stubGlobal('EventSource', FakeEventSource);
    render(<LogSection />); fireEvent.click(screen.getByRole('button', { name: /open logs/i }));
    await screen.findByText('Log sources are unavailable.');
    expect(FakeEventSource.created).toHaveLength(0);
  });
});
