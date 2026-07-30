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
