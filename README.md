# Desafio: Fundamentos de Arquitetura de Solução

Circuit breaker, cache e fallback em uma plataforma de cotação cuja disponibilidade é hoje o produto
da disponibilidade de três seguradoras instáveis.

## Descrição

Você recebe uma plataforma de cotação de seguro auto que **funciona e é ruim**: ela consulta três
seguradoras parceiras em série, espera o tempo que elas quiserem, e devolve erro inteiro quando
qualquer uma falha. Tudo o que é preciso para reproduzir essa degradação já está no repositório;
nada do que a resolve está.

A entrega são duas peças que valem uma só: um **SAD** (Solution Architecture Document) que decide e
justifica, e um **PoC** que implementa a fatia decidida e prova, com evidência medida por você, que
ela funciona. O documento vem primeiro, o código é subordinado a ele.

A tensão central não é técnica, é econômica e regulatória. Cada consulta a uma parceira é comprada
por R$ 0,04, o que faz do cache uma decisão financeira antes de ser uma otimização; e cada cotação
carrega CPF e placa sob contrato de operadora na LGPD, o que faz do isolamento entre corretoras uma
obrigação legal antes de ser um cuidado de engenharia. O que se avalia é a decisão defendida com
número, não a quantidade de padrões que você conhece.

## Cenário

A **Prumo Tecnologia em Seguros** é uma empresa fictícia que não vende seguro: vende software para
quem vende. O produto é o **Prumo Cota**, plataforma de cotação de **seguro auto** contratada por
corretoras. Um corretor informa motorista e veículo, o Prumo Cota consulta **três seguradoras
parceiras** e devolve as propostas da mais barata para a mais cara. A Prumo não emite apólice, não
assume risco e não precifica: ela é uma fachada sobre motores de precificação legados que não
controla, com janelas de manutenção próprias e SLAs que as próprias seguradoras descumprem. **A
instabilidade das parceiras é o coração do negócio, não um problema plantado para o exercício.**

A plataforma é **multi-tenant**: cada requisição declara a corretora no cabeçalho `X-Tenant-Id`
(duas no starter, algumas centenas em produção). Cada corretora tem condições comerciais próprias
com cada seguradora, então a mesma placa, para o mesmo motorista, volta com prêmios diferentes
conforme quem pergunta. Uma resposta entregue à corretora errada é erro de preço **e** vazamento de
dado de terceiro.

### Cada consulta custa dinheiro

O contrato com cada seguradora cobra **por consulta ao motor de precificação**, não por venda
fechada:

| | |
|---|---|
| Custo por consulta a uma seguradora | R$ 0,04 |
| Consultas por cotação | 3 (uma por parceira) |
| Cotações por dia útil | 120.000 |
| Preço cobrado da corretora, por cotação entregue | R$ 0,25 |
| Dias úteis no mês | 22 |

São 360.000 consultas por dia útil: **R$ 14.400 por dia**, ou cerca de **R$ 317.000 por mês**,
contra R$ 660.000 de receita. Quase metade da receita bruta sai pela porta da dependência externa, e
boa parte dessas consultas é repetida, porque o corretor recota a mesma placa várias vezes na mesma
conversa e o cliente pede a mesma cotação em mais de uma corretora. Cada acerto de cache é uma
consulta que não foi comprada, e é por isso que o hit rate entra com número na planilha de TCO.

As seguradoras honram o prêmio informado por **até 24 horas**, e o contrato com a corretora exige
que a cotação exibida ainda seja praticável na hora da venda. Esse é o **teto comercial** do
reaproveitamento de uma cotação. Onde parar antes dele é decisão de negócio, e é sua.

> O campo `valid_for_seconds` que as parceiras devolvem vale 300 s por padrão no starter
> (`PARTNER_QUOTE_TTL_SECONDS`, em `cmd/partner-mock/config.go`). Ele é o TTL técnico do mock, não o
> teto comercial de 24 horas. Se você amarrar o TTL do seu cache a esse campo em vez do teto de
> negócio, tudo bem, mas diga isso no SAD e faça a conta de economia com o hit rate que 300 s
> sustenta.

### O que a regulação impõe

**SUSEP.** Toda cotação apresentada a um consumidor precisa ser **auditável** (qual seguradora, qual
prêmio, para qual corretora, em que instante), retida por cinco anos e não regravável. Isso tem
custo de armazenamento, pesa na escolha de onde os dados vivem e vale igualmente para resposta
servida de cache ou de fallback: ela continua rastreável até a consulta que a originou.

**LGPD.** Uma cotação carrega CPF, ano de nascimento e placa. A corretora é **controladora**, a
Prumo é **operadora**, e o contrato entre elas proíbe uso cruzado de dados, exigindo minimização e
prazo de descarte. Uma chave de cache que devolva a uma corretora o que outra cotou não é bug, é
incidente de dados pessoais, com dever de notificação à ANPD.

As duas juntas são o que decide **on-premise, cloud ou híbrido** no seu SAD: retenção, residência e
segregação, não preferência. E disponibilidade, aqui, é promessa comercial: hoje ela é o *produto*
da disponibilidade das três parceiras.

## Objetivo

Ao final, o seu fork tem:

1. Um **SAD** em `docs/sad.md`, com as oito seções e quatro **diagramas C4 em Mermaid** (nível 1,
   nível 2 do "antes", nível 2 do "depois" e nível 3 da fatia implementada).
2. Um **circuit breaker** com os três estados explícitos na fronteira com as parceiras.
3. Um **cache** com chave escrita por extenso, contendo a corretora, e política de invalidação
   declarada.
4. Um **fallback** que substitui o 502 de hoje por um comportamento degradado e visível para quem
   chama.
5. **Três métricas de negócio** e **duas marcações de trace** que provam que os três funcionam.
6. **Dois testes determinísticos** do comportamento resiliente, com a suíte inteira passando.
7. As **evidências** em `docs/evidencias/`, medidas na sua máquina, cada uma com o comando ou a
   consulta que a gerou.
