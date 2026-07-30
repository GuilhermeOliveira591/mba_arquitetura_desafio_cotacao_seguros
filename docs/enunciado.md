# Desafio — Fundamentos de Arquitetura de Solução

> Rascunho do enunciado. Este documento vira o `README.md` do starter na publicação.

## 1. O negócio

Antes de olhar qualquer diagrama: de que empresa estamos falando e como ela ganha dinheiro. Toda
decisão que você vai defender no SAD se apoia aqui.

### A empresa, o produto e os inquilinos

A **Prumo Tecnologia em Seguros** é uma empresa fictícia. Ela não vende seguro: vende software
para quem vende. O produto é o **Prumo Cota**, plataforma de cotação de **seguro auto** contratada
por corretoras. Um corretor informa motorista e veículo, o Prumo Cota consulta **três seguradoras
parceiras** e devolve as propostas lado a lado, da mais barata para a mais cara. A Prumo não emite
apólice, não assume risco e não precifica: ela intermedeia.

A plataforma é **multi-tenant** — cada requisição declara a corretora no cabeçalho `X-Tenant-Id`
(duas cadastradas no starter, algumas centenas em produção). E isso não é detalhe de
implementação: cada corretora tem condições comerciais próprias com cada seguradora, então a mesma
placa, para o mesmo motorista, volta com prêmios diferentes conforme quem pergunta. Uma resposta
entregue à corretora errada é erro de preço **e** vazamento de dado de terceiro.

O Prumo Cota é uma fachada sobre sistemas que a Prumo não controla — motores de precificação
legados, com janelas de manutenção próprias e SLAs que as próprias seguradoras descumprem. Se a
dependência externa cai, não há produto. **A instabilidade das parceiras é o coração do negócio,
não um problema plantado para o exercício.**

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

São 360.000 consultas por dia útil: **R$ 14.400 por dia**, ~**R$ 317.000 por mês** em chamadas a
parceiro, contra R$ 660.000 de receita. Quase metade da receita bruta sai pela porta da dependência
externa — e boa parte dessas consultas é repetida, porque o corretor recota a mesma placa várias
vezes na mesma conversa e o cliente pede a mesma cotação em mais de uma corretora.

As seguradoras honram o prêmio informado por **até 24 horas**, e o contrato com a corretora exige
que a cotação exibida ainda seja praticável na hora da venda. Existe, portanto, um teto legítimo
para reaproveitar uma cotação — e reaproveitar por mais tempo é mais barato e mais arriscado ao
mesmo tempo. **Onde parar é decisão de negócio, e é sua.**

### O que a regulação impõe

**SUSEP.** A Prumo processa o registro das operações de corretoras reguladas. Toda cotação
apresentada a um consumidor precisa ser **auditável** — qual seguradora, qual prêmio, para qual
corretora, em que instante — retida por cinco anos e não regravável. Isso tem custo de
armazenamento, pesa na escolha de onde os dados vivem e obriga que resposta servida de cache ou de
fallback continue rastreável até a consulta que a originou.

**LGPD.** Uma cotação carrega CPF, ano de nascimento e placa: dados pessoais. A corretora é
**controladora**, a Prumo é **operadora**, e o contrato entre elas proíbe uso cruzado de dados,
exigindo minimização e prazo de descarte. Isolamento entre inquilinos deixa de ser cuidado de
engenharia e vira obrigação legal: uma chave de cache que devolva a uma corretora o que outra
cotou não é bug — é incidente de dados pessoais, com dever de notificação à ANPD.

### Por que isso importa para a sua entrega

1. **O cache é decisão financeira.** Cada acerto é uma consulta que não foi comprada; o efeito do
   hit rate entra com número na planilha de TCO da seção 8 do seu SAD.
2. **On-premise vs. cloud se decide com compliance, não com preferência.** SUSEP e LGPD são os
   argumentos que a seção 5 do seu SAD tem que enfrentar: retenção, residência e segregação.
3. **Disponibilidade é promessa comercial.** Hoje a disponibilidade do Prumo Cota é o *produto* da
   disponibilidade das três parceiras. Quem vende para corretora não pode entregar isso.

## 2. Entrega 1 — o SAD

A primeira entrega é um documento: o **Solution Architecture Document** do Prumo Cota depois da sua
intervenção. Ele descreve o sistema que você vai construir na entrega 2 e, mais que descrever,
**justifica**. A ordem importa: o PoC é subordinado ao SAD, não o contrário. Você implementa a
fatia que o seu próprio documento defendeu.

Escreva para três leitores que existem de verdade e que não leem template:

- o **CTO da Prumo**, que aprova (ou não) o gasto e quer ver a conta;
- quem vai **operar** a plataforma às 3h da manhã, e precisa saber o que fazer quando uma parceira
  afundar;
