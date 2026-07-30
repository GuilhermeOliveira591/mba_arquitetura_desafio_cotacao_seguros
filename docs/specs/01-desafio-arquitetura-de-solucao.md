# Desafio — Fundamentos de Arquitetura de Solução

> Status: rascunho · Autor: Guilherme · Data: 2026-07-28

## 1. Problema

A matéria **Fundamentos de Arquitetura de Solução** (Full Cycle / FCTECH) cobre 86 aulas em 9
módulos: software enterprise, solution architecture, design patterns (2 partes), AWS
Well-Architected Framework, observabilidade, on-premise vs. cloud, cloud native e Solution
Architecture Document. Para obter o **certificado de conclusão**, o aluno precisa entregar um
desafio — uma funcionalidade construída dentro de uma estrutura que a escola já deixa preparada.

Esse desafio ainda não existe para esta matéria. Sem ele não há certificação, e o conteúdo —
que é majoritariamente conceitual e documental — fica sem instrumento de verificação.

A dificuldade específica: o conteúdo é de **decisão arquitetural**, não de implementação. Um
desafio só de código não avalia TCO, escolha on-prem vs. cloud, pilares do WAF nem o SAD. Um
desafio só documental é fácil de produzir com prosa vazia e não prova entendimento — o aluno
escreve "usaremos circuit breaker" sem nunca ter visto um abrir.

## 2. Objetivo & critérios de sucesso

- **Objetivo:** entregar um desafio de certificação para a matéria, composto de repositório
  starter e enunciado, que force o aluno a decidir como arquiteto e a provar na prática o que
  decidiu.
- **Sucesso quando:**
  1. Um aluno consegue clonar/forkar o starter e ter o ambiente completo de pé com **um comando**
     (`docker compose up`), sem conta em cloud provider e sem custo.
  2. O enunciado tem critérios **quantitativos e verificáveis por leitura estática**, de modo que
     dois corretores diferentes cheguem ao mesmo veredito.
  3. A entrega do aluno exercita **as duas naturezas** da matéria: decisão documentada (SAD) e
     comportamento comprovado em execução (PoC instrumentado).

## 3. Formato & contexto

- **Tipo de entrega:** repositório starter (Go + Docker Compose) + enunciado em Markdown.
- **Novo projeto / feature em sistema existente:** projeto do zero, seguindo o padrão já
  estabelecido nos desafios existentes da Full Cycle.
- **Padrão de referência observado** (extraído de `corrige-desafio-arch` e `corrige-design-docs-ia`):
  repositório público `devfullcycle/*` que o aluno **forka**; enunciado no `README.md`; entrega na
  branch `main` com o README substituído pela documentação do processo; correção **estática**
  (sem executar a aplicação); veredito binário aprovado / não aprovado.
- **Destino do starter** (decidido em 2026-07-29): o starter é construído e publicado **neste
  repositório**, `GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros`. Ele não nasce em
  `devfullcycle/*`. O padrão acima permanece como referência de **formato** — fork, enunciado no
  `README.md`, entrega na `main`, correção estática — não de endereço.
- **Material das aulas** (decidido em 2026-07-29): o diretório `arquitetura_de_solucao/` é insumo
  de pesquisa e **não faz parte da árvore do starter**. Ele fica fora do que o aluno forka; o que
  o desafio precisa do material entra por referência no enunciado, não por cópia do conteúdo.

## 4. Escopo

### Dentro (MVP)

**A. Repositório starter (Go)** — o que a escola entrega pronto:

- `quotation-api`: API de cotação de seguros **multi-tenant** (corretoras identificadas por
  `tenant_id`). Recebe um pedido de cotação e consulta as seguradoras parceiras, agregando as
  respostas. Funciona, mas de forma ingênua: chamada síncrona, sem proteção e sem cache.