8. Um **README do processo** substituindo este enunciado, como porta de entrada da entrega.

## Repositório base

Repositório: <https://github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros>.
Parta da branch `main`. **Forke** e entregue no seu fork, público, também na `main`.

Você precisa de **Docker com Compose v2** para subir o ambiente e de **Go 1.25** para os alvos que
rodam Go direto na sua máquina (`make test`, `make smoke`, `make build`, `make run`). Como
`make test` e `make smoke` fazem parte da entrega, instale os dois. Não é preciso conta em cloud,
chave de API nem qualquer gasto.

```bash
make up     # docker compose up -d --build
make ps     # lista os oito servicos do ambiente e o estado de cada um
```

Atritos conhecidos do primeiro minuto:

- Na primeira execução as imagens são compiladas. Conte com cerca de um minuto a mais.
- O `make up` não espera os serviços ficarem saudáveis. Rode `make ps` e só então chame a API; quem
  espera por você é o `make reproduce`, que sobe com `--wait`.
- Se `make` não existir na sua máquina, todo alvo do `Makefile` é uma linha de `docker compose` ou
  de `go`, e `make help` lista todos.
- A primeira cotação pode voltar **502** e demora cerca de **1,9 s**. Isso é o desafio, não erro de
  instalação.

| Serviço | Endereço | Para quê |
|---|---|---|
| `quotation-api` | <http://localhost:8080> | a API que você vai proteger |
| Jaeger | <http://localhost:16686> | os traces |
| Prometheus | <http://localhost:9090> | as métricas |
| `partner-slow` · `partner-flaky` · `partner-degrading` | portas 9001 · 9002 · 9003 | as parceiras, direto, sem passar pela API |
| Redis | `localhost:6379` | o cache que ainda não existe |
| OTel Collector | `localhost:4317` (gRPC) e `4318` (HTTP) | destino da telemetria |

`make down` derruba tudo e **zera o cenário**. Nada é persistido, de propósito: o Jaeger guarda
traces em memória, o Prometheus não tem volume e ainda retém só a última hora, o Redis sobe sem AOF
nem RDB, e o contador de sequência das parceiras vive no processo. É isso que faz o "antes" e o
"depois" serem comparáveis, e é por isso que evidência de gráfico se captura **durante** a execução.

## O que já existe (e você vai estender)

### O contrato

| Rota | O que faz |
|---|---|
| `POST /quotes` | cotação agregada das três parceiras; exige o cabeçalho `X-Tenant-Id` |
| `GET /healthz` | saúde do processo |

Repare em quem vai onde: a **corretora viaja no cabeçalho**, o CPF e a placa viajam no corpo. O
inquilino é contexto da chamada, não dado de seguro, e é essa separação que a sua chave de cache
vai ter que respeitar.

### O mapa do código

São **menos de 800 linhas de Go** entre a API e o `internal/` (sem contar testes). Dá para ler tudo
em vinte minutos, e vale a pena antes de decidir qualquer coisa.

| Caminho | O que é | Você mexe aqui? |
|---|---|---|
| `cmd/quotation-api/main.go` | boot da API: configuração, telemetria, rotas | provavelmente, para ligar o que você construir |
| `internal/quotation/handler.go` | contrato HTTP e validação do inquilino | talvez, se o fallback mudar a resposta |
| `internal/quotation/service.go` | agregação em série, tudo ou nada | **sim**, é onde cache e fallback aparecem |
| `internal/quotation/request.go` | corpo do pedido e da resposta | talvez, pelo mesmo motivo |
| `internal/partner/client.go` | a fronteira com a parceira, sem proteção nenhuma | **sim**, é onde o circuit breaker nasce |
| `internal/platform/telemetry.go` | SDK do OTel já inicializado | quase nunca: as suas métricas de negócio são código novo |
| `internal/platform/config.go` | leitura de variáveis de ambiente | sim, se você acrescentar parâmetros (TTL, limiares) |
| `cmd/partner-mock/` | as três parceiras, um binário parametrizado | **não**: é restrição do cenário |
| `cmd/loadgen/` | o gerador de carga | não |
| `deploy/otel/` · `deploy/prometheus/` | Collector e Prometheus | só se você mudar o destino da telemetria |
| `docker-compose.yml` · `Makefile` · `Dockerfile` | o ambiente e os comandos | sim, com moderação |

Dois arquivos merecem leitura comparada, porque a diferença entre eles é o desafio inteiro:
`internal/partner/client.go`, onde a chamada sai sem nenhuma proteção, e
`cmd/partner-mock/feasibility_test.go`, que testa comportamento resiliente de forma determinística.
O segundo é o molde de como você vai testar o primeiro.

O starter entra **verde**: `make test` passa inteiro, sem teste pulado e sem teste quebrado. Se
alguma coisa ficar vermelha depois, foi você quem mexeu, e isso conta contra a entrega.

### As três parceiras: a instabilidade é determinística

| Parceira | Porta | Comportamento |
|---|---|---|
| `partner-slow` | 9001 | 1500 ms ± 200 ms, sempre. Nunca falha, só é lenta demais para uma agregação em série |
| `partner-flaky` | 9002 | 150 ms ± 50 ms, com 40% das chamadas devolvendo 503 |
| `partner-degrading` | 9003 | 120 ms ± 40 ms em repouso; acima de 5 chamadas simultâneas soma 300 ms por chamada extra, até o teto de 6000 ms. Nunca falha: afunda |

A instabilidade **não é sorteada a cada execução**. Ela vem da semente `20260729`, então a sequência
de falhas da `partner-flaky` é a mesma na sua máquina e na de quem corrige, inclusive a rajada de
**nove falhas consecutivas nas sequências 49 a 57**, que é o que garante que um circuit breaker de
fato abre. Isso não é estimativa: está medido em `docs/smoke-test-factibilidade.md` e preso por
teste em `cmd/partner-mock/feasibility_test.go`.