- o **encarregado de dados / auditoria**, que precisa provar à ANPD e à SUSEP que a cotação servida
  de cache continua rastreável e que uma corretora nunca vê o dado de outra.

Se uma frase do seu SAD não serve a nenhum dos três, ela é enchimento.

### Três regras que valem para o documento inteiro

**Regra 1 — rastreabilidade. Citar arquivo que não existe reprova.** Toda afirmação sobre o sistema
aponta para algo verificável: um caminho real do repositório (`internal/partner/client.go`), um
trecho deste enunciado, ou uma medição que você fez e colou. A correção é estática — quem corrige
**abre os caminhos citados**. Um `internal/resilience/breaker.go` que só existe no documento é
reprovação imediata, não desconto de nota, e o mesmo vale para biblioteca inventada, métrica que
ninguém emite e endpoint que o código não serve. Escrever sobre o que ainda não existe é permitido
e esperado — desde que esteja marcado como o que é: proposta. O que reprova é apresentar ficção
como fato.

**Regra 2 — números, não adjetivos.** "Escalável", "robusto", "alta disponibilidade", "baixa
latência" e "performático" não são requisitos: são opiniões. Um requisito não funcional precisa de
**métrica, valor, unidade e método de medição** — "p95 do `POST /quotes` abaixo de 800 ms, medido
no histograma `http_server_request_duration_seconds` do Prometheus". Sem os quatro, não conta como
requisito. O mesmo vale para dinheiro: a seção 8 é uma planilha, não um parágrafo dizendo que o
cache "reduz custos".

**Regra 3 — diagrama é código, em C4 com Mermaid.** Todo diagrama vai em bloco Mermaid — três
crases seguidas de `mermaid` — dentro do próprio Markdown. Imagem colada não vale: o que precisa
ser verificado é o texto, porque ele dá `diff`, aparece na revisão e não desatualiza em silêncio.
Níveis obrigatórios: **1 (contexto)** e **2 (contêiner)** na seção 2 do seu SAD, **3 (componente)**
na seção 4, e só da fatia que você implementa. Nível 4 (código) não é pedido.

O Mermaid tem `C4Context`/`C4Container` nativos, ainda experimentais; um `flowchart` também serve,
desde que a semântica do C4 esteja preservada — pessoa, sistema, contêiner e sistema externo
distinguíveis, e **toda relação rotulada com o que trafega e por qual tecnologia**. Seta sem rótulo
não é diagrama de arquitetura.

```mermaid
flowchart LR
  broker(["Corretora<br/><i>pessoa</i>"])
  api["quotation-api<br/><i>contêiner · Go</i>"]
  slow["partner-slow<br/><i>sistema externo</i>"]

  broker -- "cota um risco<br/>HTTP/JSON · X-Tenant-Id" --> api
  api -- "consulta prêmio<br/>HTTP/JSON" --> slow
```

### As oito seções

#### 1. Introdução — propósito, escopo, restrições, pressupostos

Diga para quem o documento serve e qual decisão ele sustenta. Delimite o escopo pelos dois lados: o
SAD cobre a plataforma de cotação e a sua **fronteira** com as seguradoras — não redesenha o motor
de precificação delas nem o CRM da corretora. E separe com clareza o que é **restrição** (imposta:
Go, ambiente local sem custo, três parceiras, cobrança por consulta, retenção SUSEP de cinco anos,
papel de operadora na LGPD) do que é **decisão** (sua: TTL, limiares do breaker, onde a plataforma
roda). Confundir os dois é o erro que faz o documento inteiro perder autoridade — ninguém defende
uma escolha que apresenta como imposição.

Pressuposto é número que você assume sem poder medir hoje. Declare cada um com o valor, a origem e
a **consequência se ele for falso**: "assumo que 30% das cotações se repetem em até uma hora (o
corretor recota a mesma placa na mesma conversa); se for 5%, o cache não se paga e a seção 8 muda
de veredito".

- **Mínimo verificável:** escopo com lista explícita do que ficou de fora; restrições separadas das
  decisões; cada pressuposto com valor, origem e o que muda no documento se ele cair.
- **Não conta:** pressuposto sem consequência declarada — é enfeite, não pressuposto.

#### 2. Visão geral da arquitetura

Uma página que mostre o **antes** e o **depois**, com os diagramas C4 de nível 1 e 2. O "antes" não
é imaginação: os contêineres estão no `docker-compose.yml` e os nomes têm que bater
(`quotation-api`, `partner-slow`, `partner-flaky`, `partner-degrading`, `redis`, `otel-collector`,
`jaeger`, `prometheus`). O nível 1 mostra os atores externos — a corretora e as três seguradoras —
e deixa visível o que a seção 1 deste enunciado afirma: a Prumo intermedeia, não precifica.