- **Três mocks de seguradoras parceiras, deliberadamente ruins** — a instabilidade é determinística
  e igual para todos os alunos, o que é pré-requisito de correção justa:
  - `partner-slow` — latência alta e constante.
  - `partner-flaky` — falha intermitente com taxa configurável.
  - `partner-degrading` — degrada conforme a carga aumenta.
- `docker-compose.yml` subindo tudo: API, 3 mocks, Redis (cache), OTel Collector, Jaeger (traces)
  e Prometheus (métricas).
- **Vácuo proposital:** o starter **não** tem circuit breaker, **não** tem cache e **não** tem
  instrumentação. É exatamente o que o aluno constrói.
- Um gerador de carga simples, para que o aluno consiga reproduzir o cenário de falha.

**B. Enunciado (`README.md` do starter)** — o que o aluno deve entregar:

1. **SAD completo**, seguindo as 8 seções ensinadas na aula:
   introdução (propósito, escopo, restrições, pressupostos) · visão geral da arquitetura ·
   requisitos funcionais e não funcionais · detalhamento da arquitetura · implementação ·
   operação e gestão de mudanças · recuperação de desastres · tecnologias, custos e pessoal (TCO).
2. **PoC de resiliência** sobre o starter:
   - **Circuit breaker** com os três estados explícitos (fechado, aberto, meio aberto).
   - **Cache** com política de invalidação **declarada e justificada** (a cotação vence — o TTL é
     decisão de negócio, não detalhe técnico).
   - **Fallback** para quando o parceiro está indisponível.
3. **Instrumentação com OpenTelemetry**, na dose necessária para **evidenciar** o comportamento:
   transições de estado do circuit breaker, `hit`/`miss` do cache e latência por parceiro.
   A observabilidade aqui é **método de prova**, não um segundo exercício.
4. **Evidências** de execução: traces e métricas mostrando o sistema sob falha, antes e depois.
5. **README do processo** substituindo o enunciado, no padrão dos desafios existentes.

**C. Amarrações que dão unidade ao desafio** (o que faz as duas metades serem uma só):

- O **cache liga-se ao TCO**: cada consulta a parceiro custa dinheiro por chamada. O aluno mede o
  efeito do hit rate na planilha de TCO da seção 8 do SAD. Uma linha de código com efeito na
  planilha financeira.
- A **regulação (SUSEP + LGPD) liga-se ao on-premise vs. cloud**: a decisão da seção 5 do SAD
  precisa ser defendida com o compliance do domínio, não com preferência pessoal.
- O **PoC é subordinado ao SAD**: o aluno implementa a fatia que o próprio documento justificou.

### Fora (não-objetivos)

- **Skill `corrige-*` de correção automatizada** — decidido explicitamente para um projeto
  seguinte, depois de ver as primeiras entregas reais. O enunciado desta entrega será escrito de
  forma já corrigível, para que a skill futura seja quase transcrição dele.
- **Solução de referência / gabarito formal** — fora do escopo desta entrega.
- Qualquer coisa que exija **conta paga em cloud provider**. Ambiente 100% local, custo zero.
- Service mesh, Kubernetes e deploy real — citáveis no SAD, não implementáveis no PoC.
- Construir a aplicação de cotação de verdade: o starter é esqueleto funcional, não produto.

## 5. Padrão do projeto

- **Arquitetura (do starter):** a definir na implementação; deve ser simples e legível, já que o
  starter é material didático — o aluno precisa entender o código em minutos para poder modificá-lo.
