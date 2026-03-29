# Thimble Kubernetes Deployment

Kustomize manifests for running thimble as a sidecar container alongside your application.

## Quick Start

Deploy the base manifests:

```bash
kubectl apply -k deploy/kustomize/base/
```

## Overlays

Use the dev overlay (reduced resources, latest tag):

```bash
kubectl apply -k deploy/kustomize/overlays/dev/
```

Use the prod overlay (higher resources, pinned image tag):

```bash
kubectl apply -k deploy/kustomize/overlays/prod/
```

## Adding Plugins

Add plugin definitions to the ConfigMap in `deploy/kustomize/base/configmap.yaml`. Each plugin is a JSON file mounted into `/data/plugins/`:

```yaml
data:
  my-plugin.json: |
    {
      "name": "my-plugin",
      "version": "1.0.0",
      "tools": [
        {
          "name": "my_tool",
          "description": "Does something useful",
          "command": "my-command --json"
        }
      ]
    }
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `THIMBLE_DATA_DIR` | `/data` | Directory for SQLite databases and session data |

## Architecture

The deployment runs thimble as a sidecar with:

- **MCP server** on stdio (single-binary, no daemon or gRPC port)
- **PersistentVolumeClaim** for SQLite databases and session state
- **ConfigMap** for plugin definitions mounted as files
- **Health checks** via `thimble doctor` liveness probe