Feche com o **delta** em lista: o que você acrescenta, o que muda de lugar e o que sai. Um
parágrafo curto de "a quem esta arquitetura serve" ancora tudo o que vem depois.

- **Mínimo verificável:** C4 nível 1 e nível 2 em Mermaid; o diagrama do "antes" bate com os
  serviços do `docker-compose.yml`; lista explícita das mudanças entre antes e depois.
- **Reprova:** contêiner no diagrama que não existe nem no compose nem na sua entrega — é o caso
  mais comum de violação da regra 1.

#### 3. Requisitos funcionais e não funcionais

Os funcionais saem do contrato que já existe, não da sua cabeça: `POST /quotes` exige `X-Tenant-Id`
e rejeita quem não mandar; a resposta agrega as três parceiras ordenadas da mais barata para a mais
cara (`internal/quotation/service.go`); toda cotação apresentada tem que ficar auditável por cinco
anos; e uma resposta servida de cache ou de fallback continua rastreável até a consulta que a
originou. Escreva cada um como frase testável, com identificador (RF-01…), e acrescente os que a
**sua** arquitetura cria.

Os não funcionais vão em tabela, e o "hoje" é medido, não estimado — ele está no
`docs/roteiro-cenario-de-falha.md` e você reproduz com `make reproduce`:

| ID | Requisito | Métrica | Hoje | Alvo | Como medir | Por que este número |
|---|---|---|---|---|---|---|
| RNF-01 | Latência da cotação | p95 do `POST /quotes` | 8,01 s sob carga | *sua decisão* | `http_server_request_duration_seconds` | *sua justificativa* |

A coluna que separa o arquiteto do preenchedor de template é a última. Um alvo de p95 tem que vir
de algum lugar — do tempo que o corretor aguenta esperar com o cliente na frente dele, do SLA que a
Prumo vende, ou da soma das parceiras no melhor caso. "Porque é um bom número" não é origem.

- **Mínimo verificável:** funcionais com identificador e frase testável; não funcionais com
  métrica, valor, unidade, método e origem do alvo; a coluna "hoje" coerente com o que o roteiro
  mede.
- **Não conta:** qualquer não funcional sem número ou sem método de medição.

#### 4. Detalhamento da arquitetura

O coração do documento. Aqui entram as decisões, cada uma no formato **contexto → opções
consideradas → escolha → consequências, inclusive as ruins**. No mínimo: circuit breaker, cache e
fallback — e, se você mexer neles, timeout e a serialidade da agregação, que hoje soma as três
parceiras em vez de sobrepô-las.

Todo parâmetro numérico é uma decisão e precisa da sua defesa: por que o breaker abre com N falhas
em uma janela de M segundos e não com outro par; quantas requisições o estado meio aberto deixa
passar; qual o TTL do cache diante da janela de **24 horas** em que a seguradora honra o prêmio — e
por que parar antes dela, se você parar. Escreva a **chave do cache literalmente**: se `tenant_id`
não estiver nela, você acabou de projetar um incidente de dados pessoais, não uma otimização. E
diga o que o fallback devolve, como o cliente sabe que aquilo é fallback, e como a resposta
continua auditável para a SUSEP.

O C4 de nível 3 mora aqui, cobrindo a fatia protegida — a fronteira com a parceira vive em
`internal/partner/client.go` e a agregação em `internal/quotation/service.go`.

- **Mínimo verificável:** os três mecanismos com opções descartadas e consequências; todo parâmetro
  numérico justificado; a chave de cache escrita por extenso; C4 nível 3 da fatia implementada.
- **Não conta:** "usaremos circuit breaker" sem os três estados, sem limiar e sem janela. É
  exatamente a frase que este desafio existe para tornar impossível.

#### 5. Implementação

Duas perguntas: **como se constrói** e **onde roda**.

Como: o mapa de cada mecanismo para o arquivo do repositório onde ele nasce, com caminhos reais;
qual biblioteca e por que ela (a de circuit breaker que expõe os três estados explicitamente vale
mais que a mais estrelada); e como o comportamento é **testado sem depender de sorte** — o starter
já mostra o padrão em `cmd/partner-mock/feasibility_test.go`, que prova de forma determinística que
os defaults das parceiras permitem um breaker abrir.

Onde: é aqui que se decide **on-premise, cloud ou híbrido**, e a decisão se defende com o
compliance do domínio, não com preferência. SUSEP impõe retenção de cinco anos, registro não
regravável e uma conversa sobre residência do dado; a LGPD põe a Prumo como operadora, com
segregação entre controladoras e prazo de descarte. Some o custo, que reaparece na seção 8. "Cloud
é o padrão de mercado" não é argumento — é ausência de argumento.

