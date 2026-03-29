import Badge from './Badge';

interface KindBadgeProps {
  kind: 'skill' | 'agent';
  size?: 'sm' | 'md';
}

export default function KindBadge({ kind, size = 'sm' }: KindBadgeProps) {
  if (kind === 'agent') {
    return <Badge variant="accent" size={size}>A</Badge>;
  }
  return <Badge variant="info" size={size}>S</Badge>;
}
