# Nota de decisão: grupo de controle (holdout)

**Criado:** 2026-11-20
**Autor:** Tiago Vilas Boas
**Task:** O7 do [roadmap-value-proof.md](roadmap-value-proof.md)
**Status:** aguarda decisão do mantenedor. Nada foi implementado.

Downshift by Tiago Vilas Boas — https://github.com/tiagovilasboas/downshift-ai

## Pergunta

A matriz de outcome por origem (O3, O4) é observacional: o orquestrador escolhe
quem desce de tier, então a origem não é aleatória. Para afirmar que o rewrite
do Downshift **não causa** mais falhas, seria preciso um grupo de controle: uma
fração aleatória das decisões de rewrite ficaria sem reescrita, rodando no
modelo pedido.

## Dados de hoje (30 dias até 2026-11-20)

| Origem | Tasks | Checagem falhou |
|---|--:|--:|
| Rewrite Downshift | 175 | 3 (1,7%) |
| Orquestrador abaixo da sessão | 283 | 1 (0,4%) |
| Modelo da sessão | 174 | 2 (1,1%) |

Cerca de 40% das tasks aceitas rodaram sem checagem, então a taxa de falha real
é desconhecida para elas.

## Tamanho de amostra

Teste de duas proporções, bicaudal, α = 0,05 e poder de 80%:

| Detectar | Tasks por grupo |
|---|--:|
| 1,6% → 3,2% (dobro da taxa atual) | ~1.430 |
| 1,6% → 4,8% (triplo) | ~470 |
| 3% → 6% | ~750 |

São cerca de 175 rewrites por mês. Com metade indo para o controle, cada grupo
recebe uns 88 por mês, então detectar o dobro da taxa atual levaria **mais de
16 meses**. Mesmo o triplo levaria uns 5 meses.

## Custo

Cada task do controle roda no tier pedido, que é mais caro, de propósito. Com
50% de holdout, metade da economia de roteamento some enquanto o experimento
durar.

## Recomendação

**Não implementar o holdout agora.** O volume não sustenta um resultado em prazo
útil, e o custo é certo. Em vez disso:

1. Reduzir as tasks sem checagem, que hoje são 40% das aceitas. Sem checagem,
   nenhum desenho mede falha.
2. Manter a matriz observacional (O4), com amostras pequenas marcadas, e
   comparar por tipo de agente, que é o fator mais parecido entre as origens.
3. Reavaliar quando houver ≥ 500 rewrites por mês **ou** a taxa de falha do
   rewrite passar de 2× a da sessão por dois meses seguidos.

O8 e O9 (implementar o holdout e o relatório causal) ficam fora do roadmap até
essa reavaliação.

## Para decidir

- [ ] Aceitar a recomendação (adiar holdout)
- [ ] Implementar holdout mesmo assim, com fração e prazo definidos