Toda resposta das parceiras traz os cabeçalhos `X-Partner-Name`, `X-Partner-Seq`,
`X-Partner-Latency-Ms` e `X-Partner-Inflight`, e `curl -s localhost:9003/config` mostra a
configuração efetiva de uma delas.

### O que está ausente de propósito

Quatro buracos, e nenhum deles está escondido. Todos estão comentados no próprio código:

1. **Sem timeout** (`internal/partner/client.go`): o `http.Client` tem `Timeout` zero. A API espera
   o tempo que a parceira quiser.
2. **Agregação em série** (`internal/quotation/service.go`): as parceiras são consultadas uma após a
   outra, e a latência da cotação é a **soma** das três.
3. **Tudo ou nada** (a mesma `service.go`): uma parceira falhando aborta a requisição inteira, mesmo
   que as outras duas já tenham respondido.
4. **Sem cache e sem instrumentação de negócio**: o Redis sobe e ninguém fala com ele; o SDK do
   OTel sobe e emite só telemetria genérica (HTTP de entrada, HTTP de saída, runtime do Go).

Isso é o enunciado, não um esquecimento. Não abra issue nem PR no repositório de origem "corrigindo"
o starter: preencher esses quatro buracos, com justificativa e com prova, **é a sua entrega**.

### Reproduza a falha antes de decidir qualquer coisa

```bash
make reproduce
```

Um comando: sobe o ambiente, espera ficar saudável e dispara a carga (10 requisições sequenciais de
baseline, depois 200 com 50 em voo). O relatório sai no terminal.

O roteiro **`docs/roteiro-cenario-de-falha.md`** ensina a ler esse relatório e a achar no Jaeger a
cascata em série que o explica: três chamadas coladas uma na outra, e um trace com erro em que o
span da terceira parceira nem chega a existir. Leia antes de escrever código, porque ele é o "antes"
da sua entrega.

### Os comandos

| Comando | O que faz |
|---|---|
| `make up` · `make down` · `make ps` · `make logs` | ciclo de vida do ambiente (`down` remove os volumes e zera o cenário) |
| `make reproduce` | sobe tudo, espera ficar saudável e reproduz a degradação de ponta a ponta |
| `make load ARGS="-concurrency 100"` | roda só a carga, no ambiente já de pé (`ARGS="-h"` lista as opções) |
| `make smoke` | prova determinística de que um breaker abre com os defaults; precisa de Go, não precisa de Docker |
| `make test` · `make fmt` · `make vet` · `make tidy` | o dia a dia em Go |
| `make run` · `make build` | rodar ou compilar a API fora do container |
| `make help` | lista tudo isso |

## Tecnologias obrigatórias

- **Go 1.25** para a API e para os testes. A linguagem do starter não muda.
- **Docker com Compose v2** para o ambiente. O `docker-compose.yml` do starter é o ponto de partida.
- **Redis** como cache. Ele já sobe no compose e a API ainda não fala com ele.
- **OpenTelemetry** para a instrumentação, com o SDK que já está inicializado em
  `internal/platform/telemetry.go`. Traces vão para o Jaeger e métricas para o Prometheus, ambos
  via Collector.
- **Mermaid** para todo diagrama, dentro do próprio Markdown. Imagem colada de diagrama não vale.
- A biblioteca de circuit breaker é **decisão sua**, e a escolha se defende no SAD.

Proibido: alterar `cmd/partner-mock/` e alterar os perfis das parceiras no compose
(`PARTNER_SEED`, `PARTNER_FAILURE_RATE`, latências, degradação) para tornar o desafio mais fácil.

## Regras de negócio e semântica de erros

O comportamento de hoje está em `internal/quotation/handler.go` e em `internal/quotation/request.go`.
As mensagens abaixo são as do starter, e são a fonte única de verdade: se a sua entrega falar de um
erro, é por este nome.

| Situação | Status | Corpo |
|---|---|---|
| Sem o cabeçalho `X-Tenant-Id` | 400 | `{"error":"X-Tenant-Id is required"}` |
| Corretora fora de `TENANTS` (o compose define `corretora-a,corretora-b`) | 403 | `{"error":"broker not enabled on this platform"}` |
| JSON inválido ou com campo desconhecido | 400 | `{"error":"invalid body: ..."}` |
| Campo obrigatório ausente ou inválido | 400 | `{"error":"driver.document is required"}` e as demais de `Normalize` |
| Cobertura fora da lista | 400 | `{"error":"coverage must be comprehensive or third_party"}` |
| Qualquer parceira falhando | 502 | `{"error":"partner insurer unavailable","partner":"partner-flaky"}` |
| Sucesso | 200 | as três cotações ordenadas da mais barata para a mais cara |

Outras regras que valem para a sua entrega:

- Coberturas aceitas: `comprehensive` (padrão, quando o campo vem vazio) e `third_party`.
- A ordenação da resposta é por prêmio crescente, e continua sendo, mesmo em resposta degradada.
- O `X-Tenant-Id` volta no cabeçalho da resposta de sucesso.
- Resposta servida de cache ou de fallback continua sendo cotação apresentada a um consumidor, e
  portanto continua auditável nos termos da SUSEP.
- **Inventar prêmio é proibido.** Um preço fabricado apresentado ao consumidor é problema
  regulatório, não bug de aplicação.

## Requisitos: entrega 1, o SAD

A primeira entrega é o **Solution Architecture Document** do Prumo Cota depois da sua intervenção.
Ele descreve o sistema que você vai construir na entrega 2 e, mais que descrever, **justifica**. O
PoC é subordinado ao SAD: você implementa a fatia que o seu próprio documento defendeu.

Escreva para três leitores que existem de verdade e que não leem template: o **CTO da Prumo**, que
aprova (ou não) o gasto e quer ver a conta; quem vai **operar** a plataforma às 3h da manhã e
precisa saber o que fazer quando uma parceira afundar; e o **encarregado de dados e auditoria**, que
precisa provar à ANPD e à SUSEP que a cotação servida de cache continua rastreável e que uma
corretora nunca vê o dado de outra. Se uma frase do seu SAD não serve a nenhum dos três, ela é
enchimento.