- **Mínimo verificável:** mapa mecanismo → arquivo real do repositório; decisão de hospedagem com
  pelo menos uma alternativa descartada e o motivo regulatório e financeiro; estratégia de teste do
  comportamento resiliente.

#### 6. Operação e gestão de mudanças

O SAD é lido em produção, sob pressão. A observabilidade que você instrumentar na entrega 2 é o
instrumento desta seção: diga **o que se olha** (estado do breaker, hit rate do cache, latência por
parceira), **qual limiar dispara alerta** e **qual ação o alerta exige**. Alerta sem limiar e sem
ação não é alerta, é ruído.

Escreva pelo menos um **runbook** completo de um cenário real — "o breaker da `partner-degrading`
está aberto há dez minutos" — com o que o plantonista faz, o que ele **não** faz, e quando escala.
E trate a gestão de mudanças pelo caso que importa neste domínio: o TTL do cache é parâmetro de
**negócio**, porque mexe em risco de preço vencido e em custo. Quem aprova mudá-lo, como ela chega
em produção, como se reverte e como se mede se melhorou.

- **Mínimo verificável:** cada alerta com métrica, limiar e ação; um runbook do começo ao fim; o
  caminho completo de mudança de um parâmetro de negócio, com reversão.
- **Não conta:** "monitoraremos com Prometheus e Grafana". Isso é uma lista de compras.

#### 7. Recuperação de desastres

RTO e RPO com número — e **por classe de dado**, porque elas não valem o mesmo. Perder o cache
custa dinheiro e latência; perder o registro de auditoria da SUSEP é infração regulatória. Um RPO
único para os dois é sinal de que a seção não foi pensada.

Cubra no mínimo três cenários: (a) o Redis inteiro se perde; (b) uma parceira fica fora por seis
horas; (c) perda do site ou da região onde a plataforma roda. Para cada um: o que a corretora vê, o
que **degrada** e o que **para** (não é a mesma coisa), quanto custa em reais o modo degradado, e o
caminho de volta. O cenário (a) tem endereço na seção 8: cache vazio é 100% das consultas compradas
de novo.

- **Mínimo verificável:** RTO e RPO numéricos por classe de dado; três cenários com efeito no
  cliente, custo e procedimento de retorno; distinção explícita entre degradar e parar.

#### 8. Tecnologias, custos e pessoal (TCO)

A seção que fecha o circuito — a linha de código da entrega 2 tem que aparecer aqui como dinheiro.
Os números de partida estão na seção 1 deste enunciado: R$ 0,04 por consulta, três consultas por
cotação, 120.000 cotações por dia útil, 22 dias úteis, R$ 0,25 de receita por cotação entregue.

Entregue quatro coisas:

1. **A conta de parceiro por cenário.** Uma linha para hoje (hit rate zero) e uma para cada hit
   rate que o **seu** TTL sustenta, com as colunas: hit rate assumido · consultas compradas por mês
   · custo mensal · economia contra hoje · percentual da receita. Sem valores prontos aqui: a conta
   é sua.
2. **O custo da infraestrutura que você acrescenta.** Redis, retenção de traces e métricas, e o
   armazenamento de auditoria — este último **cresce todo mês** e precisa de cinco anos de
   retenção, então mostre pelo menos o ano 1 e o ano 5, não só o primeiro mês.
3. **Pessoal.** Quantas pessoas, quais papéis, para construir e depois para operar, com custo
   mensal. Arquitetura que ninguém tem gente para manter é arquitetura que não existe.
4. **O veredito.** O cache se paga? Em quanto tempo? Se a resposta for "não", diga — um SAD que
   conclui contra a solução preferida, com a conta na mão, vale mais que um que conclui a favor sem
   ela.

**É aqui que o documento inteiro é testado por coerência.** O hit rate desta seção tem que ser
compatível com o TTL da seção 4 e com o pressuposto de recotação da seção 1. TTL de quinze minutos
sustentando 70% de acerto exige explicação; se ela não estiver em lugar nenhum, o número é chute —
e um chute na planilha contamina a decisão que ele sustenta.

- **Mínimo verificável:** planilha de custo de parceiro por cenário; custo de infraestrutura com a
  curva da retenção de cinco anos; custo de pessoal; veredito explícito de payback; coerência
  numérica com as seções 1 e 4.
- **Não conta:** preço de instância copiado de calculadora sem dizer quantas instâncias e por quê.

### O que este SAD não é

Não é um template preenchido, e a diferença é verificável de fora: **cada seção referencia outra**.
O requisito da seção 3 aparece como mecanismo na 4, como arquivo na 5, como alerta na 6, como
cenário de desastre na 7 e como reais na 8. Um documento em que as oito seções poderiam ser lidas
em qualquer ordem, sem que nada quebrasse, é oito documentos curtos — não um SAD.

