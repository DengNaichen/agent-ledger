import { useCallback } from "react";
import { apiPath, Label, LabelGroup } from "./shared";
import { useResource } from "./useResource";

export type LabelFilterValue = { mode: string; labels: number[]; groups: number[] };
export const emptyLabelFilter: LabelFilterValue = { mode: "any", labels: [], groups: [] };

export function useLabels() {
  const { data: labels, error: labelError, reload: reloadLabels } = useResource<{ labels: Label[] }>(apiPath("/labels?include_archived=true"), { labels: [] });
  const { data: groups, error: groupError, reload: reloadGroups } = useResource<{ groups: LabelGroup[] }>(apiPath("/label-groups"), { groups: [] });
  const reload = useCallback(async () => { await Promise.all([reloadLabels(), reloadGroups()]); }, [reloadLabels, reloadGroups]);
  return { labels: labels.labels, groups: groups.groups, error: labelError || groupError, reload };
}
export type Catalog = ReturnType<typeof useLabels>;