### Três regras que valem para o documento inteiro

**Regra 1, rastreabilidade. Citar arquivo que não existe reprova.** Toda afirmação sobre o sistema
aponta para algo verificável: um caminho real do repositório, um trecho deste enunciado, ou uma
medição que você fez e colou. A correção é estática, e quem corrige **abre os caminhos citados**. Um
`internal/resilience/breaker.go` que só existe no documento é reprovação imediata, e o mesmo vale
para biblioteca inventada, métrica que ninguém emite e endpoint que o código não serve. Escrever
sobre o que ainda não existe é esperado, desde que marcado como proposta; o que reprova é apresentar
ficção como fato.

**Regra 2, números, não adjetivos.** "Escalável", "robusto", "alta disponibilidade" e "performático"
são opiniões. Um requisito não funcional precisa de **métrica, valor, unidade e método de medição**,
como em "p95 do `POST /quotes` abaixo de 800 ms, medido no histograma
`http_server_request_duration_seconds` do Prometheus". Sem os quatro, não conta. O mesmo vale para
dinheiro: a seção 8 é uma planilha, não um parágrafo dizendo que o cache "reduz custos".

**Regra 3, diagrama é código, em C4 com Mermaid.** Todo diagrama vai em bloco Mermaid dentro do
Markdown, porque texto dá `diff`, aparece na revisão e não desatualiza em silêncio. Níveis
obrigatórios: **1 (contexto)** e **2 (contêiner)** na seção 2 do SAD, **3 (componente)** na seção 4,
só da fatia que você implementa. Nível 4 não é pedido. `C4Context` e `C4Container` são nativos e
ainda experimentais; um `flowchart` também serve, desde que pessoa, sistema, contêiner e sistema
externo fiquem distinguíveis e **toda relação seja rotulada com o que trafega e por qual
tecnologia**. Seta sem rótulo não é diagrama de arquitetura.

```mermaid
flowchart LR
  broker(["Corretora<br/><i>pessoa</i>"])
  api["quotation-api<br/><i>contêiner · Go</i>"]
  slow["partner-slow<br/><i>sistema externo</i>"]

  broker -- "cota um risco<br/>HTTP/JSON · X-Tenant-Id" --> api
  api -- "consulta prêmio<br/>HTTP/JSON" --> slow
```

### As oito seções

O que cada uma precisa conter, com os contra-exemplos que costumam aparecer no lugar do conteúdo,
está em **[`docs/guia-sad.md`](docs/guia-sad.md)**. Leia antes de escrever. Em resumo:

1. **Introdução:** propósito, escopo, restrições separadas das decisões, e pressupostos com valor,
   origem e consequência se forem falsos.
2. **Visão geral:** o antes e o depois em C4 nível 1 e 2, mais o delta em lista.
3. **Requisitos funcionais e não funcionais**, com o "hoje" medido por você.
4. **Detalhamento:** as decisões de breaker, cache e fallback, com todo parâmetro numérico
   justificado e a chave de cache escrita por extenso, mais o C4 nível 3 da fatia implementada.
5. **Implementação:** como se constrói (mecanismo para arquivo real) e onde roda (on-premise, cloud
   ou híbrido, decidido pelo compliance).
6. **Operação e gestão de mudanças:** o que se olha, com que limiar, qual ação, mais um runbook e o
   caminho de mudança do TTL.
7. **Recuperação de desastres:** RTO e RPO por classe de dado, e três cenários.
8. **Tecnologias, custos e pessoal:** a conta de parceiro por hit rate, a infraestrutura, o pessoal
   e o veredito de payback.

O que separa um SAD de um template preenchido é verificável de fora: **cada seção referencia
outra**. O requisito da seção 3 aparece como mecanismo na 4, como arquivo na 5, como alerta na 6,
como cenário de desastre na 7 e como reais na 8. Um documento em que as oito poderiam ser lidas em
qualquer ordem, sem que nada quebrasse, é oito documentos curtos, não um SAD.

**De 10 a 20 páginas equivalentes** é a faixa esperada, como orientação e não como regra: abaixo
dela é provável que alguma seção tenha ficado sem conteúdo; acima, releia procurando enchimento.

## Requisitos: entrega 2, o PoC

A segunda entrega é código: a fatia do seu SAD construída sobre o starter e **provada em execução**.
Ela é pequena de propósito. Não é aqui que você mostra fôlego de desenvolvedor, é aqui que você
mostra que a arquitetura que defendeu sobrevive ao contato com a `partner-flaky`. O vácuo que você
preenche está identificado no código: `internal/partner/client.go` (cliente sem timeout, sem
proteção, sem fallback) e `internal/quotation/service.go` (agregação em série, tudo ou nada). O
Redis já sobe no compose e a API ainda **não fala com ele**: essa ligação é sua.

A subordinação ao SAD vale nos dois sentidos. O documento **pode** propor mais do que o PoC
implementa, desde que diga qual fatia foi implementada e qual ficou como proposta. O contrário não
pode: um limiar de breaker ou um TTL que aparece no código sem estar defendido no SAD é um número
chutado, mesmo que funcione. Se você descobrir implementando que a decisão estava errada, **volte e
corrija o SAD**. Divergência entre os dois é o que perde ponto; mudar de ideia com evidência na mão
é o que se espera de um arquiteto.

### 1. Circuit breaker: os três estados têm que ser visíveis de fora

Fechado, aberto e meio aberto são três comportamentos diferentes, e a sua entrega precisa mostrar os
três acontecendo. O que costuma passar batido é o significado operacional do estado aberto:
**aberto quer dizer que a parceira não é chamada**. Se a requisição sai e o resultado é descartado,
você não economizou o R$ 0,04 da consulta nem o tempo de espera.

