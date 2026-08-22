import { LoaderCircle, type LucideIcon } from 'lucide-react';
import type { ButtonHTMLAttributes, ReactNode } from 'react';

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
  size?: 'sm' | 'md' | 'lg';
  loading?: boolean;
  icon?: LucideIcon;
  children?: ReactNode;
};

export function ActionButton({ variant = 'primary', size = 'md', loading = false, icon: Icon, children, className = '', disabled, ...props }: Props) {
  return <button className={`action-button ${variant} ${size} ${className}`} disabled={disabled || loading} {...props}>
    {loading ? <LoaderCircle className="spin" aria-hidden="true" /> : Icon ? <Icon aria-hidden="true" /> : null}
    {children && <span>{children}</span>}
  </button>;
}
