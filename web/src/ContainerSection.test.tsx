import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { ContainerSection } from './ContainerSection';
import type { ContainerOverview } from './containers';
const sampledAt = '2026-01-01T00:00:00Z';
const overview: ContainerOverview = { schemaVersion: 1, sampledAt, mode: 'DEMO', status: 'AVAILABLE', reasonCode: null, ageMs: 0, containers: [{ alias: 'demo-service', metadata: { status: 'AVAILABLE', sampledAt, ageMs: 0, state: 'RUNNING', health: 'HEALTHY', restartCount: 2 }, cpu: { status: 'AVAILABLE', sampledAt, ageMs: 0, percent: 125 }, memory: { status: 'AVAILABLE', sampledAt, ageMs: 0, usedBytes: 256, totalBytes: 1024 } }] };
afterEach(cleanup);
describe('ContainerSection', () => {
  it('announces setup and pending states without creating fake rows', () => {
    const { rerender } = render(<ContainerSection overview={null} freshness={{ overview: false, rows: {} }} />);
    expect(screen.getByRole('region', { name: 'Selected containers' })).toHaveTextContent('Not enabled');
    rerender(<ContainerSection overview={{ ...overview, status: 'NOT_READY', ageMs: null, containers: [] }} freshness={{ overview: false, rows: {} }} />);
    expect(screen.getByRole('status')).toHaveTextContent('Waiting for container data');
  });
  it('renders aliases as plain text with honest independent sensor states', () => {
    render(<ContainerSection overview={overview} freshness={{ overview: true, rows: { 'demo-service': { metadata: true, cpu: false, memory: true } } }} />);
    expect(screen.getByText('demo-service')).toBeVisible();
    expect(screen.getByText('RUNNING · HEALTHY')).toBeVisible();
    expect(screen.getByText('Stale')).toBeVisible();
    expect(screen.getByText('Cache-adjusted memory')).toBeVisible();
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });
  it('hides all rows when the overview expires', () => {
    render(<ContainerSection overview={overview} freshness={{ overview: false, rows: { 'demo-service': { metadata: true, cpu: true, memory: true } } }} />);
    expect(screen.getByText('Container overview is stale. Waiting for a new sample.')).toBeVisible();
    expect(screen.queryByText('demo-service')).not.toBeInTheDocument();
  });
});