Decida e defenda o **escopo**: um breaker por parceira, um por par (parceira, corretora), ou um só
para todas. Um breaker global derruba a plataforma inteira porque uma das três afundou; pode ser
defendido, mas dificilmente é o que você quer num agregador de fornecedores independentes. Os
parâmetros vêm do SAD: quantas falhas em qual janela abrem, quanto tempo o circuito fica aberto,
quantas requisições o meio aberto deixa passar, o que fecha e o que reabre. E o comportamento se
testa **sem depender de sorte**: a rajada de nove falhas nas sequências 49 a 57 da `partner-flaky`
diz de antemão onde o breaker deve abrir.

- **Não conta:** biblioteca importada e configurada sem uma única evidência de transição de estado.
  É a versão em código da frase "usaremos circuit breaker".

### 2. Cache: a chave é o contrato de isolamento

Escreva a **chave por extenso**, no SAD e no README do processo. Ela precisa conter, no mínimo, a
corretora e a identificação normalizada do risco cotado. Se o seu cache é por parceira ou pela
cotação agregada é decisão sua, com consequência sua: cache por parceira sobrevive a uma parceira
fora, cache agregado economiza mais e vence inteiro de uma vez.

> **Chave de cache sem `tenant_id` reprova.** Não é rigor de estilo: é a mesma placa devolvendo à
> corretora A o prêmio negociado pela corretora B, erro de preço e incidente de dados pessoais, com
> dever de notificação à ANPD. É o único defeito isolado do PoC que reprova sozinho.

O **TTL é decisão de negócio** e tem o teto comercial de 24 horas do Cenário; maior que isso não é
cache agressivo, é cotação que ninguém honra. Parar antes é o esperado, mas diga por quê, e o número
que você escolher é o mesmo que sustenta o hit rate da planilha da seção 8. E TTL não é a política
inteira: declare também o que acontece com a entrada quando o breaker abre (a cotação vencida ainda
é servida? por quanto tempo? a corretora fica sabendo?), o que invalida uma entrada antes da hora, e
como as entradas somem quando têm que sumir, porque o Redis do starter é efêmero mas prazo de
descarte de dado pessoal é obrigação da LGPD, não configuração de container.

- **Reprova:** chave sem isolamento por corretora.

### 3. Fallback: o que a corretora recebe quando não há resposta

Hoje a cotação morre: uma parceira falha e a requisição inteira vira 502, jogando fora as respostas
que já tinham chegado (`internal/quotation/service.go`). Decidir o que colocar no lugar disso é a
parte mais próxima do negócio de todo o PoC. Três saídas são legítimas, e você escolhe uma e
defende:

- **resposta parcial**, entregando as parceiras que responderam e dizendo explicitamente qual
  faltou;
- **cotação anterior**, servida do cache, marcada como tal, com a idade dela na resposta;
- **recusa explícita**, se o produto não admite proposta incompleta: um erro de negócio claro,
  distinguível de um erro genérico, com o que a corretora deve fazer.

Em qualquer das três, a corretora tem que saber que aquilo é degradado, o que significa que o
contrato de resposta muda (`Response`, em `internal/quotation/request.go`), e a mudança tem que
estar documentada. Prêmio inventado continua proibido.

- **Não conta:** log dizendo "fallback acionado" enquanto o cliente continua recebendo 502.

### 4. Instrumentação: é o seu método de prova, não um segundo exercício

Não se pede observabilidade aqui para você exercitar OpenTelemetry, e sim porque **nenhuma afirmação
desta entrega vale sem o dado que a sustenta**: "o breaker abre" se prova com a série temporal do
estado, não com o parágrafo dizendo que abre. Por isso o starter já entrega o SDK, o Collector, o
Jaeger e o Prometheus de pé (`internal/platform/telemetry.go`): o plumbing não é o exercício, e três
horas gastas nele são três horas roubadas do que está sendo avaliado. O que já vem pronto é genérico
(HTTP de entrada, HTTP de saída, runtime do Go); o que falta é o de negócio, e são três coisas:

1. **Transição de estado do breaker**, por parceira, com origem e destino. Você precisa conseguir
   desenhar o degrau fechado → aberto → meio aberto → fechado no tempo, com um valor observável do
   estado atual, um contador de transições, ou os dois. E a requisição curto-circuitada tem que
   dizer isso no trace, senão ela aparece como uma cotação misteriosamente rápida.
2. **`hit` e `miss` do cache**, com atributo que permita calcular o hit rate em PromQL. Este é o
   número que alimenta a planilha da seção 8; sem ele, a economia que você projeta é chute.
3. **Latência por parceira**, com significado de domínio. Hoje ela existe só pela borda HTTP
   (`http_client_request_duration_seconds`, atributo `server_address`). Reaproveitar essa métrica é
   legítimo, desde que você diga que reaproveitou e mostre a consulta.

**Atributo é dado retido.** `tenant_id` é útil e aceitável; **CPF, placa e `quote_id` não entram em
span nem em métrica**, porque além de dado pessoal fora de lugar, placa em rótulo de métrica é
cardinalidade sem teto. E instrumente pouco: span por função e métrica por variável é ruído com
custo de retenção, e a seção 6 do seu SAD vai ter que explicar quem olha aquilo.

- **Não conta:** métrica citada na documentação que o código não emite. É a regra 1 aplicada ao
  código: quem corrige procura o nome no repositório.

### 5. Evidências: o formato

Uma evidência tem três partes: **o comando ou a consulta que a gerou**, **o artefato** e **uma
legenda de uma linha dizendo o que se vê nele**. Faltando qualquer uma, é figura decorativa. Tudo
mora em `docs/evidencias/` e é referenciado do README do processo.

**O "antes" é seu.** Rode `make down && make reproduce` na sua máquina *antes* de tocar no código.
Os números do roteiro foram medidos em outra máquina, e copiá-los é apresentar ficção como fato. As
latências vão diferir, e tudo bem: o que se compara é o seu antes com o seu depois. Nas duas
execuções, use a **carga padrão do `make reproduce`** (10 de baseline, depois 200 com 50 em voo);
comparar cargas diferentes não compara nada.

