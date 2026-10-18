type Props = { values: Array<number | null>; label: string; min?: number; max?: number };

export function Sparkline({ values, label, min = 0, max = 100 }: Props) {
  const width = 300;
  const height = 72;
  const paths: string[] = [];
  let segment: string[] = [];
  const flush = () => {
    if (segment.length) paths.push(segment.join(' '));
    segment = [];
  };
  const usable = values.slice(-450);
  usable.forEach((value, index) => {
    if (value === null || !Number.isFinite(value)) { flush(); return; }
    const x = usable.length < 2 ? width : index / (usable.length - 1) * width;
    const y = height - Math.max(0, Math.min(1, (value - min) / (max - min))) * height;
    segment.push(`${segment.length ? 'L' : 'M'}${x.toFixed(2)},${y.toFixed(2)}`);
  });
  flush();
  return <svg className="sparkline" viewBox={`0 0 ${width} ${height}`} role="img" aria-label={label} preserveAspectRatio="none">
    <line x1="0" y1="71.5" x2="300" y2="71.5" className="sparkline-baseline" />
    {paths.map((path, index) => <path key={index} d={path} className="sparkline-path" />)}
  </svg>;
}
