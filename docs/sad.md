# SAD: Prumo Cota (plataforma de cotação de seguro auto)

## 1. Introdução


## 2. Visão geral da arquitetura

### Nível 1: Contexto (vale para o antes e o depois: a fronteira com o mundo não muda)

```mermaid
flowchart LR
  broker(["Corretora<br/><i>pessoa</i>"])
  prumo["Prumo Cota<br/><i>sistema de software</i>"]
  slow(["Seguradora parceira<br/><i>sistema externo · partner-slow</i>"])
  flaky(["Seguradora parceira<br/><i>sistema externo · partner-flaky</i>"])
  degrading(["Seguradora parceira<br/><i>sistema externo · partner-degrading</i>"])

  broker -- "cota motorista + veículo<br/>HTTP/JSON · X-Tenant-Id" --> prumo
  prumo -- "consulta prêmio<br/>HTTP/JSON" --> slow
  prumo -- "consulta prêmio<br/>HTTP/JSON" --> flaky
  prumo -- "consulta prêmio<br/>HTTP/JSON" --> degrading
```

A Prumo intermedeia três seguradoras e devolve a lista ordenada por prêmio; ela não precifica risco
nem emite apólice (os motores de precificação são das seguradoras, fora da fronteira deste SAD).

### Nível 2: Contêiner, ANTES

```mermaid
flowchart LR
  broker(["Corretora<br/><i>pessoa</i>"])

  subgraph prumo["Prumo Cota"]
    api["quotation-api<br/><i>contêiner · Go</i>"]
    redis[("redis<br/><i>contêiner · Redis 7</i>")]
    collector["otel-collector<br/><i>contêiner · OTel Collector</i>"]
    jaeger["jaeger<br/><i>contêiner · Jaeger</i>"]
    prometheus["prometheus<br/><i>contêiner · Prometheus</i>"]
  end

  slow(["partner-slow<br/><i>sistema externo</i>"])
  flaky(["partner-flaky<br/><i>sistema externo</i>"])
  degrading(["partner-degrading<br/><i>sistema externo</i>"])

  broker -- "POST /quotes<br/>HTTP/JSON · X-Tenant-Id" --> api
  api -- "consulta em série, sem timeout<br/>HTTP/JSON" --> slow
  api -- "consulta em série, sem timeout<br/>HTTP/JSON" --> flaky
  api -- "consulta em série, sem timeout<br/>HTTP/JSON" --> degrading
  api -. "sobe no compose, nenhum client conecta<br/>RESP · não usado" .-> redis
  api -- "traces + métricas genéricas<br/>OTLP/gRPC :4317" --> collector
  collector -- "traces" --> jaeger
  prometheus -- "scrape métricas<br/>HTTP :8889" --> collector
```

Uma falha em qualquer parceira aborta a requisição inteira (`internal/quotation/service.go`), e o
tempo de resposta é a soma das três chamadas sequenciais, sem teto (`internal/partner/client.go`, sem
`Timeout`).

### Nível 2: Contêiner, DEPOIS

```mermaid
flowchart LR
  broker(["Corretora<br/><i>pessoa</i>"])

  subgraph prumo["Prumo Cota"]
    api["quotation-api<br/><i>contêiner · Go<br/>+ circuit breaker, cache, fallback</i>"]
    redis[("redis<br/><i>contêiner · Redis 7</i>")]
    collector["otel-collector<br/><i>contêiner · OTel Collector</i>"]
    jaeger["jaeger<br/><i>contêiner · Jaeger</i>"]
    prometheus["prometheus<br/><i>contêiner · Prometheus</i>"]
  end

  slow(["partner-slow<br/><i>sistema externo</i>"])
  flaky(["partner-flaky<br/><i>sistema externo</i>"])
  degrading(["partner-degrading<br/><i>sistema externo</i>"])

  broker -- "POST /quotes<br/>HTTP/JSON · X-Tenant-Id" --> api
  api -- "consulta protegida por breaker + timeout 2s<br/>HTTP/JSON" --> slow
  api -- "consulta protegida por breaker + timeout 2s<br/>HTTP/JSON" --> flaky
  api -- "consulta protegida por breaker + timeout 2s<br/>HTTP/JSON" --> degrading
  api -- "lê/escreve cotação em cache<br/>chave por tenant+parceira+risco · RESP" --> redis
  api -- "traces + métricas genéricas e de negócio<br/>OTLP/gRPC :4317" --> collector
  collector -- "traces" --> jaeger
  prometheus -- "scrape métricas<br/>HTTP :8889" --> collector
```

### O delta

- **Acrescenta:** uso efetivo do `redis` (já subia ocioso no compose) como cache de cotação por
  parceira; timeout de 2000ms e circuit breaker por parceira (`sony/gobreaker`, 5 falhas consecutivas)
  na chamada de `internal/partner/client.go`; fallback de resposta parcial com complemento de cotação
  anterior de cache quando uma parceira falha ou está com o circuito aberto; três métricas de negócio
  e duas marcações de trace novas, exportadas ao mesmo `otel-collector` que já está de pé.
- **Muda de lugar:** nada muda de contêiner (os mesmos oito serviços do `docker-compose.yml`
  continuam existindo com o mesmo papel). Toda a diferença fica dentro do processo `quotation-api`
  (o cliente HTTP decorado) e no `redis`, que passa de ocioso a consumido.
- **Sai:** nada sai. O contrato de sucesso de `POST /quotes` é estendido (não substituído) para
  carregar a marcação de degradação que o fallback introduz (ver seção 4).

**A quem esta arquitetura serve:** para a corretora, o delta é a diferença entre um 502 quando
qualquer parceira tropeça e uma cotação parcial ou levemente desatualizada, mas utilizável, na maioria
das vezes em que isso acontece; para quem opera a plataforma às 3h da manhã, é ter um estado de
circuito e um hit rate para olhar antes de um cliente ligar reclamando; para o encarregado de dados, é
a garantia de que a cotação servida de cache ou de fallback continua isolada por corretora e
rastreável até a consulta que a originou.

## 3. Requisitos funcionais e não funcionais



## 4. Detalhamento da arquitetura



## 5. Implementação



## 6. Operação e gestão de mudanças



## 7. Recuperação de desastres



## 8. Tecnologias, custos e pessoal (TCO)