| Evidência | Formato | O que ela tem que mostrar |
|---|---|---|
| Relatório antes/depois | **texto colado**, saída completa do `make reproduce`, mesma carga nas duas | a diferença de p95, de taxa de sucesso e de vazão |
| Saída do `make test` | **texto colado** | os testes do starter e os seus, todos passando |
| Trace com breaker aberto | screenshot ou export JSON do Jaeger | a requisição que **não chamou** a parceira curto-circuitada, e respondeu rápido |
| Trace servido de cache | screenshot ou export JSON do Jaeger | a cotação sem os spans de saída para as parceiras |
| Estado do breaker no tempo | gráfico **com a consulta PromQL colada como texto** | o degrau fechado → aberto → meio aberto → fechado |
| Hit rate do cache | gráfico **com a consulta** | a curva subindo conforme o cache aquece |
| p95 do `POST /quotes` | gráfico **com a consulta**, um por execução | o valor no "antes" e o no "depois", na mesma escala e com a janela declarada |

Três regras de formato decidem se a evidência é verificável:

1. **Relatório vai como texto, não como imagem**, porque texto dá `diff`, dá busca e cabe na revisão.
2. **Gráfico sem a consulta ao lado não vale**, e toda evidência declara a **janela de tempo**
   (`Last Hour` no Jaeger, `[5m]` no PromQL). O gráfico é a afirmação, a consulta é a fonte.
3. **Capture o gráfico durante a execução.** O Prometheus não tem volume e retém uma hora, e o
   `make down` zera Jaeger e Prometheus. Por isso o p95 do "antes" e o do "depois" são duas
   capturas, uma em cada sessão; a comparação numérica entre eles é o relatório de texto.

Formato dos arquivos: relatórios em texto; imagens em PNG ou JPG legíveis em tamanho real; export de
trace do Jaeger em JSON, melhor que screenshot e menor. Nada de PDF com print dentro, e imagem acima
de 5 MB é sinal de que você exportou a tela errada.

- **Não conta:** screenshot de dashboard sem a consulta; evidência que mostra a métrica existindo
  mas nunca mudando de valor. O que se pede é a **transição**, não a existência.
- **Reprova:** "antes" copiado do roteiro ou de outro aluno.

### 6. Testes

`make test` tem que passar, incluindo os testes que já vieram. Quebrar o starter para o seu código
caber é regressão, não refatoração, e como a correção é estática, é a saída colada em
`docs/evidencias/` que prova que passou. O comportamento resiliente se testa como
`cmd/partner-mock/feasibility_test.go` ensina: determinístico, sem `sleep` e sem esperar que a sorte
coopere. Rodar uma variação declarada dos perfis das parceiras para explorar o limiar é legítimo,
desde que a evidência que sustenta a sua entrega venha dos defaults e que `make smoke` siga verde.

- **Não conta:** teste que passa por `sleep` calibrado na máquina de quem escreveu.

## Restrições (não negociáveis)

- **Não altere `cmd/partner-mock/`.** As três parceiras são a restrição do cenário, não parte da
  solução.
- **Não altere os perfis das parceiras no compose** (`PARTNER_SEED`, `PARTNER_FAILURE_RATE`,
  latências, parâmetros de degradação) na configuração que sustenta a sua evidência.
- **Não quebre o que já passa.** Os testes do starter continuam verdes.
- **Não reescreva o starter.** A entrega é a extensão dele, não um projeto novo com o mesmo nome.
- **A chave de cache contém a corretora.** Sem exceção.
- **Prêmio é sempre de parceira ou de cache rastreável.** Nada de valor calculado pela plataforma.
- Se você encontrar uma limitação real no starter que te impeça de cumprir um requisito, documente
  no README do processo em vez de reescrever o mecanismo. Limitação documentada é decisão de
  arquitetura; limitação contornada em silêncio é entrega que não se consegue avaliar.

## Fora de escopo

- Retry com backoff, bulkhead, rate limiting, fila, autenticação e Kubernetes. Cite no SAD o que
  fizer sentido citar, mas **não implemente**. Se você está escrevendo o quarto pattern, você saiu
  da matéria.
- Persistência de auditoria de verdade. A auditoria é decisão de arquitetura no SAD e custo na
  seção 8, não uma tabela para você criar.
- Garantia transacional entre cache e parceira, e qualquer discussão de concorrência além da que a
  paralelização da agregação exigir.
- Integração real com qualquer seguradora. As três parceiras são e continuam mocks.
- Corrigir bugs pré-existentes do starter que não sejam os quatro buracos declarados.
- Interface de usuário, cadastro de corretoras, cobrança, relatório para o corretor.
- Reescrever a telemetria genérica que já vem pronta.

## Contratos sugeridos

A cotação de hoje, que continua funcionando depois da sua entrega:

```bash
curl -s localhost:8080/quotes \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-Id: corretora-a' \
  -d '{"driver":{"document":"12345678901","birth_year":1985},
       "vehicle":{"plate":"ABC1D23","model":"Onix 1.0","year":2020,"value_cents":7500000},
       "coverage":"comprehensive"}'
```

A resposta de sucesso, como o starter a devolve hoje (`internal/quotation/request.go`):

```json
{
  "tenant_id": "corretora-a",
  "quotes": [
    {
      "partner": "partner-flaky",
      "quote_id": "partner-flaky-3f2a1c0b9d8e7f60",
      "premium_cents": 84210,
      "currency": "BRL",
      "coverage_cents": 5000000,
      "valid_for_seconds": 300
    }
  ],
  "elapsed_ms": 1873
}
```

O exemplo mostra **uma** entrada de `quotes` para caber na página; a resposta real traz as três,
ordenadas por `premium_cents` crescente. Os valores são ilustrativos: o prêmio é gerado por hash em
`cmd/partner-mock/behavior.go`, entre R$ 500,00 e R$ 2.000,00, e o `valid_for_seconds` sai de
`PARTNER_QUOTE_TTL_SECONDS`, que vale 300 no starter.

