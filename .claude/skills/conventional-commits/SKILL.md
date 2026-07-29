---
name: conventional-commits
description: Skill canônica para criar commits seguindo o padrão Conventional Commits 1.0.0 (https://www.conventionalcommits.org/en/v1.0.0/). Use sempre que for hora de commitar — em especial QUANDO UMA ISSUE FOR CONCLUÍDA, peça autorização ao usuário para rodar esta skill antes de commitar. Monta a mensagem (type, escopo, descrição, corpo, footer), referencia a issue e NUNCA usa emojis. Gatilhos: "faz o commit", "commita isso", "conventional commit", "concluí a issue", "/conventional-commits".
user-invocable: true
allowed-tools:
  - Read
  - AskUserQuestion
  - Bash(git *)
---

# /conventional-commits — Commits no padrão Conventional Commits 1.0.0

Esta skill monta e cria commits seguindo a especificação
[Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/).
Ela analisa o que está staged/alterado, propõe uma mensagem **sem emojis**, **confirma**
com o usuário e então commita.

```
ler o diff → classificar (type/escopo) → redigir mensagem → CONFIRMAR → git commit
```

Idioma: conduza a **conversa** em **português (BR)**, mas **escreva a mensagem de commit
em inglês** (type, descrição, corpo e footer — tudo em inglês).

Argumentos recebidos: `$ARGUMENTS` (pode conter contexto extra, número da issue, ou estar vazio).

---

## Fluxo do projeto (onde esta skill se encaixa)

```
/kickoff → /create-issue → /create-branch → (implementação) → /conventional-commits
```

- [[kickoff]] (`/kickoff`) — ideia crua → spec → tarefas.
- [[create-issue]] (`/create-issue`) — onde a issue nasce.
- [[create-branch]] (`/create-branch`) — branch de trabalho onde o código foi feito.
- [[conventional-commits]] (`/conventional-commits`) — **você está aqui**: commit ao concluir
  a issue (encerre referenciando-a, ex.: `Closes #N`).

---

## Quando rodar (gatilho importante)

**Sempre que uma issue for concluída, peça autorização ao usuário para rodar esta skill**
antes de criar o commit. Não commite por conta própria: pergunte algo como
_"A issue está concluída — quer que eu monte o commit no padrão Conventional Commits?"_
e só prossiga após o "pode".

Também vale para qualquer pedido de commit ("commita isso", "faz o commit").

---

## Princípios

1. **Sem emojis.** Nunca adicione emojis na mensagem (nem no título, nem no corpo, nem no footer).
2. **Confirmação antes de commitar.** Sempre mostre o preview da mensagem completa e só
   rode `git commit` depois do "pode".
3. **Aderência à spec.** Estrutura, types e `BREAKING CHANGE` seguem o Conventional Commits 1.0.0.
4. **Não invente mudança.** A mensagem descreve o que o diff realmente faz. Se não estiver
   claro, pergunte — não chute.
5. **Reportar, não forçar.** Se faltar algo (nada staged, repo sujo de forma inesperada,
   erro no commit), pare e explique.
6. **Em dúvida, pergunte e APRENDA.** Sempre que não souber qual é o padrão da
   organização (type vs. type, escopo, um commit ou vários, mensagem em inglês, etc.),
   **pergunte ao usuário** em vez de chutar — e **registre a decisão** para que ela
   vire o padrão das próximas vezes (veja "Aprender o padrão da organização").

---

## Formato da mensagem (spec 1.0.0)

```
<type>[escopo opcional][!]: <descrição>

[corpo opcional]

[footer(s) opcional(is)]
```

- **type**: obrigatório. Veja a tabela abaixo.
- **escopo**: opcional, entre parênteses, indica a área afetada — ex.: `feat(auth):`.
- **`!`**: opcional, antes do `:`, sinaliza breaking change — ex.: `feat(api)!: ...`.
- **descrição**: curta, no imperativo, minúscula, sem ponto final — ex.: `adiciona login por email`.
- **corpo**: opcional, separado por **uma linha em branco**; explica o _porquê_ e o _o quê_.
- **footer(s)**: opcional, separado por **uma linha em branco**; usa `Token: valor` ou
  `Token #valor` (ex.: `Refs: #12`, `Closes #12`, `Reviewed-by: Z`).

### Types aceitos

| Type | Quando usar |
|------|-------------|
| `feat` | Nova funcionalidade (corresponde a MINOR no semver) |
| `fix` | Correção de bug (corresponde a PATCH no semver) |
| `docs` | Só documentação |
| `style` | Formatação, espaços, ponto-e-vírgula — sem mudar lógica |
| `refactor` | Mudança de código que não corrige bug nem adiciona feature |
| `perf` | Melhoria de performance |
| `test` | Adiciona ou corrige testes |
| `build` | Sistema de build ou dependências (ex.: npm, docker) |
| `ci` | Configuração de CI (ex.: GitHub Actions) |
| `chore` | Tarefas de manutenção que não entram nas anteriores |
| `revert` | Reverte um commit anterior |

### Breaking changes
Sinalize de uma das duas formas (pode usar as duas):
- `!` após o type/escopo: `feat(api)!: remove endpoint legado`
- footer `BREAKING CHANGE: <descrição do impacto>` (em maiúsculas, com espaço).

### Exemplos (em inglês, sem emoji)

```
feat(newsletter): add email subscription

Create the subscription form and persist the lead in the database.

Refs: #7
```

```
fix: correct send date calculation

Closes #12
```

```
refactor(api)!: rename `user_name` field to `username`

BREAKING CHANGE: API consumers must update the field name.
```

---

## Validação automática neste repo (husky + commitlint)

Este repositório tem um **porteiro automático** que valida a mensagem de TODO commit:

- **husky** instala um hook `commit-msg` (`.husky/commit-msg`) que roda
  `npx --no -- commitlint --edit "$1"`.
- **commitlint** valida contra `@commitlint/config-conventional` (`commitlint.config.mjs`).
- Se a mensagem violar as regras, **o commit é barrado** — não adianta "confirmar" um
  preview que o hook depois rejeita.

Monte a mensagem **já dentro das regras do `config-conventional`**:

- **type** ∈ { `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`,
  `chore`, `revert` } (bate com a tabela de types desta skill).
- **header (1ª linha) ≤ 100 caracteres** (`header-max-length`).
- **cada linha do corpo ≤ 100 caracteres** (`body-max-line-length`) — quebre linhas longas.
- **linha em branco** obrigatória entre header → corpo e corpo → footer.
- **descrição** não termina com ponto final; type/descrição em minúsculas.

Se o hook **barrar** mesmo assim: **leia a saída do commitlint**, corrija exatamente o que
ele apontou e refaça — não force `--no-verify` nem contorne o hook sem o usuário pedir.

---

## Fluxo

### 1. Entender o estado do repositório
- `git status` e `git diff --staged` (e `git diff` se nada estiver staged) para ver o que muda.
- `git log --oneline -10` para captar o **estilo** já usado nos commits (a mensagem é sempre em inglês).
- Se **nada estiver staged**, pergunte se deve `git add` os arquivos relevantes (mostre quais).
  Não dê `git add -A` às cegas — confirme o escopo.

### 2. Classificar
- Escolha o **type** olhando o diff. Na dúvida entre dois (ex.: `feat` vs `refactor`),
  use `AskUserQuestion` oferecendo a recomendação como primeira opção "(Recomendado)".
- Defina o **escopo** se houver uma área clara (módulo/pasta). Se não, omita — escopo é opcional.
- Avalie se é **breaking change**.

### 3. Redigir a mensagem (em inglês)
- Descrição curta no imperativo, minúscula, sem ponto final — **em inglês**.
- Corpo só se agregar (o _porquê_). Não repita o óbvio do título.
- **Footer com a issue**: se houver issue associada, inclua `Refs: #N` (ou `Closes #N`
  se o commit a encerra). Pergunte o número se não souber.
- **Nunca** adicione emojis.

### 4. Confirmar
Mostre o **preview** exato da mensagem:

```
<type>(<escopo>): <descrição>

<corpo, se houver>

<footer, se houver>
```

E liste os arquivos que entram no commit. Só siga após "pode commitar" (ou equivalente).

### 5. Commitar
Use heredoc para preservar quebras de linha:

```bash
git commit -m "$(cat <<'EOF'
<type>(<escopo>): <descrição>

<corpo>

<footer>
EOF
)"
```

Depois de commitar:
- Mostre o hash e o resumo (`git log --oneline -1`).
- **Por padrão, aguarde revisão antes do `git push`.** Não faça push automático: mostre o
  commit e espere o usuário aprovar ("pode dar push"). Só empurre quando ele pedir.
- Se o commit falhar (hook, nada staged), **pare e reporte** o erro exato.

---

## Aprender o padrão da organização

A skill não decide sozinha quando o padrão é ambíguo — ela **pergunta e aprende**.

1. **Antes de decidir**, cheque convenções já registradas, nesta ordem:
   - este `SKILL.md` (a seção "Convenções aprendidas" abaixo);
   - `git log` (estilo real dos commits do repo);
   - a memória do projeto (decisões anteriores do usuário).
2. **Se ainda houver dúvida**, use `AskUserQuestion` oferecendo as opções com a
   recomendação como primeira "(Recomendado)" e o porquê — nunca chute o padrão.
3. **Depois que o usuário decidir**, **registre** a decisão para virar default:
   - adicione uma linha em "Convenções aprendidas" (abaixo) com a regra;
   - e/ou salve na memória do projeto se for uma preferência ampla.
   Assim, da próxima vez a skill já segue sem perguntar de novo.

### Convenções aprendidas

Padrões confirmados pelo usuário (cresce com o tempo):

- **Mensagem em inglês**; conversa em português.
- **Sem emojis** em nenhuma parte da mensagem.
- **Commits atômicos**: cada commit representa **uma mudança lógica completa e coerente**,
  com um **propósito único**. O que importa **não é a contagem de arquivos** — uma mudança
  coerente pode tocar vários arquivos e ainda ser **um** commit; o teste é se tudo ali serve
  ao mesmo propósito. Por outro lado, propósitos **independentes** (ex.: duas skills distintas,
  ou uma feature + um refactor não relacionado) vão em **commits separados**, mesmo que seja
  um arquivo cada.
- **Push só após revisão**: por padrão, aguardar aprovação do usuário antes do `git push`.
- **commitlint ativo**: o repo valida a mensagem via hook `commit-msg` (husky +
  `config-conventional`). Header e linhas do corpo ≤ 100 chars. Se o hook barrar, ler a
  saída e corrigir — nunca usar `--no-verify` por conta própria (veja "Validação automática").

---

## Guardrails

- **Sem emojis** em nenhuma parte da mensagem.
- **Confirmação explícita antes de commitar.** Sempre mostre o preview.
- **Aderência ao Conventional Commits 1.0.0** — type válido, descrição no imperativo,
  separações por linha em branco, `BREAKING CHANGE` quando aplicável.
- **Não inventa** mudança que o diff não tem.
- **Aguarda revisão antes do push.** Por padrão, não empurra sem aprovação do usuário.
- **Respeita o commitlint do repo.** Header e linhas do corpo ≤ 100 chars; nunca burla o
  hook com `--no-verify` sem o usuário pedir.
- **Falhou? Reporta.** Não force contorno em erro de hook/commit — leia a saída e corrija.
