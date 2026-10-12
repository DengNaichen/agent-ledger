import { ReactNode } from "react";

export function PageHeader({ children, actions }: { children: ReactNode; actions?: ReactNode }) {
  return <header className="header-row"><nav className="breadcrumb" aria-label="Breadcrumb">{children}</nav>{actions}</header>;
}
