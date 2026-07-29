---
name: create-issue
description: Skill canônica para criar issues — use sempre que o pedido for abrir/registrar uma issue. Pergunta PRIMEIRO em qual canal a issue vai (GitHub ou outro), guia a configuração da ferramenta escolhida quando ela exigir acesso/credencial, coleta o mínimo necessário (título, descrição, critério de aceite, labels), confirma e cria. Hoje implementa apenas GitHub (via `gh`), mas é desenhada para abrir para outros destinos (Jira, GitLab, Linear...). Gatilhos: "cria uma issue", "abre uma issue", "registra isso como issue", "/create-issue".
user-invocable: true
allowed-tools:
  - Read
  - AskUserQuestion
  - Bash(gh *)
  - Bash(git *)
  - Bash(command -v *)
---

# /create-issue — Criar issues rapidamente

Esta skill cria uma ou mais issues a partir de uma descrição: coleta o mínimo
necessário, **confirma** e cria no destino escolhido. É para quando o usuário já
sabe o que quer registrar.

> Se a ideia ainda é crua e precisa virar spec e tarefas antes, o caminho é a `/kickoff`.

Argumentos recebidos: `$ARGUMENTS` (pode conter a descrição da issue; pode estar vazio).

Idioma: conduza tudo em **português (BR)**, a menos que o usuário escreva em outro idioma.

```
escolher DESTINO → configurar a ferramenta (se preciso) → coletar o que falta → CONFIRMAR → criar
```

**Esta é a skill canônica para criar issues.** Sempre que o pedido for abrir/registrar
uma issue, é ela que conduz — inclusive **perguntando primeiro em qual canal** a issue
vai (GitHub ou outro) e guiando a configuração daquele canal quando ele exigir algo.

---

## Fluxo do projeto (onde esta skill se encaixa)

```
/kickoff → /create-issue → /create-branch → (implementação) → /conventional-commits
```

- [[kickoff]] (`/kickoff`) — quando a ideia ainda é crua e precisa virar spec/tarefas
  antes. A `kickoff` **delega para esta skill** a criação das issues do plano.
- [[create-issue]] (`/create-issue`) — **você está aqui**: fonte canônica de criação de issue.
- [[create-branch]] (`/create-branch`) — **próximo passo** depois da issue: a branch de
  trabalho antes de codar.
- [[conventional-commits]] (`/conventional-commits`) — commit ao concluir a issue.

---

## Princípios

1. **Atrito mínimo.** Se o usuário já deu título e descrição, não fique perguntando.
   Pergunte só o que falta para uma issue boa.
2. **Confirmação antes de criar.** Sempre mostre o preview (título + corpo + labels +
   destino) e crie só depois do "pode criar". Criar issue é ação externa e visível.
3. **Não invente requisito.** O que não foi dito não entra no corpo. Se for importante e
   estiver faltando, pergunte; se for detalhe menor, deixe de fora.
4. **Reportar, não forçar.** Se algo falhar (sem auth, label inexistente, sem remote),
   pare e explique — não tente contornar silenciosamente.

---

## O que esta skill precisa ACESSAR

Declarado aqui para ficar explícito o que a skill toca no ambiente:

- **`gh` (GitHub CLI)** — para criar a issue no GitHub. Precisa estar instalado e
  autenticado (escopo de `repo`).
- **Git remote** — o repositório de destino é inferido do `git remote` do diretório
  atual. Se houver mais de um repo possível, confirme com o usuário.
- **Leitura local opcional** — se a issue referenciar um spec (`docs/specs/...`) ou
  arquivo do projeto, pode ler para enriquecer o contexto. Nunca cria/edita arquivos.

> A skill **não** edita código, não faz commit e não abre PR. O escopo é exclusivamente
> abrir issues.

---

## O que esta skill precisa SOLICITAR ao usuário

Campos de uma issue (peça só o que faltar; use `AskUserQuestion` quando fizer sentido
oferecer opções):

| Campo | Obrigatório? | Observação |
|-------|--------------|------------|
| **Título** | Sim | Curto e acionável. Se vier só uma frase longa, proponha um título enxuto. |
| **Descrição / contexto** | Sim | O quê e por quê. Pode vir dos argumentos. |
| **Critério de aceite** | Recomendado | "Como saber que terminou." Ofereça-se para rascunhar a partir da descrição. |
| **Labels** | Opcional | Só aplique labels que **já existam** no repo (veja abaixo). |
| **Milestone / assignee** | Opcional | Só se o usuário pedir. Não preencha por conta própria. |
| **Destino** | Sim — **sempre pergunte** | GitHub ou outro canal. É o **primeiro** passo. Veja "Destinos". |