Também não é um documento longo por obrigação. Quantos requisitos, quantos diagramas e quantas
páginas estão reunidos no bloco **Critérios quantitativos**, no fim deste enunciado, e é lá que
você confere se entregou o suficiente.

## 3. Entrega 2 — o PoC

A segunda entrega é código: a fatia do seu SAD construída sobre o starter e **provada em execução**.
Ela é pequena de propósito. Não é aqui que você mostra fôlego de desenvolvedor — é aqui que você
mostra que a arquitetura que defendeu no documento sobrevive ao contato com a `partner-flaky`.

A subordinação ao SAD é literal, e vale nos dois sentidos. O documento **pode** propor mais do que o
PoC implementa, desde que diga qual fatia foi implementada e qual ficou como proposta. O que não
pode existir é o contrário: um limiar de breaker ou um TTL que aparece no código sem estar defendido
na seção 4 do SAD é um número chutado, mesmo que funcione. Se você descobrir implementando que a
decisão do documento estava errada — ótimo, é para isso que serve um PoC. **Volte e corrija o SAD**,
dizendo o que a medição mostrou. Divergência entre os dois é o que perde ponto; mudar de ideia com
evidência na mão é o que se espera de um arquiteto.

### Três mecanismos, e o freio de mão

Você constrói exatamente isto:

1. **Circuit breaker** com os três estados explícitos, na fronteira com as parceiras.
2. **Cache** com política de invalidação declarada e justificada.
3. **Fallback** para quando a parceira não responde.

Mais **timeout** e, se você quiser, **paralelizar a agregação** — dois acompanhantes baratos que a
seção 4 do seu SAD provavelmente vai defender de qualquer jeito.

E nada além disso. Retry com backoff, bulkhead, rate limiting, fila, autenticação, persistência de
auditoria de verdade, Kubernetes: **não implemente**. Cite no SAD o que fizer sentido citar — lá é o
lugar de propor. Aqui, cada mecanismo a mais é tempo que sai das evidências e vai para o plumbing, e
o que está sendo avaliado é a decisão, não a quantidade de padrões que você conhece. Se você está
escrevendo o quarto pattern, você saiu da matéria.

O vácuo que você preenche está identificado no código, sem mistério: `internal/partner/client.go`
(cliente sem timeout, sem proteção, sem fallback) e `internal/quotation/service.go` (agregação em
série, tudo ou nada). O Redis já sobe no compose — sem persistência, de propósito — e a API ainda
**não fala com ele**: essa ligação é sua.

### Circuit breaker — os três estados têm que ser visíveis de fora

Fechado, aberto e meio aberto não são jargão de prova: são três comportamentos diferentes, e a sua
entrega precisa mostrar os três acontecendo. O que costuma passar batido é o significado operacional
do estado aberto: **aberto quer dizer que a parceira não é chamada**. Se a requisição sai e o
resultado é descartado, você não economizou o R$ 0,04 da consulta nem o tempo de espera — você
escreveu um contador de falhas com nome bonito.

Decida e defenda o **escopo** do breaker: um por parceira, um por par (parceira, corretora), ou um
só para todas. Um breaker global é a decisão que derruba a plataforma inteira porque uma das três
afundou — pode ser defendida, mas dificilmente é o que você quer num agregador de três fornecedores
independentes.

Os parâmetros vêm do SAD: quantas falhas em qual janela abrem, quanto tempo o circuito fica aberto,
quantas requisições o meio aberto deixa passar, o que fecha e o que reabre. E o comportamento se
testa **sem depender de sorte**: o starter mostra o padrão em
`cmd/partner-mock/feasibility_test.go`, e `docs/smoke-test-factibilidade.md` documenta a rajada de
nove falhas consecutivas nas sequências 49 a 57 da `partner-flaky` — você sabe de antemão onde o
breaker deve abrir.

- **Mínimo verificável:** os três estados nomeados no código; o estado aberto realmente não chama a
  parceira; escopo do breaker declarado; parâmetros idênticos aos da seção 4 do SAD.
- **Não conta:** biblioteca importada e configurada sem uma única evidência de transição de estado.
  É a versão em código da frase "usaremos circuit breaker".

### Cache — a chave é o contrato de isolamento

Escreva a **chave por extenso**, no SAD e no README do processo. Ela precisa conter, no mínimo, a
corretora e a identificação normalizada do risco cotado; se o seu cache é por parceira ou pela
cotação agregada é decisão sua, com consequência sua — cache por parceira sobrevive a uma parceira
fora, cache agregado economiza mais e vence inteiro de uma vez.

> **Chave de cache sem `tenant_id` reprova.** Não é rigor de estilo: é a mesma placa devolvendo à
> corretora A o prêmio negociado pela corretora B — erro de preço e incidente de dados pessoais, com
> dever de notificação à ANPD. É o único defeito isolado do PoC que reprova sozinho.

