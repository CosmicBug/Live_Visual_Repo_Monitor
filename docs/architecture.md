# Architecture

```text
GitHub App / Webhooks
        |
        v
webhook-gateway  --repo.github.event-->  NATS JetStream
        |                                     |
        v                                     v
PostgreSQL                              repo-worker
                                             |
                                             v
                                  GitHub API hydration
                                             |
                                             v
                                  Julia modeler /compile
                                             |
                                             v
                                  algebraic model + patch
                                             |
                                             v
                                  realtime-api WebSocket
                                             |
                                             v
                                  React/Cytoscape dashboard
```

## Services

### `webhook-gateway`

Receives GitHub webhooks, validates `X-Hub-Signature-256`, normalizes payloads, stores deliveries, deduplicates by `X-GitHub-Delivery`, and publishes a durable event.

### `repo-worker`

Consumes durable GitHub events. For `push`, it fetches the repository tree at the new commit, extracts files, directories, manifests, workflows, source imports, and dependency edges. It computes a snapshot diff and asks the Julia modeler to compile a new algebraic visualization model.

### `julia-modeler`

Compiles repository snapshots and events into:

- Catlab-compatible wiring/string diagram JSON.
- AlgebraicPetri-compatible Petri net JSON.
- Hybrid visualization graph.
- Incremental visualization patch.

### `realtime-api`

Serves the latest model, recent patches, and a WebSocket stream of live patches.

### `frontend`

Displays topology, Petri, and hybrid modes using Cytoscape.js with ELK layered layout. Changed elements pulse/glow/blink depending on state.

## Model semantics

| Repository concept | Diagram concept |
|---|---|
| Repository | Outer system box |
| Directory | Nested/compound box |
| File | Leaf box |
| Import/dependency | Wire/edge |
| Workflow | Build/test process box |
| GitHub event | Dynamic event node/token |
| PR/CI/release state | Petri token over place/transition |

