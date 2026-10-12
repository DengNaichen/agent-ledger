import { LabelManager } from "./LabelManager";
import { PageHeader } from "./PageHeader";
import { NavLink, pagePath } from "./shared";
import { useLabels } from "./useLabels";

export default function LabelsPage() {
  const catalog = useLabels();
  return <><PageHeader actions={<NavLink to={pagePath("/")}>Back to issues</NavLink>}><span>Labels</span></PageHeader><LabelManager catalog={catalog} onMerged={() => {}} /></>;
}
