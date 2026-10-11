"""Read-only snapshot of a Linear team into JSON files. Usage: python3 -I export.py <out_dir> <team_key>"""
import json, os, sys, time, urllib.request, urllib.error

out, team_key = sys.argv[1], sys.argv[2]
key = open(os.path.expanduser("~/.config/linear/api-key")).read().strip()

def gql(query, variables=None):
    body = json.dumps({"query": query, "variables": variables or {}}).encode()
    for attempt in range(5):
        req = urllib.request.Request("https://api.linear.app/graphql", body, {"Authorization": key, "Content-Type": "application/json"})
        try:
            res = json.load(urllib.request.urlopen(req, timeout=60))
        except urllib.error.HTTPError as e:
            detail = e.read().decode()[:500]
            if e.code in (429, 500, 502, 503) and attempt < 4:
                time.sleep(5 * (attempt + 1)); continue
            sys.exit(f"HTTP {e.code}: {detail}")
        if "errors" in res:
            sys.exit(json.dumps(res["errors"], indent=2)[:2000])
        return res["data"]

def paged(field, query, variables=None, path=None):
    nodes, after = [], None
    while True:
        data = gql(query, {**(variables or {}), "after": after})
        conn = data
        for part in (path or [field]):
            conn = conn[part]
        nodes += conn["nodes"]
        if not conn["pageInfo"]["hasNextPage"]:
            return nodes
        after = conn["pageInfo"]["endCursor"]

PAGE = "pageInfo { hasNextPage endCursor }"
USER = "{ id name displayName email }"

issue_fields = f"""
  id identifier number title description priority estimate url branchName
  createdAt updatedAt startedAt completedAt canceledAt archivedAt autoArchivedAt dueDate trashed
  state {{ id name type }}
  team {{ key }}
  project {{ id name }}
  projectMilestone {{ id name }}
  cycle {{ number name }}
  parent {{ identifier }}
  creator {USER}
  assignee {USER}
  labels(first: 50) {{ nodes {{ id name }} {PAGE} }}
  relations(first: 50) {{ nodes {{ type relatedIssue {{ identifier }} }} {PAGE} }}
  attachments(first: 50) {{ nodes {{ id title subtitle url sourceType createdAt }} {PAGE} }}
  comments(first: 50) {{ nodes {{ id body createdAt updatedAt editedAt user {USER} botActor {{ name }} parent {{ id }} }} {PAGE} }}
"""

team = gql("query($key: String!) { teams(filter: { key: { eq: $key } }) { nodes { id key name issueCount } } }", {"key": team_key})["teams"]["nodes"][0]
issues = paged("issues", f"""query($team: String!, $after: String) {{
  issues(first: 25, after: $after, includeArchived: true, orderBy: createdAt, filter: {{ team: {{ key: {{ eq: $team }} }} }}) {{
    nodes {{ {issue_fields} }} {PAGE} }} }}""", {"team": team_key})

# Fetch the rest of any nested connection that did not fit on the first page.
nested = {
    "comments": f"id body createdAt updatedAt editedAt user {USER} botActor {{ name }} parent {{ id }}",
    "attachments": "id title subtitle url sourceType createdAt",
    "relations": "type relatedIssue { identifier }",
    "labels": "id name",
}
for issue in issues:
    for field, fields in nested.items():
        conn = issue[field]
        while conn["pageInfo"]["hasNextPage"]:
            more = gql(f"query($id: String!, $after: String) {{ issue(id: $id) {{ {field}(first: 50, after: $after) {{ nodes {{ {fields} }} {PAGE} }} }} }}",
                       {"id": issue["id"], "after": conn["pageInfo"]["endCursor"]})["issue"][field]
            conn["nodes"] += more["nodes"]
            conn["pageInfo"] = more["pageInfo"]
        issue[field] = conn["nodes"]

labels = paged("issueLabels", f"query($after: String) {{ issueLabels(first: 100, after: $after, includeArchived: true) {{ nodes {{ id name color description isGroup archivedAt parent {{ id name }} team {{ key }} }} {PAGE} }} }}")
states = paged("workflowStates", f"query($after: String) {{ workflowStates(first: 100, after: $after) {{ nodes {{ id name type position team {{ key }} }} {PAGE} }} }}")
projects = paged("projects", f"query($after: String) {{ projects(first: 25, after: $after, includeArchived: true) {{ nodes {{ id name description content state startDate targetDate completedAt canceledAt archivedAt createdAt updatedAt url lead {USER} teams {{ nodes {{ key }} }} projectMilestones {{ nodes {{ id name description targetDate }} }} }} {PAGE} }} }}")
users = paged("users", f"query($after: String) {{ users(first: 100, after: $after, includeDisabled: true) {{ nodes {{ id name displayName email active }} {PAGE} }} }}")
documents = paged("documents", f"query($after: String) {{ documents(first: 25, after: $after) {{ nodes {{ id title content createdAt updatedAt url project {{ id name }} creator {USER} }} {PAGE} }} }}")

os.makedirs(out, exist_ok=True)
for name, value in {"team": team, "issues": issues, "labels": labels, "states": states, "projects": projects, "users": users, "documents": documents}.items():
    with open(os.path.join(out, f"{name}.json"), "w") as f:
        json.dump(value, f, ensure_ascii=False, indent=1)
numbers = sorted(i["number"] for i in issues)
print(f"team {team['key']} issueCount={team['issueCount']} exported={len(issues)} archived={sum(1 for i in issues if i['archivedAt'])} trashed={sum(1 for i in issues if i.get('trashed'))} min={numbers[0]} max={numbers[-1]}")
print(f"comments={sum(len(i['comments']) for i in issues)} attachments={sum(len(i['attachments']) for i in issues)} relations={sum(len(i['relations']) for i in issues)} labels={len(labels)} projects={len(projects)} documents={len(documents)} users={len(users)}")
