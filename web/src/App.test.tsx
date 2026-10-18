import { render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';

afterEach(() => vi.unstubAllGlobals());

describe('dashboard shell', () => {
  it('describes the pending state without inventing telemetry', () => {
    vi.stubGlobal('EventSource', class { addEventListener() {} removeEventListener() {} close() {} });
    render(<App />);
    expect(screen.getByRole('heading', { name: 'System overview' })).toBeVisible();
    expect(screen.getByRole('status')).toHaveTextContent('Waiting for the first sample');
    expect(screen.getByText('NO SAMPLE')).toBeVisible();
    expect(screen.queryByText(/%/)).not.toBeInTheDocument();
  });
});
