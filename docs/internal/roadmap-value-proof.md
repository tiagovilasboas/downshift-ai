# Roadmap — provar o valor (nota 7,5 → 9)

**Criado:** 2026-11-20
**Autor:** Tiago Vilas Boas

Downshift by Tiago Vilas Boas — https://github.com/tiagovilasboas/downshift-ai

A engenharia está acima do valor comprovado. Este roadmap ataca os quatro
pontos que faltam para provar o efeito: compactação da família `other`,
resultado por tier, eventos fora do Claude Code e foco do escopo. Cada linha é
um PR, no máximo 5 pontos, mesclável sozinho.

## Linha de base (30 dias até 2026-11-20, claude-code)

| Métrica | Valor hoje | Meta |
|---|--:|--:|
| Saída cortada (total) | 9% (3,8 MB de 41,3 MB) | ≥ 18% |
| Família `other`, corte líquido | 9,9% | ≥ 25% |
| Checagem falhou, rewrite Downshift abaixo | 3 de 187 (1,6%) | não piorar |
| Checagem falhou, orquestrador abaixo | 6 de 357 (1,7%) | referência |
| Checagem falhou, modelo da sessão | 3 de 203 (1,5%) | referência |
| Harnesses não-Claude com evento via hook | 0 | ≥ 1 |
| Linhas `unknown` (harness) | 1835 | atribuídas ou com motivo |

As três linhas de "checagem falhou" vêm de um cruzamento feito à mão (jq) com
outra definição de origem. A definição única do O2 (`telemetry.OriginOf`),
usada pelo report desde o O3, conta 175 / 283 / 174 tasks, com 3 / 1 / 2
falhas. Mais 109 tasks ficam `unattributed`. Compare o fechamento com os
números do report, não com os desta tabela.

## Achados que mudam a ordem

- `other` é quase todo saída de post-run (10.982 de 11.400 linhas). FamilyNone
  nunca chega a um compressor (`internal/execwrap/compact.go:37`), e o corte
  balanceado conta linhas (60+80), então saídas de 4–20 KB com menos de 140
  linhas nunca são cortadas.
- O resultado da checagem do filho (`verification`) já é gravado
  (`internal/childend/record.go:153`), mas nada o lê. "Rework" só conta fim com
  erro, e filhos em background quase nunca reportam erro.
- `internal/cascade/deadends.go:36-44` grava SHA-256 sem salt do texto da
  tarefa. Reversível por dicionário: fere a regra "task text só em memória".
- Comandos compostos com `curl`/`wget`/`glab` como programa interno podem
  estar indo ao CCR pelo corte balanceado (não verificado).
- Cursor não está instalado nesta máquina; Antigravity e Kiro têm hooks
  configurados, mas nenhum disparou. Cursor, Antigravity e Kiro não declaram
  `ShellEnv`, por isso execuções diretas viram `unknown`.

## Tasks

Prefixos: **S** segurança, **C** compactação, **O** resultado, **H** harness,
**F** foco.

| # | Task | Pts | Depende de | Onda |
|---|---|--:|---|--:|
| S1 | CCR não guarda saída de composto com `curl`/`wget`/`glab` | 2 | — | 1 |
| S2 | `cascade`: trocar SHA-256 sem salt por HMAC com salt por instalação, ou remover o hash | 2 | — | 1 |
| C1 | Gravar código de cabeça (vocabulário fechado) e faixa de linhas nas linhas `other` | 3 | — | 1 |
| O1 | Juntar `verification` e outcome em cada task (último registro vence) | 3 | — | 1 |
| O2 | Extrair "origem" do spawn (rewrite, orquestrador abaixo, sessão) em helper único | 3 | — | 2 |
| C3 | Fixtures de saída não classificada e benchmark offline da linha de base | 2 | — | 2 |
| H1 | `run-all.sh` falha quando o fixture não grava evento | 2 | — | 2 |
| C2 | `report` quebra `other` por programa interno, cabeça e faixa | 2 | C1 | 3 |
| O3 | Matriz origem × tier × tipo de agente no JSON do report | 3 | O1, O2 | 3 |
| H2 | Spike: variáveis de ambiente das shells de Cursor, Antigravity e Kiro | 1 | — | 3 |
| C4 | Passo sem perda para saída não classificada (ANSI, progresso, duplicatas) | 3 | C3, S1 | 4 |
| O4 | Mostrar aceite vs retrabalho por origem, rotulado "observacional" | 3 | O3 | 4 |
| H3 | Declarar `ShellEnv` verificado no H2 | 2 | H2 | 4 |
| H4 | Motivo do `unknown` no report; documentar entry `post_tool_use` | 2 | — | 4 |
| C5 | Corte por orçamento de bytes com CCR, mantendo linhas de falha | 5 | C4, C2 | 5 |
| O5 | Sinais de outcome por harness como dado em `HarnessCapabilities` | 2 | O3 | 5 |
| O6 | Claude Code: ler fim com erro de filho em background no SubagentStop | 3 | — | 5 |
| H5 | E2E: fixture Codex prova evento com `harness=codex` e `entry=hook` | 3 | H1 | 5 |
| F1 | `docs/reference/frozen-packages.md` com pacotes congelados | 1 | decisão | 5 |
| F2 | `gateway` e `discover` marcados `Experimental` no clispec | 2 | decisão | 6 |
| F3 | archtest bloqueia código novo em pacote congelado sem exceção | 3 | F1 | 6 |
| F4 | Texto do `report` sobre aceite do `loop` alinhado à realidade | 2 | decisão | 6 |
| C6 | Docs: CCR e matriz de harness para a compactação genérica | 1 | C5 | 6 |
| O7 | Nota de decisão: grupo de controle (holdout), tamanho de amostra | 1 | — | 6 |

**Total:** 55 pontos. No máximo três PRs abertos por vez; a onda seguinte só abre
quando uma anterior fechar.

## Decisões do mantenedor (fora da execução automática)

1. **Holdout (O7 → O8, O9):** deixar uma fração das decisões DOWNSHIFT sem
   reescrita para medir causa ([holdout-decision.md](holdout-decision.md)). Gasta cota premium de propósito; desligado por
   padrão. Com ~200 rewrites/mês, distinguir 3% de 6% exige ~750 por grupo.
2. **Congelar** ([frozen-packages.md](../reference/frozen-packages.md)) `loop`, `cascade`, a superfície `context` do `contextopt`,
   `gateway`, `missedsavings`, `operatorexport`; mover `cursorbridge` para fora
   de `internal/`.
3. **F4:** corrigir o texto de ajuda ou ligar o aceite do `loop` ao `report`.
4. **Fingerprint de tarefa com salt (O10):** detecta retrabalho aceito e refeito,
   mas persiste derivado do texto da tarefa. Exige revisão de segurança.

## Fora de escopo

- Corte com perda em saída com exit ≠ 0.
- Compostos cujo programa interno é leitura de arquivo (ganho ~0, risco real).
- Remoção de pacote sem medir uso real antes.

## Como medir no fechamento

`downshift report --days 30` contra a linha de base acima: `other` e total
cortado, bytes recuperados com `retrieve` (≤ 10% do economizado) e a matriz de
outcome por origem. Comparar por cabeça e por tipo de agente, não só totais.
