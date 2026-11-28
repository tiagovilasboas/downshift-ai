<div align="center">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="../brand/downshift-mark-darkbg.svg">
  <img src="../brand/downshift-mark-lightbg.svg" alt="Símbolo do Downshift" width="90">
</picture>

# Downshift

### Corte o desperdício de tokens no Claude Code, no Codex e em outros agentes de programação.

Agentes de programação com IA gastam boa parte do contexto com logs de teste, saída de build e status, e cada subagente roda no seu modelo principal, a menos que algo defina outro. Isso queima tokens e enche a janela de contexto. Downshift é uma CLI open source que compacta a saída dos comandos suportados antes que o agente a leia, mantendo falhas e códigos de saída: 69,4% menos bytes num benchmark de 128 fixtures. Também aplica o modelo e o esforço escolhidos para cada subagente, dentro da política que você declara. Um único binário Go.

Funciona com harnesses de programação, incluindo Claude Code e Codex CLI; as capacidades de integração e os modelos disponíveis variam por harness e sessão.

[Comece aqui](#comece-aqui) · [Documentação](../guides/install.md) · [Suporte por harness](../reference/harness-matrix.md) · [Evidências](#o-que-as-evidências-mostram) · [Como contribuir](../../CONTRIBUTING.md)

[English](../../README.md) · [Português (Brasil)](README.md)

**Open source · Apache 2.0 · Maturidade do produto: beta**
</div>

![Agentes do Downshift trabalhando juntos sobre um fundo escuro](../brand/readme-media/hero.png)

## Comece aqui

Instale um binário de release compatível, escolha seu harness, revise a configuração proposta e verifique a instalação. Para o roteamento integrado, o harness compatível precisa estar instalado; o compactor também pode ser executado diretamente como CLI independente. O assistente interativo pode configurar hooks, MCP, skills e política de sessão conforme suas escolhas. Ele mostra as alterações propostas e pede confirmação antes de gravar.

Os pacotes de release atendem macOS e Linux em arm64 e amd64, e Windows em amd64. A disponibilidade dos arquivos varia conforme o release; confira a página de releases para sua plataforma e os arquivos de verificação. Há suporte a Homebrew (`tiagovilasboas/tap`), instalador shell (`install.sh`) e Windows PowerShell (`install.ps1`).

[Releases](https://github.com/tiagovilasboas/downshift-ai/releases) · [Instalação](../guides/install.md) · [Início rápido](../guides/quickstart.md) · [Configuração](../guides/config.md) · [Desfazer configuração](../reference/cli.md#downshift-setup) · [Desinstalar](../reference/cli.md#downshift-uninstall)

```bash
# macOS & Linux (Homebrew)
brew install tiagovilasboas/tap/downshift

# macOS & Linux (arm64 / amd64)
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift-ai/main/install.sh | bash

# Windows (PowerShell, amd64)
irm https://raw.githubusercontent.com/tiagovilasboas/downshift-ai/main/install.ps1 | iex

# No seu repositório:
downshift setup
downshift doctor

# Autocompletar opcional no shell (bash, zsh, fish, powershell):
downshift completion zsh > "${fpath[1]}/_downshift"

# Depois, em qualquer sistema: instala a próxima release e atualiza cada harness
downshift update
```

[![O assistente interativo detecta harnesses, permite escolher componentes e revisar alterações, grava após a confirmação e depois o usuário verifica a configuração](../brand/readme-media/setup-flow.svg)](../brand/readme-media/setup-flow.png)

## Novo na v0.25.0

- **Mais compactação em bash e `Read` (`#245`, `#246`, `#247`)**: compacta saídas multilinha sem `\n` final, trata `--harness=<nome>` como não-interativo mesmo sob PTY de agentes, reduz o piso do `Read` para `8 KiB` em arquivos somente-leitura (`*.lock`, `go.sum`, `*.jsonl`, `*.log`, `*.min.*`) com dedup de sub-faixas verificado por hash de linha, e expande regras `read-only` seguras (`git -C`, `git tag -l`, `git config --list`, `git grep`, `docker ps`/`images`/`logs`).
- **Aviso de plugin defasado e atribuição fiel (`#244`)**: `doctor` alerta quando o commit do plugin instalado está atrás do binário e `downshift compact` infere o harness via `core.AllHarnessCaps()` apenas quando há correspondência única.

[Notas da v0.25.0](../releases/v0.25.0.md)

## Novo na v0.24.1

- **Refresh automático em background do cache de atualização (`#241`)**: `session --panel` atualiza `latest_release.json` via processo *detached* com cooldown (`DOWNSHIFT_NO_NETWORK=1` desativa), mantendo o aviso `↑ vX.Y.Z` na barra atualizado automaticamente.
- **Pipes sem espaço após `2>&1`/`2>/dev/null`, `/dev/null` em cadeias e `|| :` (`#242`)**: suporta `2>&1|tail` / `2>/dev/null|grep`, segmentos não guardados com descarte `/dev/null` em cadeias e `|| :` junto com `|| true`.

[Notas da v0.24.1](../releases/v0.24.1.md)

## Novo na v0.24.0

- **Mais reescritas pré-execução e pipes seguros (`#235`, `#238`)**: divisão de cadeias respeitando aspas até 32 segmentos, `git -C <dir>` seguro, pipes sem espaço e de dois estágios (`cmd|grep|tail`), `cat -n` / `sed -n 'A,$p'`, famílias read-only com guardas (`wc`, `find`, `grep`, `rg`, `cat`, `git fetch`) e `quoted_rules` por binário.
- **Maior cobertura pós-execução e MCP (`#233`, `#234`, `#239`)**: podador genérico de payloads MCP, `search_cap` sem número de linha, compactação de stderr (`git fetch`/`pull`/`push`), fallback escalado para saídas verbosas, piso de `512` B no dedup de `Read` e `BalanceFailed` para falhas `FamilyNone`.
- **Versão ao vivo e aviso de atualização no `downshift-agents` (`#236`, `#237`)**: a barra exibe `Downshift - vX.Y.Z` (com `↑ vY.Z.W` quando o cache `latest_release.json` tiver versão mais nova), além do aviso no pane e no `/ds-agents summary`; `discover` também corrige vazamento de timestamp zero e ignora `run_in_background` (`#232`).

[Notas da v0.24.0](../releases/v0.24.0.md)

## Novo na v0.23.0

- **Poda MCP, dedup e `providers` enxuto (`#222`, `#226`, `#230`)**: podadores semânticos para Sentry (`search_issues`, `search_events`) e Datadog (`execute_code`) reduzem fixtures de `4.682 -> 2.497` bytes (`-46,7%`), leituras repetidas de docs MCP na mesma sessão viram ponteiro curto do CCR e `providers` cai de `1.089 -> 176` bytes (`-83,8%`) por padrão.
- **Sem recompactar `retrieve` e `discover` fiel (`#223`, `#224`)**: `downshift retrieve` (CLI e MCP) nunca é recompactado, e `downshift discover` contabiliza a compactação pós-execução de bash e MCP.
- **Telemetria, `report`, `stats` e `doctor` unificados (`#225`, `#227`, `#228`, `#229`)**: economia real + rotação em `context-compactions`, `exec-retrievals` na mesma janela com tool/família, propagação de effort/complexity no `childend`, classificação de comandos compostos e checagem de `scores.json` parado e harnesses inativos no `doctor`.

[Notas da v0.23.0](../releases/v0.23.0.md)

## Novo na v0.22.0

- **Guardas contra travamento e prompt interativo (`#219`)**: `DOWNSHIFT_EXEC_TIMEOUT` limita a execução no `downshift compact` (saindo com `124` no timeout) e chamadas sem TTY injetam `GIT_TERMINAL_PROMPT=0` e `GH_PROMPT_DISABLED=1` para `git`/`gh` em background falharem rápido em vez de pendurarem.
- **Separação de telemetria e wiring explícito (`#218`)**: os read-outs de `internal/telemetry` passaram para `internal/telemetry/readout` e o registro implícito por `init()` foi substituído por `wireRuntime()` explícito em `cmd/downshift`.

[Notas da v0.22.0](../releases/v0.22.0.md)

## Novo na v0.21.0

- **Bypass ativo anti-loop (`#211`)**: após duas falhas idênticas consecutivas de mesmo digest no CCR na mesma sessão dentro de 30 minutos, a compactação com perda sai do caminho (`anti_loop_bypass`) e entrega a saída bruta de falha literalmente até o comando passar.
- **Saúde do cache de prompt e taxa de recuperação no CCR (`#212`)**: `downshift report` e `downshift stats` exibem `cache hit %` por harness (`cache_read`, `cache_write`, `input`) junto com o percentual recuperado via CCR (`% of cut`).
- **Benchmark offline em 1 comando e kit A/B público (`#213`)**: `make bench-proof` verifica 89,2% de corte de bytes (`324,913 -> 35,113` bytes), recuperação exata 4/4 com `downshift retrieve`, `anti_loop_bypass`, invariância de cache de prompt e pisos de risco em `.downshift/policy.json`; `scripts/bench-ab/` adiciona o kit pareado A/B ao vivo.
- **Documentação de segurança de cache e posicionamento neutro (`#214`)**: adicionados [Segurança de cache de prompt](../features/prompt-cache-safety.md), [Reproduzindo benchmarks](../guides/reproduce-benchmark.md) e notas de coexistência com `/compact`.

[Notas da v0.21.0](../releases/v0.21.0.md)

## Otimização de tokens para agentes de programação

Para desenvolvedores que utilizam agentes de código com IA, suítes longas de testes e saídas verbosas de build não apenas gastam tokens — elas diluem a janela de contexto. À medida que o histórico de terminal se acumula, a capacidade do modelo de reter instruções anteriores, invariantes de arquitetura e regras de negócio se degrada. O Downshift compacta a saída de comandos suportados antes de ela chegar ao agente, preservando o foco da memória de trabalho, códigos de saída e detalhes de falhas intactos. Saídas menores protegem a capacidade do contexto e reduzem o consumo de tokens a cada turno.

O Downshift também aplica escolhas de modelo e nível de esforço de raciocínio ao trabalho delegado, dentro da política que você declara. Modelos de fronteira com alto esforço permanecem dedicados a decisões de arquitetura, auditorias de segurança e investigação de bugs complexos, enquanto subagentes mecânicos rodam em modelos menores e mais ágeis, sem invalidar o cache de prompt da sessão principal.

| Capacidade | O que faz |
| --- | --- |
| **Compactação de contexto & saída** | Evita a degradação do contexto ao encurtar o ruído de comandos limpos, preservando logs de falha, com recuperação bruta por 24 h (`downshift retrieve <id>`). |
| **Compactação de leituras do desktop** | Encurta as leituras da árvore de acessibilidade do `agent-desktop` sem perder campo, e manda uma releitura da mesma janela só com as linhas que mudaram ([guia](../guides/agent-desktop.md)). |
| **Roteamento de modelo & esforço por subagente** | Aplica o tier do modelo e o nível de esforço de raciocínio na criação do subagente conforme a política, preservando capacidade de fronteira para decisões complexas. |
| **Relatórios locais & telemetria** | Mede tokens, estimativas relativas em unidades de cota, trabalho mecânico na sessão principal e cobertura sem enviar prompts, código ou caminhos para fora da máquina. |

### Valor prático nas sessões do dia a dia

Mesmo quando o fluxo já separa planejamento de execução ou usa subagentes, as sessões diárias ainda desperdiçam cota e contexto de forma previsível. O Downshift ataca quatro pontos práticos direto no fluxo do harness:

- **Guardar cota de fronteira para decisões difíceis:** buscas e checagens mecânicas delegadas a subagentes rodam em tiers menores e proporcionais, mantendo intacto o cache de prompt da sessão principal; `downshift stats`, `downshift report` e o painel `downshift-agents` também mostram estimativas relativas em unidades de cota (assumindo tokens iguais, por harness — nunca como fatura).
- **Cortar ruído sem perder o dado bruto:** suítes limpas, listas do `git`, `WebFetch`/`mcp__*` (> 4 KB no Claude Code) e `Read` grandes (> 64 KB) chegam enxutos para o agente não perder o foco nas regras do sistema, enquanto códigos de saída permanecem intactos e o CCR guarda a saída elegível localmente por 24 h para leitura sob demanda via `downshift retrieve <id>`.
- **Revelar delegação perdida e volume fora da compactação:** o `downshift report` aponta chamadas mecânicas que continuam rodando direto na sessão principal de fronteira, e `downshift discover --dedup` mostra o volume de Bash e não-Bash elegível por harness.
- **Zero latência extra de LLM com skills leves:** roteamento e compactação rodam localmente em código Go determinístico (fail-open, `events.jsonl` sem prompts), enquanto `downshift setup --yes` instala skills concisas como `ponytail` nos harnesses compatíveis (`SkillsDir`) e acompanha a adoção em 30 dias localmente.

[Quando usar o Downshift](../guides/when-to-use.md) · [Arquitetura](../design/architecture.md)

Selecione um diagrama para vê-lo em tamanho maior. Os rótulos das artes estão em inglês; o texto abaixo explica cada fluxo em português.

## Onde o Downshift atua

[![Um harness de programação acessa o Downshift por caminhos separados de roteamento de subagentes e execução de comandos; as observações dependem do harness](../brand/readme-media/operating-surfaces.svg)](../brand/readme-media/operating-surfaces.png)

O Downshift participa em dois pontos distintos: a criação de um subagente pode ser verificada contra a política e os modelos conhecidos da sessão, e a saída de comandos suportados pode ser compactada antes de chegar ao agente. Integração e observação variam entre harnesses; a execução direta do compactor está disponível quando não há encapsulamento automático.

## Reduza a saída dos comandos e mantenha os detalhes

[![Um comando de desenvolvimento gera uma saída completa; o Downshift mostra uma versão menor ao agente, enquanto a saída bruta elegível pode ser armazenada localmente com CCR e recuperada depois](../brand/readme-media/compaction.svg)](../brand/readme-media/compaction.png)

Um comando suportado é executado normalmente. O Downshift mostra uma versão mais curta ao agente, preservando o código de saída e os detalhes de falhas. Com o **Command Context Recovery (CCR)** ativado, a saída bruta elegível pode ser armazenada localmente e recuperada depois; linhas repetidas podem ser agrupadas no resultado exibido. O encapsulamento depende do harness, e a execução direta do compactor está disponível. A redução da saída varia conforme o comando e o formato.

```bash
downshift compact -- go test ./...
```

A saída limpa é compactada enquanto falhas e códigos de saída são preservados. Em execuções sem TTY na entrada padrão (`stdin`), o encapsulador injeta `GIT_TERMINAL_PROMPT=0` e `GH_PROMPT_DISABLED=1` para que comandos `git`/`gh` em segundo plano falhem rápido em vez de travar esperando senha, e `DOWNSHIFT_EXEC_TIMEOUT` (ex.: `30s`, `2m`) limita a execução do processo filho com código de saída `124` ao expirar.

[![As famílias reconhecidas incluem testes, builds e verificações, Git e infraestrutura; o comportamento depende do comando e do formato de saída](../brand/readme-media/command-families.svg)](../brand/readme-media/command-families.png)

O Downshift reconhece formas específicas de comandos de teste, build, lint, Git e infraestrutura. Formatos reconhecidos podem ser compactados ou repassados conforme a família e a saída. Formatos de saída desconhecidos permanecem brutos; um comando fora da lista permitida é recusado pelo encapsulador e pode ser executado diretamente. Os detalhes de falhas e o código de saída são preservados. Consulte a referência de ferramentas para conhecer as formas exatas.

[Compactação](../features/compaction.md) · [Ferramentas suportadas](../reference/supported-tools.md) · [CCR](../guides/ccr.md)

### Apps do desktop: computer use com menos tokens

Um agente que controla apps do desktop com o `agent-desktop` lê a árvore de acessibilidade do app em JSON depois de cada ação. O Downshift compacta essas leituras também:

- **Primeira leitura de uma janela:** uma linha por elemento, sem perder campo (um teste decodifica de volta para o mesmo JSON).
- **Releitura da mesma janela:** só as linhas que mudaram, com a leitura completa a um `downshift retrieve` de distância.
- **Todo harness:** automático no Claude Code (e em reescritas pré-execução somente leitura onde suportado); nos outros, por `downshift compact -- agent-desktop snapshot ...`.

Em fixtures sintéticos, a primeira leitura teve de 53% a 60% menos bytes, uma releitura depois de uma ação 97% menos, e um ciclo ler, agir, conferir de três leituras 84,9% menos. Ainda não foi medido num desktop real.

```bash
downshift compact -- agent-desktop snapshot --app Files -i
```

[Guia do agent-desktop](guias/agent-desktop.md)

## Roteie subagentes dentro da sua política

[![O orquestrador delega o trabalho; o Downshift verifica a política, os modelos conhecidos da sessão e as capacidades do harness antes de aplicar uma escolha ou manter o modelo solicitado](../brand/readme-media/routing.svg)](../brand/readme-media/routing.png)

O orquestrador escolhe o trabalho a delegar. O Downshift aplica decisões de modelo na criação do subagente, sujeito à sua política e às informações de modelos conhecidas para aquele harness e sessão. Quando não consegue estabelecer uma escolha segura, mantém o modelo solicitado.

A profundidade da integração varia. Uma reescrita emitida pelo Downshift, por si só, não comprova que o harness a aplicou. O código de roteamento do Downshift não faz uma chamada adicional a LLM. A escolha se aplica à criação do subagente; o Downshift não altera o modelo da sessão principal no meio do turno.

[Roteamento de modelos](../features/routing.md) · [Política](../guides/policy.md) · [Modelos da sessão](../reference/session-models.md)

## Veja seus agentes em ação

![Ilustração conceitual inspirada no painel Downshift Agents: um agente líder coordena agentes especialistas](../brand/readme-media/agents.png)

O Downshift inclui perfis de agentes e playbooks de orquestração para pesquisa, implementação, revisão, segurança, documentação e outras atividades. Um líder coordena o trabalho independente e reúne os resultados.

<a id="5-agent-orchestration-context-triggers"></a>
### Orquestração de agentes (gatilhos contextuais)

No chat do seu harness de programação, descreva sua tarefa e digite:
- `downshift full-power` (alias `potência total` / `potencia-total`): ativa os especialistas necessários respeitando o teto de capacidade da sessão.
- `downshift shift <squad>`: ativa um domínio específico (`core`, `quality`, `security`, `leadership`).

<p align="center"><img src="../brand/readme-media/agents-panel-illustration.png" width="420" alt="Ilustração sanitizada baseada em um painel Downshift Agents"></p>

*Ilustração sanitizada baseada em um screenshot de uma sessão Claude Code. Nomes de tarefas, modelos e métricas da sessão foram ocultados. Esta é uma prévia visual editada, não um benchmark ou screenshot sem alterações.*

[Guia de agentes](../guides/agents.md) · [Catálogo de agentes](../agents/README.md) · [Skills](../../skills/README.md)

## Funciona com seu harness de programação

O Downshift foi criado com Claude Code e oferece suporte a outros harnesses em diferentes estágios de verificação. **Suportado** significa que o `doctor` verificou o hook em uma sessão real; **Beta** significa que o hook rodou em uma sessão real e está documentado, mas o `doctor` ainda não o verificou; **Experimental** significa que o efeito não foi confirmado ou foi visto uma vez. Critérios e evidências: [níveis de suporte](../reference/harness-matrix.md#support-tiers).

| Harness | Nível | Evidência de roteamento |
| --- | --- | --- |
| Claude Code | Suportado | Reescrita observada em um subagente real; o `doctor` a lê de sessões reais. |
| Codex | Beta | Reescrita emitida; aplicação inferida, sem observação direta. Precisa da flag de hooks e da confiança via `/hooks`; o `doctor` lê a flag e indica a correção. |
| OpenCode | Beta | Roteamento e uso do subagente observados no OpenCode 2.2.5 pelo plugin ponte. |
| KiroCrew | Beta | Bloqueio por política e nova tentativa observados; sem reescrita de modelo no lugar. |
| Kiro IDE | Beta | Payload do hook capturado ao vivo; só observação, sem reescrita. |
| Cursor | Experimental | Reescrita emitida; aplicação pelo executor não confirmada. |
| Antigravity | Experimental | Um subagente observado em uma versão. |

Os modelos disponíveis para um harness ou sessão não são considerados disponíveis automaticamente em outro. Consulte a [matriz completa de harnesses](../reference/harness-matrix.md) para detalhes de integração, evidências e limites.

## O que as evidências mostram

| Medição | Resultado | Escopo |
| --- | --- | --- |
| Compactação de saída sem perda | **60,0% menos bytes** | 79 fixtures sem perda em um corpus sintético. |
| Compactação de saída com CCR | **69,4% menos bytes** | Corpus sintético de 128 fixtures (53,4% com CCR desligado), saída de comandos e leituras do agent-desktop; repetições exibidas podem ser agrupadas e a saída bruta pode ser recuperada. |
| Compactação balanced, saídas grandes | **80,9% menos bytes** | 4 saídas reais deste repositório acima de 16 KB (`go test -v`, `git show`, `git log -p`, `grep -rn`); 45,2% só com o compressor sem perda. As linhas cortadas voltam com `downshift retrieve`. |
| Roteamento de subagentes | **24,1% menos tokens por execução aceita** | Estudo controlado com 46 execuções no Claude Code para tarefas mecânicas, realizado antes da decisão O8. |

São medições separadas, com unidades e cargas de trabalho diferentes. Os totais do corpus sintético incluem apenas fixtures processadas pelo compactor e excluem fixtures preservadas. O resultado de roteamento é evidência histórica; ele não estabelece o efeito da implementação após O8. A economia no uso cotidiano sem restrições ainda não foi estabelecida. Estimativas de tokens baseadas em bytes não são uma fatura nem uma medida dos limites do plano. O número da v0.18.0 acima (3,5 a 4,5 vezes) é uma projeção a partir de transcripts gravados, não uma destas medições.

[Benchmark de compactação](../reference/bench.md) · [Estudo de roteamento](../evidence/four-arm-mechanical-2026-10-21.md) · [Guia de evidências](../guides/evidence.md)

### Reproduza você mesmo em 5 segundos

Valide compactação, recuperação sem perdas no CCR, preservação de falhas em testes, bypass automático anti-loop e invariância de cache de prompt direto no binário compilado, sem gastar tokens de API:

```bash
make bench-proof
```

Para rodar testes pareados **Off vs. Downshift** ao vivo no Claude Code ou no Codex (medindo taxa de aceitação objetiva com `go test` oculto além de tokens faturáveis, `cache hit %` e estimativa em preço de lista de API), use o kit em [como reproduzir o benchmark](../guides/reproduce-benchmark.md).

O projeto também publica estudos com resultado negativo ao lado dos positivos: dois lotes controlados pré-v0.20 no Claude Code não mostraram queda nos tokens faturáveis quando os comandos já vinham curtos ou encadeados ([estudo de compactação](../evidence/compaction-token-study-2026-11-16.md), [estudo pós-O8](../evidence/post-o8-mechanical-study-2026-11-16.md)) — o que motivou as expansões de corte pós-execução (4 KiB), pipes/cadeias e saída não classificada na `v0.18`–`v0.20`.

## Por que usar Downshift além de `/compact` e roteamento nativo?

- **Cache de prompt preservado por construção:** o Downshift não troca o modelo da sessão principal no meio do turno nem reescreve histórico já enviado. O roteamento atua na fronteira do subagente e a compactação ocorre na entrada da saída de ferramentas, enquanto `downshift stats` e `downshift report` expõem `cache hit %` e `cache write` por harness ([Segurança de prompt cache](../features/prompt-cache-safety.md)).
- **Compactação orientada a sinal com anti-loop ativo:** seis test runners preservam falhas linha a linha; quando uma saída não reconhecida falha duas vezes seguidas na mesma sessão (`anti_loop_bypass`), o corte com perda é desligado automaticamente para entregar o log bruto na terceira tentativa, e o `downshift report` mede o percentual recuperado via `downshift retrieve` (`% of cut`).
- **Independência de fornecedor e multi-harness:** integra Claude Code, Codex, OpenCode, Cursor, Kiro, KiroCrew e Antigravity — além de `downshift compact -- <cmd>` funcionar como CLI pura em qualquer ambiente.
- **FinOps local transparente e política nas suas mãos:** `.downshift/policy.json` garante pisos explícitos para áreas sensíveis (`authz`, pagamentos, criptografia) e mede retrabalho por veredito, cobertura não compactada e saúde do cache sem enviar prompts para fora da máquina.

## A pesquisa por trás do design

O design do Downshift parte dos trabalhos publicados pela Anthropic sobre orquestração. Um líder forte planeja e confere, workers mais baratos fazem trabalho delimitado, e cada agente a mais precisa pagar os tokens que gasta.

- [How we built our multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system) (2025): um líder Opus com subagentes Sonnet superou um único agente Opus em 90,2% em pesquisa, com cerca de 15× os tokens de um chat. O mesmo texto observa que programação se paraleliza menos que pesquisa.
- [Patterns and problems in emerging multiagent systems](https://www.anthropic.com/research/multiagent-systems) (2026): um enxame coordenado encontrou 266 vulnerabilidades contra 21 dos agentes independentes, em boa parte por cobrir mais terreno; nos mesmos diretórios, os tokens por achado foram comparáveis.
- [Riemann zeta research with 60 subagents](https://www.anthropic.com/research/riemann-zeta) (2026): de 60 subagentes, 2 desenvolveram as ideias centrais, 30 tentaram e falharam, e 13 validaram os argumentos.

Como cada achado vira um princípio de design, e o que ele não mostra: [True north](../true-north.md) · [Reconciliação](../evidence/anthropic-references-benchmark-reconciliation-2026-11-07.md)

## Feito para inspeção e controle

| Comando | Descrição e Finalidade |
|---|---|
| `downshift setup [--dry-run]` | Configura hooks nos harnesses detectados com prévia |
| `downshift doctor` | Diagnóstico de integridade do ambiente com correções automáticas |
| `downshift compact -- <cmd>` | Executa comando permitido com compactação de saída limpa |
| `downshift retrieve <id>` | Recupera saída bruta mantida no buffer seguro do CCR |
| `downshift session [--why]` | Exibe a árvore de subagentes da sessão e a justificativa das decisões |
| `downshift stats` | Resumo de decisões de roteamento e estimativa de tokens economizados |
| `downshift report` | Relatório consolidado de tokens direcionados por tarefa concluída |
| `downshift policy check` | Valida sintaxe e semântica do `policy.json` |
| `downshift quota` | Diagnóstico de cotas e janelas de consumo dos provedores |
| `downshift mcp serve` | Inicia servidor MCP local via stdio (13 ferramentas nativas) |

- Hooks falham abertos: erros ou timeouts mantêm a criação do subagente solicitada sem alteração.
- O roteamento é local e não faz uma chamada adicional a LLM. A telemetria registra contagens, hashes e tempos, sem registrar prompts.
- O CCR pode armazenar saídas brutas de comandos localmente. A sanitização cobre formatos comuns de credenciais, mas segredos em formatos arbitrários podem permanecer; consulte as [limitações de saída bruta](../reference/known-limitations.md#raw-output-kept-by-ccr) e a configuração antes de usá-lo com saídas sensíveis.
- Workers com escrita exigem ativação explícita e usam worktrees separados. O Downshift não mescla suas branches nem oferece uma etapa de aprovação para deploys e outras ações de risco.

[Segurança](../features/safety.md) · [Telemetria](../features/telemetry.md) · [Limitações conhecidas](../reference/known-limitations.md)

## Perguntas frequentes

**Como reduzir o consumo de tokens no Claude Code, no Codex ou no Cursor?** Instale o Downshift e rode `downshift setup`. A saída dos comandos suportados é compactada antes de entrar na janela de contexto, e cada subagente roda no modelo escolhido para ele, dentro da sua política. No Claude Code a reescrita é automática; nos outros, o agente roda `downshift compact -- <comando>`. Confira seus números com `downshift session` e `downshift report`.

**Subagentes do Claude Code gastam meu limite de uso mais rápido?** Podem gastar. Cada subagente começa com um contexto novo e faz as próprias requisições, que contam no mesmo limite da conversa principal, e roda no seu modelo principal, a menos que algo defina outro. Coloque os subagentes de rotina num modelo menor: `downshift policy init` grava um perfil `explore` que manda o Explore para o tier pequeno. [Guia](https://downshift-ai.vercel.app/pt-br/guias/subagentes-gastam-limite-claude-code/)

**O Downshift economiza meu limite de uso do Claude Code?** Ainda não foi medido. O Downshift mostra os tokens que rodaram abaixo do modelo da sessão e os bytes que a compactação removeu. Como um plano converte tokens nos limites de 5 horas e semanal não é público, então não afirmamos economia de limite.

**O roteamento de subagentes quebra o cache de prompt?** Não. A escolha vale para o subagente inteiro, não para turnos da sessão principal, que mantém um único modelo durante toda a conversa.

**Qual a diferença para o `/compact`?** O `/compact` resume o histórico que já está na janela de contexto. O Downshift encolhe a saída dos comandos antes que ela entre. Os dois se complementam.

**Qual a diferença para o `CLAUDE_CODE_SUBAGENT_MODEL`?** Essa variável define um modelo padrão para todo subagente que nada mais configura. O Downshift aplica o modelo escolhido para cada subagente dentro de uma política declarada, mantém o modelo pedido na dúvida, registra onde os tokens rodaram e também funciona no Codex, no Cursor e no OpenCode.



## Navegação da Documentação

A estrutura abaixo detalha cada etapa do ciclo de vida da requisição, combinando os objetivos de arquitetura de produto com os respectivos pontos de código:

| Documento | Foco | O que cobre |
|---|---|---|
| [01 · Entrada do Fluxo](arquitetura/01-entrada.md) | Ciclo de Vida | O que o Downshift intercepta no stdin, limites de payload e o que é ignorado |
| [02 · Classificador](arquitetura/02-classificador.md) | Decisão | Heurísticas de regex, matriz de sinais ponderados e o limiar de confiança |
| [Visão BDI e fluxo completo (EN)](../../README.md#bdi-agentic-view-beliefs--intentions) | Arquitetura | Primeiro a lente conceitual, depois o caminho executado e os limites de observação |
| [03 · Gestão de Marchas](arquitetura/03-marcha.md) | Política | As faixas de capacidade (Small/Mid/Frontier), regras de downshift e upshift seguro |
| [04 · Fallbacks e Tolerância](arquitetura/04-fallbacks.md) | Resiliência | O contrato fail-open: como o sistema reage a erros sem quebrar o fluxo do dev |
| [05 · Catálogo e Sessão](arquitetura/05-catalogo-e-sessao.md) | Permissões | Preço de lista versus modelos realmente autorizados na sessão ativa (`session-models.json`) e, desde `downshift models discover`, os modelos que o próprio harness informa ([session-discovery](../reference/session-discovery.md), em inglês) |
| [06 · Adapters de Harness](arquitetura/06-harnesses.md) | Integração | Especificidades de integração: Claude Code, Codex e Cursor; Antigravity e KiroCrew (Experimental) |
| [07 · Boost semântico por hash](arquitetura/07-minilm.md) | Classificação | FNV em processo, centróides, condições de subida e privacidade; o nome do arquivo é histórico |
| [08 · Telemetria e Métricas](arquitetura/08-telemetria.md) | Observabilidade | Estrutura do `events.jsonl`, métricas normalizadas e auditoria sem vazamento de prompt |
| [09 · Decisões Técnicas e Trade-offs](arquitetura/09-decisoes.md) | Engenharia | Racional de arquitetura, compensações técnicas e próximos passos do projeto |
| [11 · Compressor Nativo de Contexto](../design/context-optimization.md) | Otimização | Compressor determinístico em Go e sensor de telemetria passiva |
| [13 · Plugin, skills e agente (EN)](../design/architecture.md#packaging-layers-what-an-agent-harness-loads) | Empacotamento | As cinco camadas que um harness carrega: memória (`AGENTS.md`), skills (`skills/`), hooks (binário; `hooks/hooks.json` no plugin), agente de exemplo (`agents/`) e o plugin do Claude Code (`/plugin install downshift --marketplace tiagovilasboas/downshift-ai`) |
| [14 · Companions (EN)](../guides/companions.md) | Ecossistema | Headroom como companion padrão (detectado e registrado, nunca instalado pelo Downshift), mods do Claude Code recomendados (You should know, Token Weather, Replay Theater, reflect-mod), padrões que o Downshift segue mas não empacota |


## Documentação e comunidade

[Início rápido](../guides/quickstart.md) · [Referência de CLI](../reference/cli.md) · [Arquitetura](../design/architecture.md) · [English](../../README.md) · [Artigo: como economizar tokens no Claude Code](https://dev.to/tiagovilasboas/como-economizar-tokens-no-claude-code-conheca-o-downshift-4f0m)

Encontrou um problema ou quer contribuir? Leia o [guia de contribuição](../../CONTRIBUTING.md) ou [abra uma issue](https://github.com/tiagovilasboas/downshift-ai/issues).

## Licença

[Apache License 2.0](../../LICENSE). Downshift por [Tiago Vilas Boas](https://github.com/tiagovilasboas).