- **Estrutura de pastas** (definida em 2026-07-29): layout Go padrão `cmd/` + `internal/`, raso de
  propósito — o aluno tem que achar o ponto de extensão sem caçar arquivo.

  ```
  cmd/                     # um diretório por binário
    quotation-api/         # API de cotação multi-tenant (o que o aluno instrumenta)
    partner-mock/          # mock de seguradora parceira, parametrizável nos três perfis
    loadgen/               # gerador de carga para reproduzir o cenário de falha
  internal/                # código da aplicação, não importável de fora do módulo
    quotation/             # regra de cotação e handlers HTTP
    partner/               # cliente das seguradoras parceiras — onde o circuit breaker vai nascer
    platform/              # infra compartilhada: config, servidor HTTP, log
  deploy/                  # configuração das ferramentas de terceiros
    otel/                  # OTel Collector
    prometheus/            # Prometheus
  docs/specs/              # specs do projeto (não faz parte do que o aluno forka)
  docker-compose.yml       # ambiente completo: um comando sobe tudo
  Makefile                 # comandos do dia a dia (`make help` lista todos)
  ```

  Os três perfis de mock (`partner-slow`, `partner-flaky`, `partner-degrading`) são **um binário
  parametrizado**, não três programas: o comportamento ruim vira configuração no
  `docker-compose.yml`, o que mantém o starter pequeno e a instabilidade legível em um lugar só.
- **Convenções:** Conventional Commits (pipeline de skills já configurado no ambiente:
  `/kickoff → /create-issue → /create-branch → /conventional-commits`).
- **UI & styling:** N/A — não há interface. A entrega é API + documentos + dashboards de
  ferramentas de terceiros (Jaeger, Prometheus).
- **Referência a espelhar:** os desafios `corrige-desafio-arch` (Criação de Skills / Refatoração
  Arquitetural) e `corrige-design-docs-ia` (Da Reunião ao Documento). Deste último, replicar
  especialmente o **princípio anti-alucinação**: toda afirmação do documento deve ser rastreável
  ao código real ou ao enunciado; citar arquivo inexistente reprova.

## 6. Restrições & decisões técnicas

- **Stack do starter:** Go. Escolhido por ser a linguagem-assinatura da Full Cycle, por bibliotecas
  de circuit breaker exporem os três estados explicitamente (`StateClosed`/`StateOpen`/
  `StateHalfOpen`, o conceito da aula visível no código) e por binários pequenos manterem o
  `docker compose up` rápido com 7+ contêineres.
- **Infra local:** Docker Compose. Redis para cache, Jaeger para traces, Prometheus para métricas,
  OTel Collector no meio.
- **Cenário de negócio:** plataforma de cotação de seguros multi-tenant, vendida a corretoras.
  Escolhido porque a dependência externa instável é o **coração do negócio** (não um detalhe
  plantado), o vencimento da cotação torna o TTL uma decisão de negócio real, o custo por chamada
  liga cache a TCO, e SUSEP + LGPD dão peso legítimo ao debate on-prem vs. cloud.
- **Restrição de domínio:** o cenário **não pode ser venda de ingressos** — é o fio condutor usado
  nas próprias aulas (SA, SAD e design patterns). Reusá-lo faria o aluno reproduzir o exemplo em
  vez de arquitetar.
- **Custo zero para o aluno:** nenhuma dependência de conta paga. Considerado inegociável num
  desafio de certificação.
- **Correção estática:** o enunciado deve poder ser verificado sem executar a aplicação do aluno —
  padrão de todos os desafios existentes.

## 7. Riscos & incógnitas

- **Limiares quantitativos sem calibração** (decorrência direta de cortar o gabarito do escopo):
  não se sabe quantos requisitos, ADRs ou spans exigir, nem quantas horas o desafio leva. A
  primeira turma vira o teste de calibração. → Mitigação: escrever os limiares em um único bloco
  isolado do enunciado, fácil de ajustar sem reescrever o texto; e revisar após o primeiro lote.
- **PoC canibalizar o SAD:** se a implementação ficar interessante demais, o aluno vira dev e a
  matéria de arquitetura se perde. → Mitigação: manter o PoC pequeno por construção (dois patterns,
  não cinco) e deixar o peso relativo explícito no enunciado.
- **Instrumentação OTel consumir mais tempo que o circuit breaker:** o plumbing de tracing em Go é
  mais verboso que a auto-instrumentação de Node. → Mitigação: o starter já entrega o boot do SDK e
  o Collector configurados; o aluno só cria spans e métricas de negócio.
