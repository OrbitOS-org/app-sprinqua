# Sprinkl — Arquitetura e Estado de Implementação

> Atualizado: 2026-05-08

---

## Estado geral

| Módulo | Estado |
|---|---|
| Setup Wizard (4 passos) | ✅ Implementado |
| Dashboard + controlo manual | ✅ Implementado |
| i18n (EN / PT) | ✅ Implementado |
| Board registry | ✅ Implementado (Keyestudio 4ch) |
| Zone Engine (ON/OFF/Pulse + safety timer) | ✅ Implementado |
| MQTT — recolha de config | ✅ Wizard implementado |
| MQTT — cliente publicação/subscrição | ⬜ Não implementado |
| Agendamento (scheduler) | ✅ Implementado |
| Smart Watering (sensores, meteo) | ⬜ Não implementado |
| Histórico de ativações | ⬜ Não implementado |

---

## 1. Estrutura de ficheiros

```
cmd/sprinkl/
  main.go              — Entrypoint: flags, Gravity RT client, config load, engine init
  metadata.json        — Manifest OrbitOS (package_id, permissões)

internal/
  board/
    registry.go        — Boards suportadas e mapeamento de canais GPIO
  config/
    config.go          — Load/Save config.json no workdir
  i18n/
    i18n.go            — Deteção de língua, carregamento de strings
    locales/en.json    — Strings EN
    locales/pt.json    — Strings PT
  scheduler/
    scheduler.go       — Goroutine de agendamento, NextRunFor, SetEngine
  zone/
    engine.go          — Controlo de relés, safety timer, estado das zonas
  web/
    server.go          — HTTP server, registo no AppHub, helpers i18n/render
    handlers.go        — Todos os handlers HTTP
    templates/
      wizard.html      — Página completa do wizard + step1
      step2.html       — Configuração de zonas
      step3.html       — Teste de relés
      step4.html       — Configuração MQTT
      dashboard.html   — Dashboard completo
      zones_fragment.html — Fragment HTMX para polling de zonas
      schedule_list.html  — Fragment HTMX com lista de programas
      schedule_form.html  — Página de criação/edição de programa
```

---

## 2. OrbitOS / Gravity RT

- Ligação via `client.NewClientAuto(host)` — UDS no device, TCP+mTLS do laptop
- `AppHubManager.RegisterWebUI(addr, "/sprinkl")` — regista a UI no portal OrbitOS
- `GpioManager.SetDirection / SetLevel` — controlo de relés
- `SystemManager.GetHardwareModel()` — modelo do hardware mostrado no dashboard
- Config e estado guardados **apenas no working directory** (sandbox OrbitOS)
- Porta HTTP: **8083**

---

## 3. Setup Wizard

Quatro passos, cada um renderizado via HTMX swap em `#wizard-content`:

**Step 1 — Board**
- Dropdown com boards suportadas
- Badges dos canais disponíveis (CH1…CHn) sem expor detalhes de GPIO ao utilizador

**Step 2 — Zonas**
- Uma zona por canal, com nome, tipo (aspersão / gotejamento / nebulização) e duração máxima em minutos
- Tipo default: **aspersão** (sprinkler)
- Ao submeter este passo, todos os pinos são inicializados como OUTPUT + OFF antes de avançar para os testes

**Step 3 — Testes**
- Botão por zona que ativa o relé durante 3 segundos
- Cliques repetidos no mesmo canal cancelam o pulse anterior (context.WithCancel)
- Erros de GPIO registados em log (nunca ignorados silenciosamente)

**Step 4 — MQTT**
- Toggle para ativar, campos para broker, porta, prefixo de tópicos, utilizador e password
- Se desativado, o config é guardado sem MQTT

Ao concluir o wizard, o config é gravado em `config.json` e o Zone Engine é inicializado.

---

## 4. Zone Engine

Ficheiro: `internal/zone/engine.go`

- Mapa `zones map[int]*entry` indexado pelo ID da zona
- `Init()` — inicializa todos os pinos como OUTPUT e garante estado OFF no arranque:
  1. Pre-set nível OFF antes do SetDirection (para drivers que honram o valor inicial)
  2. SetDirection(OUT)
  3. SetLevel OFF novamente como confirmação
- `TurnOn(id)` — liga relé, inicia safety timer via goroutine + context
- `TurnOff(id)` — desliga relé, cancela safety timer
- `Pulse(id, secs)` — liga e desliga automaticamente após N segundos
- `States()` — snapshot ordenado do estado de todas as zonas (usado pelo HTMX polling)

---

## 5. Board Registry

Ficheiro: `internal/board/registry.go`

Boards atualmente suportadas:

| ID | Nome | Canais | ActiveLow |
|---|---|---|---|
| `keyestudio-4ch` | Keyestudio RPI 4-Channel Relay | 4 | false |

`ActiveLow: false` — GPIO HIGH = relé ON, GPIO LOW = relé OFF (comportamento confirmado no ambiente OrbitOS/Gravity RT).

Adicionar uma nova board: acrescentar entrada ao slice `All` com o ID, nome, número de canais e slice de `Channel{Number, Pin}`.

---

## 6. Web UI

- **HTMX 2.0.4** + **Tailwind CSS CDN** — sem build step, sem bundler
- Templates Go (`html/template`) embebidos no binário via `//go:embed`
- UI mobile-first: touch targets `py-3`, viewport meta, PWA meta tags, sticky header
- Polling do dashboard: `hx-trigger="every 2s"` no fragment de zonas
- Seletor de idioma: pills EN / PT no header do wizard e no dashboard

---

## 7. Internacionalização

- Língua detetada por prioridade: cookie `sprinkl_lang` → header `Accept-Language` → default **EN**
- Strings em `internal/i18n/locales/{en,pt}.json`, embebidas no binário
- Todas as templates recebem `{{.S}}` (map de strings) e `{{.Lang}}`
- Rota `GET /lang/{code}` — escreve o cookie e redireciona de volta

---

## 8. Configuração persistida

Ficheiro: `config.json` no working directory da app (sandbox OrbitOS)

```json
{
  "setup_done": true,
  "board": "keyestudio-4ch",
  "zones": [
    { "id": 1, "name": "Jardim", "channel": 1, "type": "sprinkler", "max_secs": 1800, "enabled": true }
  ],
  "schedules": [
    { "id": 1, "zone_id": 1, "days": [1,2,3,4,5], "start_time": "07:00", "dur_mins": 15, "enabled": true }
  ],
  "mqtt": {
    "enabled": false,
    "broker": "",
    "port": 1883,
    "prefix": "sprinkl",
    "username": "",
    "password": ""
  }
}
```

---

## 9. Scheduler

Ficheiro: `internal/scheduler/scheduler.go`

- Um programa por zona: cada `Schedule` tem `ZoneID`, `Days []int`, `StartTime` (HH:MM), `DurMins`, `Enabled`
- Goroutine com tick a cada 30s; compara weekday + HH:MM com os programas ativos
- `lastRun map[int]time.Time` previne disparo duplo na mesma janela de um minuto
- `SetEngine(eng)` — injeção tardia após wizard concluído ou arranque com setup feito
- `NextRunFor(sched)` — calcula a próxima data/hora de execução (até 7 dias à frente)
- UI acessível via `/schedule` com atalhos de dia rápidos (Todos / Dias úteis / Fim de semana)

---

## 10. Manifest OrbitOS

```json
{
  "package_id": "org.orbit-os.app.sprinkl",
  "name": "Sprinkl",
  "permissions": ["SystemService/*", "GpioService/*", "AppHubService/*"]
}
```