Se o usuário pedir **várias issues de uma vez**, confirme a lista (uma linha por issue)
antes de criar e depois crie todas, reportando os links no final.

---

## Fluxo

### 1. Escolher o destino (sempre, primeiro)
**Antes de qualquer outra coisa**, pergunte em qual canal a issue será criada — com
`AskUserQuestion`, oferecendo os destinos como opções (GitHub como primeira, marcada
"(Recomendado)", já que é o único implementado hoje). O usuário pode usar "Outro" para
indicar um canal ainda não suportado.

Não assuma o destino, mesmo que o projeto tenha um remote no GitHub. A pergunta é o
primeiro passo porque tudo depois dela (configuração, campos, comando de criação) muda
conforme o canal.

### 2. Configurar a ferramenta do destino (se precisar)
Cada destino exige acesso/credencial próprio. **Cheque o que a ferramenta precisa e, se
faltar, guie o usuário a fornecer ou configurar** antes de prosseguir. Você não tem como
autenticar pelo usuário em fluxos interativos — oriente o comando e espere ele concluir.

Para **GitHub**:
1. `gh` instalado? → `command -v gh`. Se não: oriente `sudo apt-get install -y gh`.
2. Autenticado? → `gh auth status`. Se não, peça ao usuário rodar (é interativo):
   `gh auth login`.
3. Repo de destino? → `git remote -v`. Se não houver remote no GitHub, pergunte qual
   repo usar. Não assuma.

Para **outros destinos**: veja em "Destinos" o que cada um precisa. Se o destino ainda
não é suportado, diga isso e ofereça o GitHub no lugar.

### 3. Entender o pedido e coletar o que falta
- Se `$ARGUMENTS` trouxe a descrição, **reflita de volta** em 1 frase o que entendeu e
  proponha um **título**.
- Identifique o que falta (normalmente: critério de aceite e labels). Pergunte só isso.
- Para escolhas (labels etc.), use `AskUserQuestion` com a recomendação como primeira
  opção marcada "(Recomendado)" e o porquê na descrição. O usuário sempre pode usar "Outro".
- Labels (GitHub): liste as existentes com `gh label list`. **Não invente label.** Se a
  desejada não existe, pergunte se quer criá-la (`gh label create`) ou seguir sem ela.

### 4. Confirmar
Mostre o **preview** completo:

```
Destino: GitHub (<owner/repo>)
Título:  <título>
Labels:  <labels ou —>
Corpo:
  <corpo renderizado>
```

Só siga após "pode criar" (ou equivalente). Se estiver revisando, ajuste e mostre de novo.

### 5. Criar
Para **GitHub**, use o template de corpo abaixo:

```bash
gh issue create \
  --title "<título>" \
  --body "$(cat <<'EOF'
## Contexto
<o quê e por quê>

## Critério de aceite
- [ ] <...>

## Referências
<links, spec docs/specs/NN-slug.md, issues relacionadas, ou —>
EOF
)" \
  --label "<label-existente>"   # repita --label por label; omita se não houver
```

Depois de criar:
- **Mostre a(s) URL(s)** da(s) issue(s) criada(s).
- Se foram várias, liste todas com seus números/links.
- Se algo falhar, **pare e reporte** o erro exato — não tente de novo às cegas.

---

## Destinos (extensível)

Hoje só **GitHub** está implementado. A skill é desenhada para crescer: cada destino
tem (a) sua checagem de ambiente, (b) seu mapeamento de campos e (c) seu comando de
criação. Para adicionar um destino no futuro (ex.: **Jira**, **GitLab**, **Linear**),
estenda esta seção com:

- **O que precisa acessar** (ex.: Jira → URL da instância, token de API, project key).
- **O que precisa solicitar** (ex.: Jira → tipo de issue: Story/Task/Bug, epic link).
- **Como criar** (ex.: Jira → API REST / `jira` CLI; GitLab → `glab issue create`).

Enquanto um destino não estiver implementado, se o usuário pedir, diga que ainda não é
suportado e ofereça criar no GitHub no lugar.

---

## Guardrails

- **Confirmação explícita antes de criar.** Sempre mostre o preview.
- **Sem invenção.** Requisito não dito não entra no corpo; label inexistente não é aplicada.
- **Falhou? Reporta.** Não force contorno em erro de auth/permissão/label.