- **Mocks instáveis serem instáveis demais** (ou de menos): se `partner-flaky` falhar rápido demais,
  o circuit breaker não abre e o aluno não vê o efeito. → Mitigação: taxas de falha e latência
  parametrizadas por variável de ambiente, com valores padrão testados.
- **SAD virar preenchimento de template:** o maior risco pedagógico do desafio. → Mitigação: exigir
  números (TCO, projeção de carga, custo por chamada) e rastreabilidade ao código real, no espírito
  anti-alucinação do `design-docs-ia`.

## 8. Questões em aberto

- [x] ~~Nome da empresa fictícia e do produto no cenário de seguros.~~
      **Resolvido em 2026-07-30:** empresa **Prumo Tecnologia em Seguros**, produto
      **Prumo Cota** — ver `docs/enunciado.md`, seção 1.
- [x] ~~Nome do repositório e se ele nasce na organização `devfullcycle` como os demais.~~
      **Resolvido em 2026-07-29:** este repositório é o definitivo — ver seção 3, "Destino do
      starter".
- [ ] Quantas horas de esforço o desafio deve representar para o aluno (define a profundidade dos
      limiares do enunciado).
- [ ] Os limiares quantitativos concretos: quantos requisitos funcionais, quantos não funcionais,
      quantos ADRs (se houver), quantos spans/métricas obrigatórios, qual formato de evidência
      (screenshot? export de trace? ambos?).
- [x] ~~O SAD deve exigir diagramas? Se sim, em qual notação — C4 (citado na aula como referência),
      UML, ou 4+1? E como código (Mermaid/PlantUML, versionável e verificável) ou imagem?~~
      **Resolvido em 2026-07-30:** sim, **C4 em Mermaid**, como código dentro do Markdown — imagem
      colada não vale, porque o que precisa dar `diff` é o texto. Níveis 1 e 2 obrigatórios, nível 3
      só da fatia do PoC, nível 4 não é pedido. Ver `docs/enunciado.md`, seção 2, "Regra 3".
- [x] ~~O aluno entrega um único SAD ou o pacote pode incluir ADRs separados, como no
      `design-docs-ia`?~~ **Resolvido em 2026-07-30:** **SAD único**. As decisões vão no formato
      contexto → opções → escolha → consequências dentro da seção 4 do SAD, não como pacote de
      arquivos separados — ver `docs/enunciado.md`, seção 2.
- [ ] Há prazo/data de publicação da matéria que restrinja esta entrega?
- [ ] Bug no material de origem: `arquitetura_de_solucao/aws_well_architected_framework/10-sustentabilidade.md`
      é duplicata literal de `09-otimizacao-de-custos.md`. Corrigir na fonte — e decidir se o pilar
      de sustentabilidade entra ou não no escopo do desafio.
- [x] ~~O diretório `arquitetura_de_solucao/` (material das aulas) permanece neste repositório ou sai
      antes da publicação do starter?~~ **Resolvido em 2026-07-29:** sai — ver seção 3, "Material
      das aulas".

## 9. Tarefas (horizontais)

> **Pressupostos adotados** (Fase 3 seguiu sem resposta às questões em aberto): diagramas em
> **C4 com Mermaid**; **SAD único** com decisões embutidas, não pacote separado; esforço-alvo do
> aluno de **8 a 12 horas**. Os dois primeiros viraram decisão na tarefa 9 e estão marcados como
> resolvidos na seção 8. O terceiro segue pressuposto, é o mais frágil, e é o que dimensiona os
> limiares quantitativos da tarefa 11.

### Épico: Fundação

| # | Tarefa | Depende de | Tam. | Critério de aceite |
|---|--------|-----------|------|--------------------|
| 1 | Arrumar o repositório: decidir destino do starter, resolver o working tree (arquivos do projeto anterior deletados e não commitados) e definir onde vive o material das aulas | — | P | Working tree limpo (`git status` sem pendência), destino do starter decidido e registrado no spec, material das aulas em local definido |
| 2 | Esqueleto do starter: módulo Go, layout de pastas, Makefile e `docker-compose.yml` com Redis | 1 | P | `docker compose up` sobe sem erro; `make help` lista os comandos; layout documentado |