O erro de parceira, hoje:

```json
{ "error": "partner insurer unavailable", "partner": "partner-flaky" }
```

Como a resposta degradada se distingue da normal é **decisão sua**, e é isso que o fallback muda no
contrato. Um campo booleano, um bloco de metadados, um cabeçalho: escolha, documente no README do
processo e justifique no SAD. O que não é decisão sua é o fato de a corretora ter que conseguir
saber, lendo a resposta, que aquela cotação é degradada.

## Critérios de aceite

Todos obrigatórios. Os números estão dimensionados para um esforço de **8 a 12 horas** e são
**mínimos, não alvos**: bater o mínimo em tudo é uma entrega aprovável, não é uma entrega boa.

### SAD

- ☐ 6 requisitos funcionais, com identificador e frase testável, sendo ao menos 2 criados pela sua arquitetura (seção 3 do SAD)
- ☐ 5 requisitos não funcionais, com métrica, valor, unidade, método e origem do alvo, sendo ao menos 3 com a coluna "hoje" medida por você (seção 3 do SAD)
- ☐ 3 pressupostos com valor, origem e consequência se forem falsos (seção 1 do SAD)
- ☐ 4 diagramas C4 em Mermaid: nível 1, nível 2 do "antes", nível 2 do "depois" e nível 3 da fatia implementada (seções 2 e 4 do SAD)
- ☐ 4 decisões no formato contexto → opções → escolha → consequências, cada uma com ao menos uma alternativa descartada e as consequências ruins: circuit breaker, cache, fallback e hospedagem (seções 4 e 5 do SAD)
- ☐ 3 alertas com métrica, limiar e ação (seção 6 do SAD)
- ☐ 1 runbook completo, do sintoma ao escalonamento (seção 6 do SAD)
- ☐ 3 cenários de desastre, com efeito no cliente, custo do modo degradado e caminho de volta: Redis perdido, parceira fora por seis horas, perda do site ou da região (seção 7 do SAD)
- ☐ 3 linhas na conta de parceiro: hoje, com hit rate zero, e ao menos 2 hit rates que o seu TTL sustente (seção 8 do SAD)
- ☐ custo de auditoria projetado para o ano 1 e para o ano 5 (seção 8 do SAD)

### PoC

- ☐ circuit breaker com os três estados nomeados no código, escopo declarado, e o estado aberto sem chamar a parceira
- ☐ cache com a chave escrita por extenso, contendo a corretora, e política de invalidação declarada
- ☐ fallback implementado e refletido no contrato de resposta, com a degradação visível para quem chama
- ☐ 3 métricas de negócio, com o nome declarado no README do processo: estado ou transição do breaker, `hit` e `miss` do cache, latência por parceira
- ☐ 2 marcações no trace: a requisição curto-circuitada pelo breaker e a resposta servida de cache
- ☐ 2 testes determinísticos do comportamento resiliente, um do breaker abrindo e um do cache ou do fallback
- ☐ nenhum CPF, placa ou `quote_id` em atributo de span ou de métrica

### Evidências

- ☐ as 7 linhas da tabela de evidências, cada uma com o comando ou a consulta que a gerou e uma legenda de uma linha
- ☐ relatório antes/depois em texto, das duas execuções feitas por você, com a carga padrão do `make reproduce`
- ☐ toda evidência de gráfico acompanhada da consulta PromQL como texto e da janela de tempo
- ☐ saída do `make test` colada, com a suíte inteira passando

### Coerência e repositório

- ☐ todo caminho, biblioteca, métrica e endpoint citado no SAD existe no repositório, ou está marcado como proposta
- ☐ o TTL, os limiares do breaker e os nomes de métrica são os mesmos no SAD e no código
- ☐ o hit rate da seção 8 do SAD é compatível com o TTL da seção 4 e com o pressuposto de recotação
- ☐ README do processo com as cinco coisas exigidas, na raiz do fork, na branch `main`
- ☐ `docs/enunciado.md`, `docs/sad.md` e `docs/evidencias/` no lugar

## Fluxo do avaliador

A correção é **estática**: quem corrige **não roda** a sua aplicação. O que só existe em execução
não conta como entregue. Estes são os passos, na ordem, e vale a pena você percorrê-los antes do
push final:

1. Clona o fork e confirma que a entrega está na `main`.
2. Abre o `README.md` e procura as cinco coisas: link para o SAD e para as evidências, como subir e
   reproduzir, o que foi implementado e o que ficou como proposta, os nomes das métricas e dos
   atributos criados, e o que você faria diferente com mais tempo.
3. Abre `docs/sad.md` e confere as oito seções contra os critérios de aceite do SAD, contando
   requisitos, pressupostos, diagramas, decisões, alertas, cenários e linhas de custo.
4. **Abre cada caminho citado no SAD.** Qualquer arquivo, biblioteca, métrica ou endpoint que não
   exista e não esteja marcado como proposta reprova a entrega (regra 1).
5. Procura no repositório os nomes de métrica e de atributo declarados no README do processo. Se o
   código não emite, é ficção apresentada como fato.
6. Abre `internal/partner/client.go` e `internal/quotation/service.go` e procura os três mecanismos:
   os três estados do breaker nomeados, o caminho em que o estado aberto **não** chama a parceira, a
   leitura e a escrita no cache, e o comportamento de fallback.
7. Procura o `tenant_id` na construção da chave de cache. Sem ele, reprova, sem compensação.
8. Abre `docs/evidencias/` e confere as sete linhas: cada uma com comando ou consulta, artefato
   legível e legenda; o relatório antes/depois em texto e vindo da mesma carga nas duas execuções.
9. Compara os números: o TTL e os limiares do SAD são os mesmos do código, e o hit rate da planilha
   é compatível com esse TTL.
10. Confere que os defaults das parceiras no compose continuam os do starter.