O **TTL é decisão de negócio**, e tem teto: a seguradora honra o prêmio informado por até 24 horas.
Um TTL maior que isso não é cache agressivo, é cotação que ninguém honra. Parar antes das 24h é o
esperado — mas diga por quê, e o número que você escolher aqui é o mesmo que sustenta o hit rate da
planilha da seção 8 do seu SAD.

TTL, porém, não é a política inteira. Declare também: o que acontece com a entrada quando o breaker
abre (a cotação vencida ainda é servida? por quanto tempo a mais? a corretora fica sabendo?), o que
invalida uma entrada antes da hora, e como as entradas somem quando têm que sumir — o Redis do
starter é efêmero, mas prazo de descarte de dado pessoal é obrigação da LGPD, não configuração de
container.

E a resposta servida de cache continua sendo uma cotação apresentada a um consumidor: ela precisa
seguir **auditável para a SUSEP**, rastreável até a consulta original que a produziu e o instante em
que ela foi comprada.

- **Mínimo verificável:** chave escrita por extenso e contendo a corretora; TTL justificado contra a
  janela de 24 horas; política de invalidação declarada; resposta de cache distinguível e
  rastreável.
- **Reprova:** chave sem isolamento por corretora.

### Fallback — o que a corretora recebe quando não há resposta

Hoje a cotação morre: uma parceira falha e a requisição inteira vira 502, jogando fora as respostas
que já tinham chegado (`internal/quotation/service.go`). Decidir o que colocar no lugar disso é a
parte mais próxima do negócio de todo o PoC. Três saídas são legítimas, e você escolhe uma e
defende:

- **resposta parcial** — entrega as parceiras que responderam e diz explicitamente qual faltou;
- **cotação anterior** — serve do cache, marcada como tal, com a idade dela na resposta;
- **recusa explícita** — se o produto não admite proposta incompleta, um erro de negócio claro,
  distinguível de um erro genérico, com o que a corretora deve fazer.

O que não é legítimo é **inventar um prêmio**. Um preço fabricado apresentado ao consumidor é
problema regulatório, não bug de aplicação.

Em qualquer das três, a corretora tem que saber que aquilo é degradado — o que significa que o
contrato de resposta muda (`Response`, em `internal/quotation/request.go`), e a mudança tem que
estar documentada. E, de novo: se a resposta de fallback chega ao consumidor, ela é auditável como
qualquer outra.

- **Mínimo verificável:** comportamento sob parceira fora implementado e refletido no contrato de
  resposta; a degradação é visível para quem chama; a escolha entre as opções está defendida no SAD.
- **Não conta:** log dizendo "fallback acionado" enquanto o cliente continua recebendo 502.

### Instrumentação — é o seu método de prova, não um segundo exercício

Não se pede observabilidade aqui para você exercitar OpenTelemetry. Pede-se porque **nenhuma
afirmação desta entrega vale sem o dado que a sustenta**: "o breaker abre" se prova com a série
temporal do estado, não com o parágrafo dizendo que abre. Essa é a razão de o starter já entregar o
SDK, o Collector, o Jaeger e o Prometheus de pé (`internal/platform/telemetry.go`) — o plumbing não
é o exercício, e três horas gastas nele são três horas roubadas do que está sendo avaliado.

O que já vem pronto é genérico: HTTP de entrada, HTTP de saída e runtime do Go. O que falta é o de
negócio, e são três coisas:

1. **Transição de estado do breaker**, por parceira, com origem e destino. Você precisa conseguir
   desenhar o degrau fechado → aberto → meio aberto → fechado no tempo — um valor observável do
   estado atual, um contador de transições, ou os dois. E a requisição que foi curto-circuitada tem
   que dizer isso no trace (evento ou atributo no span), senão ela aparece como uma cotação
   misteriosamente rápida.
2. **`hit` e `miss` do cache**, com atributo que permita calcular o hit rate em PromQL. Este é o
   número que alimenta a planilha da seção 8 do SAD; sem ele, a economia que você projeta é chute.
3. **Latência por parceira**, com significado de domínio. Hoje ela existe só pela borda HTTP
   (`http_client_request_duration_seconds`, atributo `server_address`). Reaproveitar essa métrica é
   legítimo — desde que você diga que reaproveitou, e mostre a consulta.

**Atributo é dado retido.** `tenant_id` (algumas centenas de valores) é útil e aceitável. **CPF,
placa e `quote_id` não entram em span nem em métrica**: além de dado pessoal fora de lugar, placa em
rótulo de métrica é cardinalidade sem teto — dois problemas por uma decisão preguiçosa. E
instrumente pouco: span por função e métrica por variável não é observabilidade, é ruído com custo
de retenção, e a seção 6 do seu SAD vai ter que explicar quem olha aquilo.