### Épico: Serviços do starter

| # | Tarefa | Depende de | Tam. | Critério de aceite |
|---|--------|-----------|------|--------------------|
| 3 | Mock de seguradora parceira, parametrizável por env var (latência, taxa de falha, degradação sob carga) | 2 | M | Um binário serve os 3 perfis via configuração; comportamento determinístico e reprodutível; `partner-slow`, `partner-flaky` e `partner-degrading` sobem no compose |
| 4 | `quotation-api`: contrato HTTP, isolamento multi-tenant por `tenant_id` e agregação síncrona ingênua das 3 parceiras | 2 | M | `POST /quotes` retorna cotações agregadas; requisição sem `tenant_id` é rejeitada; **sem** circuit breaker, **sem** cache — o vácuo proposital está preservado e documentado |
| 5 | Stack de observabilidade pronta para uso: SDK do OTel já inicializado na API, Collector, Jaeger e Prometheus no compose | 2 | M | Trace da requisição aparece no Jaeger sem o aluno escrever uma linha; Prometheus coleta; ao aluno resta criar spans e métricas **de negócio** |

### Épico: Reprodutibilidade da falha

| # | Tarefa | Depende de | Tam. | Critério de aceite |
|---|--------|-----------|------|--------------------|
| 6 | Gerador de carga e roteiro de reprodução do cenário de falha | 3, 4 | P | Um comando único reproduz a degradação; roteiro documentado com o que observar no Jaeger |
| 7 | Smoke test de factibilidade do desafio | 5, 6 | P | Comprovado que, sem circuit breaker, o sistema degrada de forma **visível e reprodutível**; comprovado que os parâmetros default dos mocks permitem que um circuit breaker de fato abra |

### Épico: Enunciado

| # | Tarefa | Depende de | Tam. | Critério de aceite |
|---|--------|-----------|------|--------------------|
| 8 | Contexto de negócio: empresa fictícia, modelo multi-tenant de corretoras, SUSEP e LGPD, custo por chamada a parceiro | — | P | Meia a uma página cobrindo o "Nível 0 — negócio" da aula; nenhuma menção a venda de ingressos |
| 9 | Enunciado da entrega 1 — SAD: as 8 seções, diagramas C4/Mermaid e regra de rastreabilidade anti-alucinação | 8 | M | Cada seção com o que se espera; regra explícita de que citar arquivo inexistente reprova; exigência de números (TCO, projeção de carga) e não de adjetivos |
| 10 | Enunciado da entrega 2 — PoC: circuit breaker de 3 estados, cache com invalidação justificada, fallback, instrumentação e evidências | 8, 4 | M | Deixa claro que a instrumentação é **prova** do comportamento, não exercício à parte; define o formato das evidências de trace e métrica |
| 11 | Bloco isolado de critérios quantitativos e regras de entrega (fork, branch `main`, README do processo) | 9, 10 | P | Todos os números concentrados em um único bloco, ajustável sem reescrever o texto; regras de entrega no padrão dos desafios existentes |
| 12 | README do starter: como subir, como navegar o código e o que está propositalmente ausente | 2, 5, 6 | P | Aluno sobe o ambiente seguindo só o README; o vácuo proposital está explicado, não escondido |

### Paralelismo

- **Bloqueante inicial:** tarefa 1 trava todo o track de código.
- **Dois tracks independentes:** o épico *Enunciado* (8 → 9, 10 → 11) roda em paralelo ao track de
  código, exceto pela tarefa 10, que precisa do contrato HTTP da 4.
- **Dentro do track de código:** 3, 4 e 5 são paralelas entre si depois da 2.
- **Sequencial no fim:** 6 → 7 fecha o track de código; 11 fecha o enunciado.
- **Caminho crítico:** 1 → 2 → (4) → 6 → 7.
