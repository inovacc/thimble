[English](../README.md) | [Português](README.pt-BR.md) | [Español](README.es.md)

# thimble

Plugin MCP de binario unico para asistentes de programacion con IA. Proporciona una base de conocimiento FTS5, ejecucion poliglota de codigo, persistencia de sesion y aplicacion de politicas de seguridad — todo en proceso, sin necesidad de daemon.

## Caracteristicas

- **Servidor MCP** — transporte stdio con 41 herramientas nativas + ~80 herramientas de la API de GitHub + plugins dinamicos (execute, search, index, fetch, analyze, batch, delegate, reports, git, gh, lint)
- **Binario Unico** — cada instancia es autonoma; sin daemon, sin gRPC, sin cadena de descubrimiento
- **Base de Conocimiento FTS5** — busqueda con ranking BM25 y fallback en 5 capas (Porter, trigrama, fuzzy, embedding, TF-IDF)
- **Ejecutor Poliglota** — 11 lenguajes (shell, Python, JS/TS, Go, Rust, Ruby, PHP, Perl, R, Elixir)
- **Analisis de Codigo** — 6 parsers (Go, Python, Rust, TypeScript, Protobuf, Shell), extraccion de simbolos, grafos de llamada entre lenguajes
- **Integracion Git** — 13 herramientas MCP de git (status, diff, log, blame, branches, stash, commit, changelog, merge, rebase, conflicts, validate_branch, lint_commit) + politicas de seguridad integradas con git
- **Integracion GitHub** — 8 herramientas gh CLI via subproceso (incl. plantillas de PR) + ~80 herramientas de la API de GitHub via importacion de github-mcp-server
- **Integracion con Lint** — golangci-lint v2 via subproceso (requiere `golangci-lint` en PATH), soporte de autocorreccion
- **Marketplace de Plugins** — Instala plugins de la comunidad desde el [registro](https://github.com/inovacc/thimble-plugins) (`thimble plugin install docker`), o desde cualquier URL/ruta de GitHub. Definiciones de herramientas en JSON con sustitucion de plantillas.
- **Persistencia de Sesion** — seguimiento de eventos por proyecto, snapshots de reanudacion, contexto con presupuesto por prioridad
- **Seguridad** — aplicacion de politicas Bash, globs de denegacion de rutas de archivo, deteccion de escape de shell, bloqueo de comandos peligrosos git/gh
- **9 Adaptadores de IDE** — Claude Code, Gemini CLI, VS Code Copilot, Cursor, OpenCode, Codex, Kiro, OpenClaw, Antigravity
- **Reportes Automaticos** — Reportes de diagnostico y estadisticas en markdown legible por IA
- **Rastreo OpenTelemetry** — Traces distribuidos entre llamadas de herramientas MCP

## Instalacion

**Linux / macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/inovacc/thimble/main/scripts/install.ps1 | iex
```

**Go install:**
```bash
go install github.com/inovacc/thimble/cmd/thimble@latest
```

O descarga un binario precompilado desde [Releases](https://github.com/inovacc/thimble/releases).

**Herramientas externas opcionales:** `golangci-lint` (para herramientas de lint) y `gh` CLI (para herramientas gh) deben instalarse por separado si deseas usar esas funcionalidades. Ambos funcionan correctamente si no se encuentran en el PATH.

## Inicio Rapido

### 1. Configura hooks para tu IDE

```bash
# Detectar plataforma automaticamente
thimble setup

# O especificar explicitamente
thimble setup --client claude
thimble setup --client claude --plugin   # instalar plugin (skills, hooks, config MCP)
thimble setup --client gemini
thimble setup --client cursor
```

### 2. Usa como servidor MCP (por defecto)

```bash
# Ejecuta servidor MCP via stdio (usado por la config de hook de la IDE)
thimble
```

### 3. Diagnosticos

```bash
thimble doctor              # Ejecutar verificaciones de diagnostico
thimble hooklog             # Ver logs de interaccion de hooks
thimble hooklog --blocked   # Mostrar solo hooks bloqueados
```

## Comandos

| Comando | Descripcion |
|---------|-------------|
| *(por defecto)* | Ejecutar servidor MCP via stdio |
| `hook <platform> <event>` | Despachar evento de hook (en proceso, ~10ms) |
| `setup --client <name>` | Configurar hooks para IDE |
| `doctor` | Ejecutar verificaciones de diagnostico |
| `report [list\|show\|delete]` | Gestionar reportes generados automaticamente |
| `upgrade` | Auto-actualizacion desde GitHub Releases |
| `lint [--fix] [--fast]` | Ejecutar golangci-lint |
| `plugin list` | Listar plugins instalados y sus herramientas |
| `plugin install <source>` | Instalar plugin del registro, URL o ruta de GitHub |
| `plugin remove <name>` | Eliminar un plugin instalado |
| `plugin search` | Explorar los plugins disponibles en el registro |
| `plugin dir` | Mostrar la ruta del directorio de plugins |
| `plugin update [name]` | Actualizar plugins del registro (`--check` para simulacion) |
| `hooklog [--blocked] [--clear]` | Mostrar logs de interaccion de hooks |
| `release-notes` | Generar notas de release a partir del changelog de git |
| `publish` | Commit, tag, push y monitorear pipeline de CI |
| `publish-status` | Verificar estado de la pipeline de publicacion/release |
| `version` | Mostrar informacion de version |

## Herramientas MCP

### Herramientas Nativas (41)

| Herramienta | Descripcion |
|-------------|-------------|
| `ctx_execute` | Ejecutar codigo en 11 lenguajes, indexar salida automaticamente |
| `ctx_execute_file` | Ejecutar codigo con contenido de archivo via variable `FILE_CONTENT` |
| `ctx_index` | Indexar contenido en la base de conocimiento FTS5 |
| `ctx_search` | Buscar en la base de conocimiento con fallback en 5 capas |
| `ctx_fetch_and_index` | Obtener URL, convertir HTML a Markdown, indexar |
| `ctx_batch_execute` | Ejecutar multiples comandos + consultas de busqueda en una sola llamada |
| `ctx_stats` | Estadisticas de la base de conocimiento |
| `ctx_doctor` | Verificacion de salud e informacion de runtime |
| `ctx_analyze` | Analizar codebase, extraer simbolos, indexar en la base de conocimiento |
| `ctx_symbols` | Consultar simbolos de codigo extraidos por nombre, tipo o paquete |
| `ctx_delegate` | Enviar tarea en segundo plano para ejecucion asincrona |
| `ctx_delegate_status` | Verificar progreso/resultado de tarea en segundo plano |
| `ctx_delegate_cancel` | Cancelar una tarea en segundo plano en ejecucion |
| `ctx_delegate_list` | Listar todas las tareas en segundo plano con estado |
| `ctx_report_list` | Listar reportes generados automaticamente |
| `ctx_report_show` | Mostrar un reporte especifico |
| `ctx_report_delete` | Eliminar un reporte |
| `ctx_git_status` | Estado del repositorio, rama, cambios staged/unstaged |
| `ctx_git_diff` | Diff con control de contexto y filtrado de archivos |
| `ctx_git_log` | Historial de commits con filtrado por rango |
| `ctx_git_blame` | Atribucion por linea con informacion de commit |
| `ctx_git_branches` | Listar ramas con seguimiento upstream |
| `ctx_git_stash` | Listar, mostrar, guardar, aplicar, eliminar stashes |
| `ctx_git_commit` | Stage de archivos, crear commits con validacion |
| `ctx_git_changelog` | Generacion de changelog con conventional commits |
| `ctx_gh` | Ejecutar comandos gh CLI |
| `ctx_gh_pr_status` | Estado de pull request para la rama actual |
| `ctx_gh_run_status` | Estado de ejecucion de workflow de GitHub Actions |
| `ctx_gh_issue_list` | Listar issues del repositorio |
| `ctx_gh_search` | Buscar issues, PRs, codigo en todo GitHub |
| `ctx_gh_api` | Solicitudes directas a la API de GitHub |
| `ctx_gh_repo_view` | Metadatos e informacion del repositorio |
| `ctx_gh_pr_template` | Obtener plantilla de PR del repositorio |
| `ctx_git_merge` | Merge de ramas con deteccion de conflictos |
| `ctx_git_rebase` | Rebase con abort/continue/skip |
| `ctx_git_conflicts` | Detectar y resolver conflictos de git |
| `ctx_git_validate_branch` | Validar convenciones de nombres de ramas |
| `ctx_git_lint_commit` | Validar mensajes de commit contra convenciones |
| `ctx_lint` | Ejecutar golangci-lint en el proyecto/archivos |
| `ctx_lint_fix` | Ejecutar golangci-lint con --fix para correcciones automaticas |
| `ctx_upgrade` | Auto-actualizacion del binario thimble |

### Herramientas de la API de GitHub (~80)

Importadas de `github-mcp-server` v0.33.0 — cubre issues, PRs, repositorios, actions, code scanning, Dependabot, discussions, gists, projects, notificaciones, labels, alertas de seguridad, stars, usuarios/equipos y Copilot. Requiere `GITHUB_PERSONAL_ACCESS_TOKEN`.

### Herramientas Dinamicas de Plugins

Instala plugins de la comunidad desde el [registro de plugins](https://github.com/inovacc/thimble-plugins) o desde cualquier URL:

```bash
# Explorar plugins disponibles
thimble plugin search

# Instalar del registro (por nombre)
thimble plugin install docker
thimble plugin install kubernetes
thimble plugin install terraform

# Instalar desde GitHub
thimble plugin install github.com/user/repo/my-plugin.json

# Instalar desde URL
thimble plugin install https://example.com/plugin.json

# Gestionar
thimble plugin list              # mostrar instalados
thimble plugin update            # actualizar todos del registro
thimble plugin update docker     # actualizar plugin especifico
thimble plugin remove docker     # desinstalar
```

**Plugins disponibles en el registro:**

| Plugin | Herramientas | Descripcion |
|--------|--------------|-------------|
| **docker** | `ctx_docker_ps`, `ctx_docker_logs`, `ctx_docker_images`, `ctx_docker_stats` | Gestion de contenedores |
| **kubernetes** | `ctx_k8s_pods`, `ctx_k8s_logs`, `ctx_k8s_describe`, `ctx_k8s_events` | Operaciones de cluster |
| **terraform** | `ctx_tf_plan`, `ctx_tf_state`, `ctx_tf_output`, `ctx_tf_validate` | Gestion de infraestructura |

**Crea tu propio plugin** — consulta la [guia de creacion de plugins](https://github.com/inovacc/thimble-plugins#create-your-own-plugin).

## Desarrollo

```bash
task build    # Compilar binario
task test     # Ejecutar pruebas
task lint     # golangci-lint
task release  # GoReleaser (requiere tag git)
```

## Arquitectura

```
thimble (binario unico)
  |
  |-- MCP Bridge (stdio) ---- ContentStore (FTS5/SQLite)
  |   (41 nativas + ~80 GH)   SessionDB (eventos, snapshots)
  |                            PolyglotExecutor (11 lenguajes)
  |-- Hook Dispatcher -------- Security Engine (politicas)
  |   (PreToolUse/PostToolUse) CodeAnalysis (6 parsers)
  |                            TaskDelegate (segundo plano)
  |-- Comandos CLI ----------- GitOps (13 operaciones)
  |   (lint, hooklog, doctor)  GhCli (subproceso)
  |                            Linter (subproceso)
  |-- Sistema de Plugins ----- Report Engine
      (hot-reload, registro)   OTel Tracing
```

## Licencia

BSD 3-Clause