A frase que resume tudo: **ler o SAD e abrir o código não pode produzir dois sistemas diferentes.**
O mesmo TTL, o mesmo limiar e o mesmo nome de métrica têm que aparecer nos dois. Se qualquer um
divergir, ou se qualquer caminho citado não abrir, a entrega está incompleta.

## Estrutura obrigatória do entregável

```
README.md                        # o README do processo (substitui este enunciado)
docs/enunciado.md                # este enunciado, preservado para as citacoes da regra 1
docs/sad.md                      # o SAD completo, as oito secoes
docs/evidencias/                 # as evidencias, referenciadas pelo README
docs/guia-sad.md                 # do starter (nao alterar)
docs/roteiro-cenario-de-falha.md # do starter (nao alterar)
docs/smoke-test-factibilidade.md # do starter (nao alterar)
cmd/partner-mock/                # do starter (nao alterar)
cmd/loadgen/                     # do starter (nao alterar)
cmd/quotation-api/               # voce estende
internal/                        # voce estende
docker-compose.yml               # voce estende, com moderacao
```

Arquivos extras legítimos são bem-vindos e ficam nas pastas do próprio starter: pacote novo em
`internal/`, teste novo junto do código que ele testa, documento de apoio em `docs/`.

## Entregável e regras de entrega

1. **Forke este repositório.** A entrega vive no seu fork, público.
2. **Entregue na branch `main`.** O que estiver em outra branch não é lido.
3. **O `README.md` deixa de ser este enunciado** e passa a ser o **README do processo**, a porta de
   entrada da sua entrega. Mova este texto para `docs/enunciado.md`, porque a regra 1 permite citar
   trechos dele e quem corrige precisa conseguir abrir o que você citou.
4. **Entregas que reescrevem a base não são aceitas.** O starter é o ponto de partida obrigatório.
5. **O veredito é binário:** aprovado ou não aprovado.

O README do processo é curto e tem cinco coisas: link para o SAD e para as evidências; como subir o
ambiente e reproduzir a sua versão; **o que foi implementado e o que ficou como proposta**; os nomes
das métricas e dos atributos que você criou; e o que você faria diferente com mais tempo. Reserve um
parágrafo para descrever, em prosa, o coração da sua solução: o que você protegeu, com o quê, e o
que isso custou.

## Bônus (opcional)

Feche o obrigatório antes de olhar para cá. Estes dois acréscimos não contam para nenhum critério
de aceite, e a ausência deles não tira nada da entrega:

- ☐ (opcional) **timeout** na chamada à parceira, com o valor defendido no SAD
- ☐ (opcional) **paralelização da agregação**, sobrepondo as três parceiras em vez de somá-las

Os dois mudam os seus números de p95, então, se implementar, trate-os como qualquer outra decisão:
contexto, opções, escolha e consequências no SAD.

## Ordem de execução sugerida

**1.** Suba o ambiente e leia o código inteiro. São menos de 800 linhas.

**2.** Rode `make down && make reproduce` e guarde a saída: esse é o seu "antes", e ele deixa de
existir assim que você mexer no código.

**3.** Ache a cascata em série no Jaeger, seguindo `docs/roteiro-cenario-de-falha.md`.

**4.** Leia `docs/guia-sad.md` e escreva as seções 1 a 4 do SAD. É o documento que fixa TTL,
limiares e escopo do breaker.

**5.** Implemente os três mecanismos, um de cada vez, com o teste determinístico junto.

**6.** Instrumente as três métricas e as duas marcações de trace, e confirme no Prometheus e no
Jaeger que elas aparecem.

**7.** Rode a carga de novo, com a mesma configuração, e colete as evidências com o ambiente de pé.

**8.** Feche as seções 5 a 8 do SAD com os números medidos, corrija o que a implementação provou
estar errado, escreva o README do processo, mova este enunciado para `docs/enunciado.md` e rode o
Fluxo do avaliador do zero no seu próprio fork.

## Dicas finais

O erro mais caro deste desafio é sutil: um breaker que muda de estado, emite métrica bonita e
continua chamando a parceira. Aberto quer dizer que a chamada não sai, e a maneira de descobrir isso
em segundos é olhar o trace de uma requisição feita com o circuito aberto e procurar o span de
saída; se ele está lá, o seu breaker é um contador de falhas com nome bonito, e a economia que você
projetou na planilha não existe. O segundo tropeço é a chave de cache, que quase sempre nasce a
partir do corpo da requisição, e o corpo não tem a corretora, porque ela viaja no cabeçalho. É por
isso que esse defeito reprova sozinho: é fácil de cometer e invisível em teste com um inquilino só.
Escreva a chave por extenso antes de escrever a função que a monta.

Os seus instrumentos de depuração estão prontos e são mais rápidos que a UI para as primeiras
perguntas: `X-Partner-Seq` e `X-Partner-Inflight` dizem em que ponto da sequência determinística
você está e quantas chamadas estão em voo; `curl -s localhost:9003/config` mostra a configuração
efetiva de uma parceira; e `make smoke` responde, sem Docker e em segundos, se um breaker com os
seus parâmetros abriria com os defaults do cenário. E guarde a filosofia do desafio: **arquitetura
aqui é a decisão que você consegue defender com um número.** Não é o padrão que você conhece, é o
limiar que você escolheu, o hit rate que ele sustenta e o gráfico que prova que ele acontece.

## O que reprova sozinho

Cada um destes decide o veredito por si, sem compensação pelo resto da entrega:

1. **Ficção apresentada como fato.** Arquivo, biblioteca, métrica ou endpoint citado como existente
   sem existir (regra 1). Propor o que ainda não existe é permitido e esperado, desde que esteja
   marcado como proposta.
2. **Chave de cache sem isolamento por corretora.**
3. **Evidência que não é sua.** "Antes" copiado do roteiro, de outra máquina ou de outro aluno.
4. **Meia entrega.** SAD sem PoC, ou PoC sem SAD. As duas metades são uma coisa só, e é exatamente
   o que este desafio existe para exigir.
