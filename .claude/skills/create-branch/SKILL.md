---
name: create-branch
description: Skill canônica para criar a branch de trabalho de uma issue seguindo o padrão Conventional Branch (https://conventionalbranch.org/). Use SEMPRE que for começar a desenvolver uma issue — antes de escrever código. Identifica a issue, escolhe o prefixo (feature/, fix/, hotfix/, chore/, release/...), monta a descrição em kebab-case, confirma e cria a branch a partir da base atualizada. Gatilhos "vou desenvolver a issue N", "começar a issue", "cria a branch", "bora implementar isso", "/create-branch".
user-invocable: true
allowed-tools:
  - Read
  - AskUserQuestion
  - Bash(git *)
  - Bash(gh *)
---

# /create-branch — Branch de trabalho no padrão Conventional Branch

Esta skill cria a branch onde uma issue será desenvolvida, seguindo a
[Conventional Branch](https://conventionalbranch.org/). Ela identifica a issue,
classifica o **tipo**, monta a **descrição** em kebab-case, **confirma** com o usuário
e então cria a branch a partir da base atualizada.

```
identificar a issue → classificar (type) → montar o nome → CONFIRMAR → git checkout -b a partir da base atualizada
```

Idioma: conduza a **conversa** em **português (BR)**. O **nome da branch é sempre em inglês**.

Argumentos recebidos: `$ARGUMENTS` (pode conter número/título da issue, ou estar vazio).

---

## Fluxo do projeto (onde esta skill se encaixa)

```
/kickoff → /create-issue → /create-branch → (implementação) → /conventional-commits
```

- [[kickoff]] (`/kickoff`) — ideia crua → spec → tarefas.
- [[create-issue]] (`/create-issue`) — onde a issue nasce (a `kickoff` delega pra ela).
- [[create-branch]] (`/create-branch`) — **você está aqui**: branch de trabalho antes de codar.
- [[conventional-commits]] (`/conventional-commits`) — **próximo passo** ao concluir: o commit.

---

## Quando rodar (gatilho importante)

**Sempre que for começar a desenvolver uma issue, rode esta skill ANTES de escrever código.**
Não comece a implementar na branch atual (em especial não em `main`/`master`/`develop`):
crie primeiro a branch de trabalho. Vale para "vou pegar a issue N", "bora implementar
isso", "começar a tarefa", etc.

Encaixe no fluxo do projeto: a issue nasce na [[create-issue]] (ou na `/kickoff`), o
desenvolvimento acontece nesta branch, e o fechamento usa a [[conventional-commits]].

---

## Princípios

1. **Branch antes do código.** Nunca desenvolva direto na base. Primeiro a branch.
2. **Aderência à spec.** Estrutura `<type>/<descrição>`, só minúsculas/dígitos/hífens,
   sem espaço, underscore, acento ou caractere especial (veja "Formato").
3. **Confirmação antes de criar.** Sempre mostre o nome final e a base, e só rode
   `git checkout -b` depois do "pode".
4. **Base atualizada.** Crie a branch a partir da base correta e atualizada (normalmente
   `main`), não de um estado sujo ou defasado.
5. **Reportar, não forçar.** Working tree suja, branch já existente, sem rede para o
   `fetch` — pare e explique, não contorne em silêncio.
6. **Em dúvida, pergunte e APRENDA.** Quando o padrão da organização for ambíguo
   (incluir número da issue? `feat` ou `feature`? qual base?), **pergunte** com a
   recomendação argumentada como primeira opção e **registre** a decisão em
   "Convenções aprendidas" para virar default.

---

## Formato (Conventional Branch)

```
<type>/<descrição>
```

- **type**: obrigatório, minúsculo. Veja a tabela abaixo.
- **descrição**: curta e descritiva, em **kebab-case** (`palavras-separadas-por-hifen`).
- **Permitido**: `a-z`, `0-9`, `-` (e `.` apenas em número de versão de `release/`).
- **Proibido**: maiúsculas, espaço, `_`, acentos/especiais; hífen/ponto duplicado,
  no início ou no fim; vizinhança hífen-ponto (`v1.-2.0`).
- Pode embutir o identificador da issue na descrição: `feature/issue-123-new-login`.
- Branches de tronco (`main`, `master`, `develop`) **não** levam prefixo.

### Types aceitos

| Type | Quando usar |
|------|-------------|
| `feature/` (ou `feat/`) | Nova funcionalidade |
| `bugfix/` (ou `fix/`) | Correção de bug |
| `hotfix/` | Correção urgente (ex.: produção) |
| `release/` | Preparação de release (ex.: `release/v1.2.0`) |
| `chore/` | Tarefas sem código de produto (deps, docs, config) |

> Prefixos de agente de IA (`ai/`, `claude/`, `cursor/`, `copilot/`, `codex/`) existem na
> spec, mas para a branch de trabalho de uma issue prefira o **type por propósito** acima.

### Exemplos

```
feature/add-email-subscription
fix/send-date-calculation
chore/upgrade-next
release/v1.2.0
feature/7-newsletter-signup
```

Inválidos: `Feature/Add-Login`, `feature/new--login`, `fix/header_bug`, `feat/assinatura`.

---

## Fluxo

### 1. Identificar a issue
- Use `$ARGUMENTS` se trouxe número/título. Se veio só o número, busque o título e os
  labels para classificar: `gh issue view <N> --json number,title,labels`.
- Se não veio nada, pergunte qual issue será desenvolvida (número ou título). Sem issue
  clara, ainda dá para criar a branch a partir de uma descrição — mas confirme.

### 2. Classificar o type
- Deduza o **type** pelo trabalho da issue e pelos **labels** (`bug` → `fix`,
  `enhancement`/`feature` → `feature`, manutenção/deps/docs → `chore`).
- Na dúvida entre dois (ex.: `feature` vs `chore`), use `AskUserQuestion` com a
  recomendação como primeira opção "(Recomendado)".

### 3. Montar o nome
- Descrição em **kebab-case**, em inglês, curta e específica — derivada do título da issue.
- Normalize: minúsculas; troque espaços/`_`/acentos por `-`; remova caracteres especiais;
  colapse hífens repetidos; tire hífen do início/fim.
- **Número da issue**: inclua conforme a convenção aprendida (veja abaixo). Se ainda não
  houver convenção, pergunte e registre.

### 4. Conferir base e estado do repositório
- `git status` — se a working tree estiver suja, **pare e pergunte** (commitar, dar stash
  ou abortar). Não crie a branch por cima de mudanças não relacionadas.
- Descubra a base (normalmente `main`) e atualize: `git fetch origin` e baseie a branch
  em `origin/<base>` (ou faça checkout da base e `git pull` antes). Confirme a base se
  houver ambiguidade.
- Cheque se a branch já existe (`git branch --list <nome>` / `git ls-remote`). Se existir,
  reporte e ofereça apenas dar checkout nela.

### 5. Confirmar
Mostre o **preview** e só siga após "pode":

```
Issue:  #<N> — <título>
Branch: <type>/<descrição>
Base:   <base atualizada, ex.: origin/main>
```

### 6. Criar
```bash
git fetch origin
git checkout -b <type>/<descrição> origin/<base>
```

Depois de criar:
- Confirme com `git status` / `git branch --show-current` que você está na branch nova.
- **Não dê push automático.** Por padrão, aguarde a primeira entrega/revisão antes de
  publicar a branch (`git push -u`). Só empurre quando o usuário pedir — alinhado com
  [[aguardar-revisao-antes-do-push]].
- Se falhar (nome inválido, branch existente, base inexistente), **pare e reporte**.

---

## Aprender o padrão da organização

A skill **pergunta e aprende** quando o padrão é ambíguo (veja [[skill-em-duvida-pergunta-e-aprende]]).

1. **Antes de decidir**, cheque: esta seção "Convenções aprendidas"; as branches
   existentes (`git branch -a`) para captar o estilo real; a memória do projeto.
2. **Se ainda houver dúvida**, use `AskUserQuestion` com a recomendação como primeira
   opção "(Recomendado)" e o porquê — nunca chute o padrão.
3. **Depois que o usuário decidir**, **registre** em "Convenções aprendidas" (e/ou na
   memória do projeto) para a skill já seguir sem perguntar de novo.

### Convenções aprendidas

Padrões confirmados pelo usuário (cresce com o tempo):

- **Inclui o número da issue** na descrição, logo após o type: `<type>/<N>-<slug>`
  (ex.: `feature/7-newsletter-signup`). Rastreia melhor a issue de origem.
- **Base padrão**: `main`. Crie a branch a partir de `origin/main` atualizada.

---

## Guardrails

- **Aderência à Conventional Branch** — `<type>/<descrição>`, kebab-case, sem maiúsculas,
  espaço, `_`, acento ou hífen/ponto duplicado.
- **Branch antes do código**; nunca desenvolver direto na base.
- **Base atualizada** — `fetch` antes; baseie em `origin/<base>`.
- **Confirmação explícita antes de criar.** Sempre mostre o preview (issue, branch, base).
- **Working tree suja? Pare e pergunte.** Não crie por cima de mudanças não relacionadas.
- **Aguarda revisão antes do push.** Não publica a branch sem aprovação.
- **Falhou? Reporta.** Não force contorno em nome inválido/branch existente/erro de base.