- **Mínimo verificável:** as três instrumentações no código, com os nomes de métrica e de atributo
  declarados no README do processo; a consulta que lê cada uma escrita na entrega; nenhum dado
  pessoal em atributo.
- **Não conta:** métrica citada na documentação que o código não emite. É a regra 1 aplicada ao
  código — quem corrige procura o nome no repositório.

### Evidências — o formato

Uma evidência tem três partes: **o comando ou a consulta que a gerou**, **o artefato** e **uma
legenda de uma linha dizendo o que se vê nele**. Faltando qualquer uma, é figura decorativa. Tudo
mora em `docs/evidencias/` e é referenciado do README do processo — arquivo que ninguém consegue
abrir não conta como entregue.

**O "antes" é seu.** Rode `make down && make reproduce` na sua máquina *antes* de tocar no código.
Os números de `docs/roteiro-cenario-de-falha.md` foram medidos em outra máquina; copiá-los é
apresentar ficção como fato, que é exatamente o que a regra 1 reprova. As latências vão diferir das
de lá, e tudo bem — o que se compara é o seu antes com o seu depois.

O conjunto mínimo:

| Evidência | Formato | O que ela tem que mostrar |
|---|---|---|
| Relatório antes/depois | **texto colado**, saída completa do `make reproduce`, mesma carga nas duas | a diferença de p95, de taxa de sucesso e de vazão |
| Trace com breaker aberto | screenshot ou export JSON do Jaeger | a requisição que **não chamou** a parceira curto-circuitada, e respondeu rápido |
| Trace servido de cache | screenshot ou export JSON do Jaeger | a cotação sem os spans de saída para as parceiras |
| Estado do breaker no tempo | gráfico **com a consulta PromQL colada como texto** | o degrau fechado → aberto → meio aberto → fechado |
| Hit rate do cache | gráfico **com a consulta** | a curva subindo conforme o cache aquece |
| p95 do `POST /quotes` | gráfico **com a consulta** | antes e depois, na mesma escala |

Três regras de formato que decidem se a evidência é verificável:

1. **Relatório vai como texto, não como imagem.** Texto dá `diff`, dá busca e cabe na revisão.
2. **Gráfico sem a consulta ao lado não vale.** Quem corrige precisa poder repetir a leitura; um
   gráfico é uma afirmação, a consulta é a fonte dela.
3. **Toda evidência declara a janela de tempo** — `Last Hour` no Jaeger, `[5m]` no PromQL. Um
   gráfico sem janela pode estar mostrando qualquer coisa.

- **Mínimo verificável:** o conjunto da tabela acima, cada item com comando/consulta e legenda,
  todos referenciados do README do processo. Quantos itens além do mínimo, no bloco **Critérios
  quantitativos**.
- **Não conta:** screenshot de dashboard sem a consulta; evidência que mostra a métrica existindo
  mas nunca mudando de valor — o que se pede é a **transição**, não a existência.
- **Reprova:** "antes" copiado do roteiro ou de outro aluno.

### Os testes e os defaults do starter

`make test` tem que passar, incluindo os testes que já vieram — quebrar o starter para o seu código
caber é regressão, não refatoração. E o comportamento resiliente se testa como o starter já ensina
em `cmd/partner-mock/feasibility_test.go`: determinístico, sem `sleep` e sem esperar que a sorte
coopere. A rajada documentada da `partner-flaky` existe justamente para isso.

Os perfis das parceiras (`PARTNER_SEED`, `PARTNER_FAILURE_RATE`, latências) são **restrição, não
decisão**. Mexer neles para o breaker abrir mais fácil é resolver o exercício mudando o enunciado, e
invalida a evidência principal. Rodar uma variação declarada — outra taxa de falha, para explorar o
limiar — é legítimo e até interessante, desde que a evidência que sustenta a sua entrega venha dos
defaults, e que `make smoke` continue verde.

### O que este PoC não é

Não é produto, não é o starter reescrito e não é lugar de mostrar repertório de padrões. São três
mecanismos, a instrumentação que prova que eles funcionam, e as evidências. Se ao fim você tem um
sistema mais bonito e nenhum gráfico mostrando o breaker abrir, você entregou a metade que não
estava sendo pedida.

## 4. Critérios quantitativos e regras de entrega

Todo número deste desafio está neste bloco. O resto do enunciado diz o que se espera e por quê; aqui
está **quanto** — para que você saiba quando parou de faltar, e para que dois corretores diferentes
cheguem ao mesmo veredito lendo a mesma entrega.

Os valores abaixo estão dimensionados para um esforço de **8 a 12 horas** e são **mínimos, não
alvos**. Bater o mínimo em tudo é uma entrega aprovável; não é uma entrega boa.

