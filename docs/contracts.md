# JSON contracts

## Normalized GitHub event

```json
{
  "delivery_id": "uuid",
  "event": "push",
  "action": "",
  "repository": {
    "id": 123,
    "owner": "owner",
    "name": "repo",
    "full_name": "owner/repo",
    "installation_id": 456
  },
  "ref": "refs/heads/main",
  "branch": "main",
  "before": "oldsha",
  "after": "newsha",
  "received_at": "2026-04-22T12:00:00Z"
}
```

## Repository snapshot

```json
{
  "repo": { "id": 123, "full_name": "owner/repo" },
  "commit": { "sha": "abc", "branch": "main", "ref": "refs/heads/main" },
  "tree": {
    "files": [ { "path": "src/main.go", "kind": "source", "language": "Go", "sha": "blob" } ],
    "directories": [ { "path": "src", "parent": "" } ]
  },
  "dependencies": [ { "source": "src/main.go", "target": "fmt", "kind": "import", "scope": "go" } ],
  "workflows": [ { "path": ".github/workflows/test.yml", "name": "test" } ]
}
```

## Visualization patch

```json
{
  "type": "VizPatch",
  "repo_id": 123,
  "patch_id": "uuid",
  "from_version": 1,
  "to_version": 2,
  "model_version": 2,
  "event": "push",
  "changes": [
    { "op": "update_node", "id": "file:src/main.go", "set": { "status": "modified" }, "animation": "glow" }
  ],
  "created_at": "2026-04-22T12:00:00Z"
}
```

## Stable visual IDs

```text
repo:<github-repo-id>
dir:<path>
file:<path>
workflow:<path>
external:<package-or-action>
place:<petri-place>
transition:<petri-transition>
token:<delivery-id>
event:<delivery-id>
pr:<number>
ci:<name>:<sha>
```
