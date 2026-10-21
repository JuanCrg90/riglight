import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { LogSection } from './LogSection';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe('LogSection', () => {
  it('does not request sources until opened and starts only after explicit confirmation', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, text: async () => JSON.stringify({ schemaVersion: 1, mode: 'DEMO', sources: [{ id: 'model.example-model', label: 'Example', kind: 'MODEL' }] }) });
    vi.stubGlobal('fetch', fetchMock);
    const created: Array<{ close: ReturnType<typeof vi.fn>; listeners: Map<string, EventListener> }> = [];
    vi.stubGlobal('EventSource', class { close = vi.fn(); listeners = new Map<string, EventListener>(); constructor() { created.push(this); } addEventListener(type: string, listener: EventListener) { this.listeners.set(type, listener); } });
    render(<LogSection />);
    expect(fetchMock).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: /open logs/i }));
    await screen.findByLabelText('Log source');
    expect(created).toHaveLength(0);
    fireEvent.click(screen.getByRole('button', { name: 'Start' }));
    expect(screen.getByText(/may contain credentials/i)).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: 'Start log stream' }));
    await waitFor(() => expect(created).toHaveLength(1));
    expect(created[0]!.close).not.toHaveBeenCalled();
    act(() => created[0]!.listeners.get('log')?.({ data: JSON.stringify({ schemaVersion: 1, mode: 'DEMO', sourceId: 'model.example-model', seq: 1, text: '<img src=x>', stream: 'STDOUT', truncated: false }) } as MessageEvent));
    expect(screen.getByText('<img src=x>')).toBeVisible();
    expect(document.querySelector('img')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Pause' }));
    expect(created[0]!.close).toHaveBeenCalled();
    expect(screen.getByText('<img src=x>')).toBeVisible();
  });
});