### Entrega 1 — o SAD

| O que | Mínimo | Onde |
|---|---|---|
| Requisitos funcionais, com identificador e frase testável | 6, sendo ao menos 2 criados pela sua arquitetura | seção 3 do SAD |
| Requisitos não funcionais, com métrica, valor, unidade, método e origem do alvo | 5, sendo ao menos 3 com a coluna "hoje" medida por você | seção 3 do SAD |
| Pressupostos com valor, origem e consequência se forem falsos | 3 | seção 1 do SAD |
| Diagramas C4 em Mermaid | 4: nível 1, nível 2 do "antes", nível 2 do "depois" e nível 3 da fatia implementada | seções 2 e 4 do SAD |
| Decisões no formato contexto → opções → escolha → consequências, cada uma com ao menos uma alternativa descartada e as consequências ruins | 4: circuit breaker, cache, fallback e hospedagem | seções 4 e 5 do SAD |
| Alertas com métrica, limiar e ação | 3 | seção 6 do SAD |
| Runbook completo, do sintoma ao escalonamento | 1 | seção 6 do SAD |
| Cenários de desastre, com efeito no cliente, custo do modo degradado e caminho de volta | 3: Redis perdido, parceira fora por seis horas, perda do site ou da região | seção 7 do SAD |
| Linhas na conta de parceiro | 3: hoje (hit rate zero) e ao menos 2 hit rates que o seu TTL sustente | seção 8 do SAD |
| Horizonte do custo de auditoria | ano 1 e ano 5 | seção 8 do SAD |

**Extensão: de 10 a 20 páginas equivalentes.** Isso é orientação, não regra — ninguém conta páginas
na correção. Abaixo da faixa é provável que alguma seção tenha ficado sem conteúdo; acima dela,
releia procurando enchimento, porque nenhum dos três leitores da seção 2 chega à página trinta.

### Entrega 2 — o PoC

| O que | Mínimo |
|---|---|
| Mecanismos implementados | 3: circuit breaker, cache e fallback. Timeout e paralelização da agregação são bem-vindos, mas não contam como um dos três |
| Métricas de negócio, com o nome declarado no README do processo | 3: estado ou transição do breaker, `hit`/`miss` do cache, latência por parceira |
| Marcações no trace | 2: a requisição curto-circuitada pelo breaker e a resposta servida de cache |
| Testes determinísticos do comportamento resiliente | 2 (um do breaker abrindo, um do cache ou do fallback), com `make test` verde |
| Evidências | as 6 linhas da tabela da seção 3, cada uma com o comando ou a consulta que a gerou e a legenda |

**Formato dos arquivos de evidência:** relatórios em texto; imagens em PNG ou JPG legíveis em
tamanho real; export de trace do Jaeger em JSON, que é melhor que screenshot e ocupa menos. Nada de
PDF com print dentro. Arquivo de imagem acima de 5 MB é sinal de que você exportou a tela errada.

### Regras de entrega

1. **Forke este repositório.** A entrega vive no seu fork, público.
2. **Entregue na branch `main`.** O que estiver em outra branch não é lido.
3. **O `README.md` deixa de ser este enunciado** e passa a ser o **README do processo**, a porta de
   entrada da sua entrega.
4. **Estrutura esperada** (o código fica nas pastas do próprio starter):

   ```
   README.md              # o README do processo, substituindo este enunciado
   docs/sad.md            # o SAD completo, as oito secoes
   docs/evidencias/       # as evidencias, referenciadas pelo README
   ```

5. **A correção é estática:** quem corrige **não roda** a sua aplicação. O que só existe em execução
   não conta como entregue.
6. **O veredito é binário:** aprovado ou não aprovado.

O README do processo é curto e tem cinco coisas: link para o SAD e para as evidências; como subir o
ambiente e reproduzir a sua versão; **o que foi implementado e o que ficou como proposta**; os nomes
das métricas e dos atributos que você criou; e o que você faria diferente com mais tempo.

### O que reprova sozinho

Consolidando o que já apareceu ao longo do enunciado — cada um destes decide o veredito por si, sem
compensação pelo resto da entrega:

1. **Ficção apresentada como fato.** Arquivo, biblioteca, métrica ou endpoint citado como existente
   sem existir (regra 1, seção 2). Propor o que ainda não existe é permitido e esperado — desde que
   esteja marcado como proposta.
2. **Chave de cache sem isolamento por corretora** (seção 3).
3. **Evidência que não é sua.** "Antes" copiado do roteiro, de outra máquina ou de outro aluno
   (seção 3).
4. **Meia entrega.** SAD sem PoC, ou PoC sem SAD. As duas metades são uma coisa só — é exatamente o
   que este desafio existe para exigir.
