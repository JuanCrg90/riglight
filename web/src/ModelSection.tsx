import type { ModelOverview, ModelReason } from './models';

type Props = { overview: ModelOverview | null; fresh: boolean };

export function ModelSection({ overview, fresh }: Props) {
  let content;
  if (!overview) {
    content = <p className="model-message">Waiting for model inventory from the local Riglight service.</p>;
  } else if (overview.status === 'DISABLED') {
    content = <p className="model-message">Model inventory is disabled. Start Riglight with <code>--llama-swap</code> to enable read-only inventory.</p>;
  } else if (overview.status === 'NOT_READY') {
    content = <p className="model-message">Model inventory is not ready yet.</p>;
  } else if (overview.status === 'UNSUPPORTED') {
    content = <p className="model-message" data-state="UNAVAILABLE">{unsupportedMessage(overview.reasonCode)}</p>;
  } else if (overview.status === 'UNAVAILABLE') {
    content = <p className="model-message" data-state="UNAVAILABLE">{unavailableMessage(overview.reasonCode)}</p>;
  } else if (!fresh) {
    content = <p className="model-message" data-state="STALE">Model inventory is stale; lifecycle states may no longer be current.</p>;
  } else {
    content = <>
      {overview.isTruncated && <p className="model-message" data-state="UNAVAILABLE">Inventory is truncated; only the first 128 aliases are shown.</p>}
      {overview.models.length === 0 ? <p className="model-message">No models are listed in the current inventory.</p> :
        <ul className="models-list">
          {overview.models.map(model => <li key={model.id}>
            <span className="model-id">{model.id}</span>
            <span className="model-state" data-state={model.state}>{stateLabel(model.state)}</span>
          </li>)}
        </ul>}
    </>;
  }

  return <section className="models-section" aria-labelledby="models-title">
    <div className="section-heading"><h2 id="models-title">llama-swap</h2><span>Read-only inventory</span></div>
    {content}
    <p className="model-note">States are reported by llama-swap. Loaded does not imply ready.</p>
  </section>;
}

function stateLabel(state: ModelOverview['models'][number]['state']): string {
  switch (state) {
    case 'LOADED': return 'Loaded';
    case 'UNLOADED': return 'Unloaded';
    case 'UNKNOWN': return 'Unknown';
  }
}

function unsupportedMessage(reason: ModelReason | null): string {
  return reason === 'API_UNSUPPORTED'
    ? 'This llama-swap API does not support model inventory.'
    : 'Model inventory is unsupported.';
}

function unavailableMessage(reason: ModelReason | null): string {
  switch (reason) {
    case 'AUTH_REQUIRED': return 'The local llama-swap endpoint requires authentication; Riglight does not configure credentials.';
    case 'CONNECT_FAILED': return 'Could not connect to llama-swap.';
    case 'TIMEOUT': return 'llama-swap did not respond in time.';
    case 'INVALID_RESPONSE': return 'llama-swap returned an invalid inventory response.';
    case 'QUERY_FAILED': return 'Model inventory could not be retrieved.';
    default: return 'Model inventory is unavailable.';
  }
}
