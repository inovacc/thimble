[English](../README.md) | [Português](README.pt-BR.md) | [Español](README.es.md)

# thimble

Plugin MCP de binario unico para assistentes de programacao com IA. Oferece uma base de conhecimento FTS5, execucao poliglota de codigo, persistencia de sessao e aplicacao de politicas de seguranca — tudo em processo, sem necessidade de daemon.

## Funcionalidades

- **Servidor MCP** — transporte stdio com 41 ferramentas nativas + ~80 ferramentas da API do GitHub + plugins dinamicos (execute, search, index, fetch, analyze, batch, delegate, reports, git, gh, lint)
- **Binario Unico** — cada instancia e autonoma; sem daemon, sem gRPC, sem cadeia de descoberta
- **Base de Conhecimento FTS5** — busca com ranking BM25 e fallback em 5 camadas (Porter, trigrama, fuzzy, embedding, TF-IDF)
- **Executor Poliglota** — 11 linguagens (shell, Python, JS/TS, Go, Rust, Ruby, PHP, Perl, R, Elixir)
- **Analise de Codigo** — 6 parsers (Go, Python, Rust, TypeScript, Protobuf, Shell), extracao de simbolos, grafos de chamada entre linguagens
- **Integracao Git** — 13 ferramentas MCP de git (status, diff, log, blame, branches, stash, commit, changelog, merge, rebase, conflicts, validate_branch, lint_commit) + politicas de seguranca integradas ao git
- **Integracao GitHub** — 8 ferramentas gh CLI via subprocesso (incl. templates de PR) + ~80 ferramentas da API do GitHub via importacao do github-mcp-server
- **Integracao com Lint** — golangci-lint v2 via subprocesso (requer `golangci-lint` no PATH), suporte a auto-correcao
- **Marketplace de Plugins** — Instale plugins da comunidade pelo [registro](https://github.com/inovacc/thimble-plugins) (`thimble plugin install docker`), ou por qualquer URL/caminho do GitHub. Definicoes de ferramentas em JSON com substituicao de templates.
- **Persistencia de Sessao** — rastreamento de eventos por projeto, snapshots de retomada, contexto com orcamento por prioridade
- **Seguranca** — aplicacao de politicas Bash, globs de negacao de caminhos de arquivo, deteccao de escape de shell, bloqueio de comandos perigosos git/gh
- **9 Adaptadores de IDE** — Claude Code, Gemini CLI, VS Code Copilot, Cursor, OpenCode, Codex, Kiro, OpenClaw, Antigravity
- **Relatorios Automaticos** — Relatorios de diagnostico e estatisticas em markdown legivel por IA
- **Rastreamento OpenTelemetry** — Traces distribuidos entre chamadas de ferramentas MCP

## Instalacao

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

Ou baixe um binario pre-compilado em [Releases](https://github.com/inovacc/thimble/releases).

**Ferramentas externas opcionais:** `golangci-lint` (para ferramentas de lint) e `gh` CLI (para ferramentas gh) devem ser instalados separadamente se voce quiser usar essas funcionalidades. Ambos funcionam normalmente caso nao sejam encontrados no PATH.

## Inicio Rapido

### 1. Configure hooks para sua IDE

```bash
# Detectar plataforma automaticamente
thimble setup

# Ou especificar explicitamente
thimble setup --client claude
thimble setup --client claude --plugin   # instalar plugin (skills, hooks, config MCP)
thimble setup --client gemini
thimble setup --client cursor
```

### 2. Use como servidor MCP (padrao)

```bash
# Executa servidor MCP via stdio (usado pela config de hook da IDE)
thimble
```

### 3. Diagnosticos

```bash
thimble doctor              # Executar verificacoes de diagnostico
thimble hooklog             # Ver logs de interacao de hooks
thimble hooklog --blocked   # Mostrar apenas hooks bloqueados
```

## Comandos

| Comando | Descricao |
|---------|-----------|
| *(padrao)* | Executar servidor MCP via stdio |
| `hook <platform> <event>` | Despachar evento de hook (em processo, ~10ms) |
| `setup --client <name>` | Configurar hooks para IDE |
| `doctor` | Executar verificacoes de diagnostico |
| `report [list\|show\|delete]` | Gerenciar relatorios gerados automaticamente |
| `upgrade` | Auto-atualizacao via GitHub Releases |
| `lint [--fix] [--fast]` | Executar golangci-lint |
| `plugin list` | Listar plugins instalados e suas ferramentas |
| `plugin install <source>` | Instalar plugin do registro, URL ou caminho do GitHub |
| `plugin remove <name>` | Remover um plugin instalado |
| `plugin search` | Navegar pelos plugins disponiveis no registro |
| `plugin dir` | Mostrar o caminho do diretorio de plugins |
| `plugin update [name]` | Atualizar plugins do registro (`--check` para simulacao) |
| `hooklog [--blocked] [--clear]` | Mostrar logs de interacao de hooks |
| `release-notes` | Gerar notas de release a partir do changelog do git |
| `publish` | Commit, tag, push e monitorar pipeline de CI |
| `publish-status` | Verificar status da pipeline de publicacao/release |
| `version` | Exibir informacoes de versao |

## Ferramentas MCP

### Ferramentas Nativas (41)

| Ferramenta | Descricao |
|------------|-----------|
| `ctx_execute` | Executar codigo em 11 linguagens, indexar saida automaticamente |
| `ctx_execute_file` | Executar codigo com conteudo de arquivo via variavel `FILE_CONTENT` |
| `ctx_index` | Indexar conteudo na base de conhecimento FTS5 |
| `ctx_search` | Buscar na base de conhecimento com fallback em 5 camadas |
| `ctx_fetch_and_index` | Buscar URL, converter HTML para Markdown, indexar |
| `ctx_batch_execute` | Executar multiplos comandos + consultas de busca em uma unica chamada |
| `ctx_stats` | Estatisticas da base de conhecimento |
| `ctx_doctor` | Verificacao de saude e informacoes de runtime |
| `ctx_analyze` | Analisar codebase, extrair simbolos, indexar na base de conhecimento |
| `ctx_symbols` | Consultar simbolos de codigo extraidos por nome, tipo ou pacote |
| `ctx_delegate` | Submeter tarefa em segundo plano para execucao assincrona |
| `ctx_delegate_status` | Verificar progresso/resultado de tarefa em segundo plano |
| `ctx_delegate_cancel` | Cancelar uma tarefa em segundo plano em execucao |
| `ctx_delegate_list` | Listar todas as tarefas em segundo plano com status |
| `ctx_report_list` | Listar relatorios gerados automaticamente |
| `ctx_report_show` | Mostrar um relatorio especifico |
| `ctx_report_delete` | Excluir um relatorio |
| `ctx_git_status` | Status do repositorio, branch, alteracoes staged/unstaged |
| `ctx_git_diff` | Diff com controle de contexto e filtragem de arquivos |
| `ctx_git_log` | Historico de commits com filtragem por intervalo |
| `ctx_git_blame` | Atribuicao por linha com informacoes de commit |
| `ctx_git_branches` | Listar branches com rastreamento upstream |
| `ctx_git_stash` | Listar, mostrar, salvar, aplicar, remover stashes |
| `ctx_git_commit` | Stage de arquivos, criar commits com validacao |
| `ctx_git_changelog` | Geracao de changelog com conventional commits |
| `ctx_gh` | Executar comandos gh CLI |
| `ctx_gh_pr_status` | Status de pull request para a branch atual |
| `ctx_gh_run_status` | Status de execucao de workflow do GitHub Actions |
| `ctx_gh_issue_list` | Listar issues do repositorio |
| `ctx_gh_search` | Buscar issues, PRs, codigo em todo o GitHub |
| `ctx_gh_api` | Requisicoes diretas a API do GitHub |
| `ctx_gh_repo_view` | Metadados e informacoes do repositorio |
| `ctx_gh_pr_template` | Obter template de PR do repositorio |
| `ctx_git_merge` | Merge de branches com deteccao de conflitos |
| `ctx_git_rebase` | Rebase com abort/continue/skip |
| `ctx_git_conflicts` | Detectar e resolver conflitos do git |
| `ctx_git_validate_branch` | Validar convencoes de nomes de branches |
| `ctx_git_lint_commit` | Validar mensagens de commit contra convencoes |
| `ctx_lint` | Executar golangci-lint no projeto/arquivos |
| `ctx_lint_fix` | Executar golangci-lint com --fix para correcoes automaticas |
| `ctx_upgrade` | Auto-atualizacao do binario thimble |

### Ferramentas da API do GitHub (~80)

Importadas do `github-mcp-server` v0.33.0 — cobre issues, PRs, repositorios, actions, code scanning, Dependabot, discussions, gists, projects, notificacoes, labels, alertas de seguranca, stars, usuarios/equipes e Copilot. Requer `GITHUB_PERSONAL_ACCESS_TOKEN`.

### Ferramentas Dinamicas de Plugins

Instale plugins da comunidade pelo [registro de plugins](https://github.com/inovacc/thimble-plugins) ou por qualquer URL:

```bash
# Navegar pelos plugins disponiveis
thimble plugin search

# Instalar do registro (por nome)
thimble plugin install docker
thimble plugin install kubernetes
thimble plugin install terraform

# Instalar do GitHub
thimble plugin install github.com/user/repo/my-plugin.json

# Instalar por URL
thimble plugin install https://example.com/plugin.json

# Gerenciar
thimble plugin list              # mostrar instalados
thimble plugin update            # atualizar todos do registro
thimble plugin update docker     # atualizar plugin especifico
thimble plugin remove docker     # desinstalar
```

**Plugins disponiveis no registro:**

| Plugin | Ferramentas | Descricao |
|--------|-------------|-----------|
| **docker** | `ctx_docker_ps`, `ctx_docker_logs`, `ctx_docker_images`, `ctx_docker_stats` | Gerenciamento de containers |
| **kubernetes** | `ctx_k8s_pods`, `ctx_k8s_logs`, `ctx_k8s_describe`, `ctx_k8s_events` | Operacoes de cluster |
| **terraform** | `ctx_tf_plan`, `ctx_tf_state`, `ctx_tf_output`, `ctx_tf_validate` | Gerenciamento de infraestrutura |

**Crie seu proprio plugin** — veja o [guia de criacao de plugins](https://github.com/inovacc/thimble-plugins#create-your-own-plugin).

## Desenvolvimento

```bash
task build    # Compilar binario
task test     # Executar testes
task lint     # golangci-lint
task release  # GoReleaser (requer tag git)
```

## Arquitetura

```
thimble (binario unico)
  |
  |-- MCP Bridge (stdio) ---- ContentStore (FTS5/SQLite)
  |   (41 nativas + ~80 GH)   SessionDB (eventos, snapshots)
  |                            PolyglotExecutor (11 linguagens)
  |-- Hook Dispatcher -------- Security Engine (politicas)
  |   (PreToolUse/PostToolUse) CodeAnalysis (6 parsers)
  |                            TaskDelegate (segundo plano)
  |-- Comandos CLI ----------- GitOps (13 operacoes)
  |   (lint, hooklog, doctor)  GhCli (subprocesso)
  |                            Linter (subprocesso)
  |-- Sistema de Plugins ----- Report Engine
      (hot-reload, registro)   OTel Tracing
```

## Licenca

BSD 3-Clause
