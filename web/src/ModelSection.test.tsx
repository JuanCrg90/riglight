import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { ModelSection } from './ModelSection';
import type { ModelOverview } from './models';

const available = (models: ModelOverview['models'] = [], isTruncated = false): ModelOverview => ({
  schemaVersion: 1,
  sampledAt: '2026-01-01T00:00:00Z',
  mode: 'DEMO',
  status: 'AVAILABLE',
  reasonCode: null,
  ageMs: 0,
  isTruncated,
  models,
});

const status = (value: ModelOverview['status'], reasonCode: ModelOverview['reasonCode'], mode: ModelOverview['mode'] = 'DEMO'): ModelOverview => ({
  schemaVersion: 1,
  sampledAt: '2026-01-01T00:00:00Z',
  mode,
  status: value,
  reasonCode,
  ageMs: null,
  isTruncated: false,
  models: [],
});

afterEach(cleanup);

describe('ModelSection', () => {
  it('shows pending before the first model event, not a disabled state', () => {
    render(<ModelSection overview={null} fresh={false} />);
    expect(screen.getByRole('heading', { name: 'llama-swap' })).toBeVisible();
    expect(screen.getByText(/Waiting for model inventory/)).toBeVisible();
    expect(screen.queryByText(/disabled/i)).not.toBeInTheDocument();
  });

  it('explains how to enable disabled inventory and distinguishes not-ready', () => {
    const { rerender } = render(<ModelSection overview={status('DISABLED', 'DISABLED', 'DISABLED')} fresh={false} />);
    expect(screen.getByText('--llama-swap')).toBeVisible();
    rerender(<ModelSection overview={status('NOT_READY', null)} fresh={false} />);
    expect(screen.getByText(/not ready yet/i)).toBeVisible();
  });

  it('renders aliases as escaped plain text with explicit coarse states', () => {
    const aliases = [
      { id: 'org/聊天-😀', state: 'LOADED' },
      { id: 'example-embed', state: 'UNLOADED' },
      { id: '<img src=x onerror=alert(1)>', state: 'UNKNOWN' },
    ] as const;
    const { container } = render(<ModelSection overview={available([...aliases])} fresh />);
    expect(screen.getByText('org/聊天-😀')).toBeVisible();
    expect(screen.getByText('Loaded')).toBeVisible();
    expect(screen.getByText('Unloaded')).toBeVisible();
    expect(screen.getByText('<img src=x onerror=alert(1)>')).toBeVisible();
    expect(screen.getByText('Unknown')).toBeVisible();
    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('a[href*="org"]')).toBeNull();
    expect(screen.getByText('States are reported by llama-swap. Loaded does not imply ready.')).toBeVisible();
  });

  it('shows an explicit empty inventory', () => {
    render(<ModelSection overview={available()} fresh />);
    expect(screen.getByText(/No models are listed/)).toBeVisible();
  });

  it('hides stale aliases and explains that lifecycle states may no longer be current', () => {
    const { container } = render(<ModelSection overview={available([{ id: 'example-chat', state: 'LOADED' }])} fresh={false} />);
    expect(screen.getByText(/inventory is stale/i)).toBeVisible();
    expect(screen.queryByText('example-chat')).not.toBeInTheDocument();
    expect(container.querySelector('.models-list .model-state')).toBeNull();
  });

  it('warns when the available inventory is truncated', () => {
    render(<ModelSection overview={available([{ id: 'model-0', state: 'UNKNOWN' }], true)} fresh />);
    expect(screen.getByText(/truncated; only the first 128 aliases are shown/i)).toBeVisible();
  });

  it.each([
    ['AUTH_REQUIRED', /requires authentication/i],
    ['CONNECT_FAILED', /could not connect/i],
    ['TIMEOUT', /did not respond in time/i],
    ['QUERY_FAILED', /could not be retrieved/i],
    ['INVALID_RESPONSE', /invalid inventory response/i],
  ] as const)('shows safe guidance for unavailable reason %s', (reason, message) => {
    render(<ModelSection overview={status('UNAVAILABLE', reason)} fresh={false} />);
    expect(screen.getByText(message)).toBeVisible();
  });

  it('describes unsupported inventory without exposing upstream details', () => {
    render(<ModelSection overview={status('UNSUPPORTED', 'API_UNSUPPORTED')} fresh={false} />);
    expect(screen.getByText(/does not support model inventory/i)).toBeVisible();
    expect(screen.queryByText(/8080|9090|\/v1\/models/i)).not.toBeInTheDocument();
  });
});
