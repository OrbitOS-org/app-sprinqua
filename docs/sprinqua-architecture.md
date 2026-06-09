# Sprinqua — Arquitetura e Estado de Implementação

> Atualizado: 2026-06-09

---

## Estado geral

| Módulo | Estado |
|---|---|
| Setup Wizard (3 passos) | ✅ Implementado |
| Dashboard + controlo manual | ✅ Implementado |
| i18n (EN / PT / DE / ES / FR / IT) | ✅ Implementado |
| Board registry (7 boards) | ✅ Implementado |
| Zone Engine (ON/OFF/Pulse + safety timer + MaxSecs) | ✅ Implementado |
| Modo zona exclusiva (só 1 relé ativo) | ✅ Implementado |
| Cores persistentes por canal (CHX) | ✅ Implementado |
| Formato de hora (24h / 12h) | ✅ Implementado |
| Agendamento (scheduler) + nome de programa | ✅ Implementado |
| Gráfico semanal de programas (Gantt) | ✅ Implementado |
| Histórico de ativações + gráfico 24h | ✅ Implementado |
| Histórico — entradas saltadas (Smart Watering) | ✅ Implementado |
| Settings page (separada do wizard) | ✅ Implementado |
| Smart Watering (Open-Meteo, mapa, threshold) | ✅ Implementado |
| MQTT — config guardada | ✅ Implementado |
| MQTT — cliente publicação/subscrição HA (active/passive) | ✅ Implementado |
| Modo Inverno (pausa scheduler, controlo manual disponível) | ✅ Implementado |
| Pulse duration configurável | ⬜ Não implementado (UI fixa em 300s; backend já aceita `?secs=`) |
| Filtros no histórico (por zona, por trigger) | ⬜ Não implementado |
| Estatísticas — total regado por zona/semana | ⬜ Não implementado |
| Smart Watering — ajuste proporcional de duração | ⬜ Não implementado |

---

## 1. Estrutura de ficheiros

```
cmd/sprinqua/
  main.go              — Entrypoint: flags, Gravity RT client, config load, engine init
  metadata.json        — Manifest OrbitOS (package_id, permissões)

internal/
  board/
    registry.go        — Boards suportadas e mapeamento de canais GPIO
  config/
    config.go          — Load/Save config.json; structs Config, Zone, Schedule, MQTT, SmartWatering
  history/
    history.go         — Store de ativações: Start/Stop/Skip/Recent/Clear, JSON persistence
  i18n/
    i18n.go            — Deteção de língua (cookie → Accept-Language → "en"), Supported(), Detect()
    locales/en.json    — Strings EN
    locales/pt.json    — Strings PT
    locales/de.json    — Strings DE
    locales/es.json    — Strings ES
    locales/fr.json    — Strings FR
    locales/it.json    — Strings IT
  mqtt/
    client.go          — Cliente MQTT (paho); publicação de estado, subscrição de comandos HA, auto-discovery
  scheduler/
    scheduler.go       — Goroutine de agendamento, NextRunFor, SetEngine, SetHistory, SetPaused
  weather/
    weather.go         — Fetch Open-Meteo (precipitação diária), cache 1h por localização
  zone/
    engine.go          — Controlo de relés, exclusive mode, safety timer, ActiveLow, estado das zonas
  web/
    server.go          — HTTP server, registo no AppHub, funcMap de templates, cookie lang
    handlers.go        — Todos os handlers HTTP
    templates/
      wizard.html         — Wizard completo + step1 (board) + seletor de idioma
      step2.html          — Configuração de zonas
      step3.html          — Teste de relés (conclui wizard)
      dashboard.html      — Dashboard completo (inclui banner Modo Inverno)
      zones_fragment.html — Fragment HTMX para polling/refresh de zonas
      schedule.html       — Página de programas
      schedule_list.html  — Fragment HTMX com lista + gráfico semanal
      schedule_form.html  — Página de criação/edição de programa (inclui campo Nome)
      history.html        — Histórico de ativações + gráfico 24h + entradas saltadas
      settings.html       — Settings: idioma, hora, Modo Inverno, zona exclusiva, MQTT, Smart Watering, hardware
```

---

## 2. OrbitOS / Gravity RT

- Ligação via `client.NewClientAuto(host)` — UDS no device, TCP+mTLS do laptop
- `AppHubManager.RegisterWebUI(addr, "/sprinqua")` — regista a UI no portal OrbitOS
- `GpioManager.SetDirection / SetLevel` — controlo de relés
- `SystemManager.GetHardwareModel()` — modelo do hardware mostrado no dashboard
- Config e estado guardados **apenas no working directory** (sandbox OrbitOS)
- Porta HTTP: **8083**

---

## 3. Setup Wizard

Três passos (step4 MQTT removido para Settings). Cada passo renderizado via HTMX swap em `#wizard-content`:

**Step 1 — Board**
- Dropdown com boards suportadas
- Badges CHX coloridos com update dinâmico via `hx-get="/setup/channels"` ao mudar a seleção
- Seletor de idioma (🌐 + dropdown) no canto superior direito

**Step 2 — Zonas**
- Uma zona por canal, com nome, tipo (aspersão / gotejamento / nebulização) e duração máxima em minutos
- Tipo default: **aspersão**
- Ao submeter, todos os pinos são inicializados como OUTPUT + OFF

**Step 3 — Testes + Conclusão**
- Botão por zona que ativa o relé durante 3 segundos
- Cliques repetidos no mesmo canal cancelam o pulse anterior (`context.WithCancel`)
- **Ao concluir**: grava config, inicializa Zone Engine, redireciona para `/setup` (Settings)

**Restart do wizard**: botão em Settings → POST `/setup/reset` → apaga zonas, programas e histórico (mantém preferências e MQTT), pede confirmação via `confirm()`.

---

## 4. Zone Engine

Ficheiro: `internal/zone/engine.go`

- Mapa `zones map[int]*entry` indexado pelo ID da zona
- `Init()` — inicializa todos os pinos como OUTPUT + OFF no arranque
- `TurnOn(id)` — liga relé, inicia safety timer; em **exclusive mode** desliga todas as outras zonas ativas primeiro (`turnOffLocked` para evitar deadlock)
- `TurnOff(id)` / `turnOffLocked(id)` — desliga relé, cancela safety timer
- `Pulse(id, secs)` — liga e desliga automaticamente após N segundos
- `SetExclusive(v bool)` — atualiza o modo em runtime (chamado ao guardar Settings)
- `States()` — snapshot ordenado do estado de todas as zonas
- `relayWrite(pin, on)` — abstrai ActiveLow: inverte nível GPIO quando `board.ActiveLow == true`

**Safety timer (MaxSecs)**
- Configurado por zona em `max_secs` (segundos); default 30 min se ≤ 0
- `TurnOn` lança goroutine que chama `TurnOff` automaticamente após `maxSecs` segundos
- Timer cancelado quando a zona é desligada manualmente ou substituída por novo `TurnOn`

**Exclusive Zone Mode** (default: ON)
- Guardado em `config.json` como `exclusive_mode` (`*bool`, nil = default true)
- `IsExclusiveMode()` — retorna true se nil ou *true
- Ao ativar uma nova zona, todas as outras são desligadas antes

---

## 5. Board Registry

Ficheiro: `internal/board/registry.go`

| ID | Nome | Canais | GPIOs | ActiveLow |
|---|---|---|---|---|
| `keyestudio-4ch` | Keyestudio RPI 4-Channel Relay | 4 | 26,20,21,16 | false |
| `seengreat-4ch` | Seengreat 4-CH Relay HAT | 4 | 6,13,19,26 | false |
| `bc-robotics-4ch` | BC Robotics 4-Channel Relay HAT | 4 | 5,6,13,22 | false |
| `waveshare-3ch` | Waveshare RPi 3-Channel Relay | 3 | 26,20,21 | false |
| `seengreat-3ch` | Seengreat 3-CH Relay HAT | 3 | — | false |
| `waveshare-pi0-6ch` | Waveshare RPi Zero 6-ch Relay | 6 | 5,6,13,16,19,20 | false |
| `waveshare-8ch` | Waveshare RPi 8-Channel Relay | 8 | 5,6,13,16,19,20,21,26 | **true** |
| `seengreat-8ch` | Seengreat 8-CH Relay Board | 8 | 6,13,19,26,12,… | false |

`ActiveLow: true` — GPIO LOW = relé ON, GPIO HIGH = relé OFF (ex: `waveshare-8ch`).

---

## 6. Web UI

- **HTMX 2.0.4** + **Tailwind CSS CDN** — sem build step, sem bundler
- Templates Go (`html/template`) embebidos no binário via `//go:embed`
- UI mobile-first: touch targets `py-3`, viewport meta, sticky header
- Navegação: header fixo com tabs (Zones · History · Schedule · ⚙ Settings), tab ativa em bold

**Routing**
| Rota | Handler |
|---|---|
| `GET /` | redirect → `/dashboard` ou `/setup/wizard` |
| `GET /dashboard` | dashboard com zonas |
| `GET /setup` | Settings page |
| `POST /setup/save` | guarda preferências |
| `POST /setup/reset` | reinicia wizard |
| `GET /setup/wizard` | wizard step1 |
| `GET /setup/channels` | fragment HTMX com badges CHX |
| `GET /history` | histórico + gráfico 24h |
| `GET /schedule` | lista de programas + Gantt |
| `GET /api/weather` | fragment HTMX com status meteo |
| `GET /lang/{code}` | muda idioma (cookie) |

**Polling e refresh de zonas**
- Polling a cada 2s via `hx-trigger="every 2s, zoneChanged from:body"`
- Após ON/OFF/Pulse: `hx-on::after-request` dispara `zoneChanged` → refresh imediato

**Cores de canal (CHX)**
- Paleta de 8 cores hex em `handlers.go` (`zoneColors`)
- Função de template `zoneColor(idx int)` — ponto único de consulta
- Aplicada em: wizard, dashboard, schedule, history

---

## 7. Internacionalização

- Cookie: `sprinqua_lang`; prioridade: cookie → `Accept-Language` → default **EN**
- Strings em `internal/i18n/locales/{en,pt,de,es,fr,it}.json`, embebidas no binário
- Todos os templates recebem `{{.S}}` (map de strings), `{{.Lang}}` e `{{.TimeFormat}}`
- `Supported()` → `["en","pt","de","es","fr","it"]`
- `Detect()` percorre `Supported()` para cookie e prefixos do Accept-Language
- Mudança de idioma: dropdown em Settings (pills com bandeiras) e no wizard; páginas normais sem switcher

---

## 8. Scheduler

Ficheiro: `internal/scheduler/scheduler.go`

- Cada `Schedule` tem `ID`, `Name` (opcional), `ZoneID`, `Days []int`, `StartTime` (HH:MM), `DurMins`, `Enabled`
- Goroutine com tick a cada 30s; compara weekday + HH:MM com os programas ativos
- `lastRun map[int]time.Time` previne disparo duplo na mesma janela de um minuto
- `SetEngine(eng)` — injeção tardia após wizard concluído
- `SetHistory(hist)` — injeção do store de histórico
- `SetPaused(bool)` — pausa/retoma o scheduler em runtime
- `NextRunFor(sched)` — calcula a próxima data/hora de execução (até 7 dias à frente)
- **Smart Watering check**: antes de executar, verifica `weather.FetchToday(lat, lon)`; se `rain >= threshold` chama `hist.Skip()` e aborta

**Scheduler fica pausado quando:**
- MQTT está em modo `passive` (HA tem controlo total)
- **Modo Inverno** está ativo

---

## 9. Histórico de Ativações

Ficheiro: `internal/history/history.go`

- `Entry` — ID, ZoneID, ZoneName, Channel, Trigger (manual/schedule/pulse/mqtt), StartedAt, EndedAt, DurSecs, **Skipped bool**
- `Store` — mutex + slice de entradas + nextID; persiste em `history.json` (máx 500 entradas)
- `Start(zoneID, trigger)` — cria nova entrada; fecha automaticamente entrada aberta para a mesma zona
- `Stop(zoneID)` — fecha a entrada aberta, calcula DurSecs
- `Skip(zoneID, trigger)` — regista entrada instantânea `Skipped: true` (Smart Watering)
- `Recent(n)` — retorna até n entradas, mais recentes primeiro
- `Clear()` — limpa tudo e remove `history.json` (chamado no reset do wizard)

**Página `/history`**
- Gráfico 24h: uma linha por zona, barras coloridas (entradas saltadas excluídas do gráfico)
- Lista: entradas normais com badge CHX colorido, trigger, hora, duração
- Entradas saltadas: fundo cinzento, borda tracejada, badge CHX mantém cor da zona, label âmbar "🌦 Saltado · Rega Inteligente"

---

## 10. Smart Watering

### 10.1 Módulo meteorológico — `internal/weather/weather.go`

- API: **Open-Meteo** (gratuito, sem API key)
- `FetchToday(lat, lon) (*Result, error)` — fetcha `precipitation_sum` diária (previsão hoje), cache 1h

### 10.2 Módulo de ajuste — `internal/adjustment/adjustment.go`

- `Calc(sw SmartWateringConfig) float64` — devolve multiplicador [0.0–2.5]
- `"manual"` → `ManualPct / 100.0`
- `"monthly"` → `MonthlyPct[mesAtual] / 100.0` (default 100 se slot a zero)

### 10.3 Config — `SmartWateringConfig`

```go
Enabled         bool
Lat, Lon        float64
RainThresholdMM float64     // skip se chuva >= threshold; default 2mm
Method          string      // "" | "manual" | "monthly" | "zimmerman" | "eto"
ManualPct       float64     // 0–250
MonthlyPct      [12]float64 // Jan=0 … Dez=11; 0 = default 100
```

### 10.4 Integração no scheduler

- Por programa: `Schedule.SmartWatering bool` — opt-in por programa
- `runSchedule` aplica `adjustment.Calc(sw)` à duração quando `sched.SmartWatering && sw.Method != ""`
- Duração ajustada = 0 → `hist.Skip()` e abort (mesmo comportamento do skip por chuva)
- O skip por chuva (limiar) é independente e corre sempre antes do ajuste de duração

### 10.5 Settings UI

- Toggle global + mapa Leaflet/OSM
- Seletor de método: Skip only / Manual / Monthly *(Zimmerman e ETo em fases futuras)*
- Manual: campo de percentagem única
- Monthly: grelha 4×3 com os 12 meses
- Card de status HTMX (`GET /api/weather`)

---

## 10.6 Fase 2 — Zimmerman (planeada)

Heurística empírica: `Watering% = 100 + (T - BT)×4×WT + (30 - BH)×WH - 200×(P - BP)×WP`

Requer fetch de `temperature_2m_max/min`, `relative_humidity_2m_mean`, `precipitation_sum` do **dia anterior** (`past_days=1`). Parâmetros configuráveis: `BT` (°F), `BH` (%), `BP`, pesos `WT/WH/WP` (0–100%).

---

## 10.7 Fase 3 — ETo / Penman-Monteith (planeada)

`Watering% = ((ETo_ontem - Precip_ontem) ÷ ETo_baseline) × 100%`

O Open-Meteo já devolve `et0_fao_evapotranspiration` — não é necessário implementar FAO-56.

**ETo baseline** — média diária anual calculada a partir dos últimos 12 meses via Archive API.

**Decisões de implementação:**

| Decisão | Escolha |
|---|---|
| Quando calcular baseline | Automaticamente na 1ª vez que o utilizador seleciona ETo como método — goroutine em background |
| Fallback enquanto sem baseline | Zimmerman temporariamente, com banner de aviso na UI: *"ETo baseline ainda não calculado. A usar Zimmerman temporariamente."* Quando o baseline ficar pronto, o sistema muda para ETo sem intervenção |
| Onde guardar | Dentro de `smart_watering` em `config.json` |
| Recálculo manual | Botão "Recalcular baseline" nas definições ETo — útil após mudança de localização |

**Campos novos em `SmartWateringConfig`:**
```json
"eto_baseline": 0.112,
"eto_baseline_calculated_at": "2026-01-15"
```
`eto_baseline_calculated_at` permite mostrar no botão "Recalcular" quando foi o último cálculo.

**Campos adicionais necessários no Open-Meteo (dia anterior):**
`et0_fao_evapotranspiration`, `precipitation_sum`

**Campos adicionais necessários na config:**
- `Altitude float64` — necessário para a Archive API (parâmetro `elevation`)

---

## 11. Modo Inverno

Configurado em Settings (toggle azul ❄️).

- `WinterMode bool` em `config.json` (`"winter_mode"`)
- Ao ativar: `sched.SetPaused(true)` — nenhum programa é executado automaticamente
- **Controlo manual continua a funcionar** (ON/OFF/Pulse no dashboard)
- Banner azul ❄️ no dashboard enquanto ativo: *"Schedules are paused. Manual control is still available."*
- Aplicado no arranque: se `winter_mode: true` no `config.json`, scheduler arranca pausado
- Pausa combinada: scheduler pausa se `WinterMode || MQTT.IsPassive()`

---

## 12. MQTT

Ficheiro: `internal/mqtt/client.go`

- Biblioteca: **paho.mqtt.golang**
- `Connect(cfg, zones, engine, hist)` — liga ao broker, subscreve tópicos, publica estado inicial
- `Disconnect()` — desliga limpo, remove callback do engine
- `OnModeChange func(string)` — callback chamado quando HA envia comando de modo

**Dois modos de operação:**
- **Standalone (active)**: Sprinqua executa os seus próprios programas; HA recebe atualizações de estado
- **HA Managed (passive)**: programas internos pausados; HA tem controlo total via MQTT

**Auto-discovery Home Assistant**
- Publica config de `switch` para cada zona em `homeassistant/switch/<prefix>_zone_<id>/config`
- Tópicos de estado: `<prefix>/zone/<id>/state` — payload `ON` / `OFF`
- Tópicos de comando: `<prefix>/zone/<id>/set` — aceita `ON`, `OFF`
- Tópico de modo: `<prefix>/mode/set` — aceita `active`, `passive`

---

## 13. Gráfico semanal (Gantt)

Renderizado em `schedule_list.html` (atualiza com toggle/delete via HTMX).

- **Zoom adaptativo**: eixo X escala ao intervalo real dos programas ativos (± 30 min de buffer, mínimo 60 min)
- **Ruler dinâmico**: 5 marcas calculadas em Go e passadas ao template (`Ruler [5]string`)
- **Lane packing temporal**: programas não sobrepostos partilham a mesma linha
- **Legenda**: pills coloridos por zona
- **Tooltip**: ao passar o rato mostra `zona · hora · duração`

---

## 14. Configuração persistida

Ficheiro: `config.json` no working directory da app

```json
{
  "setup_done": true,
  "board": "waveshare-8ch",
  "time_format": "24h",
  "exclusive_mode": true,
  "winter_mode": false,
  "zones": [
    { "id": 1, "name": "Jardim", "channel": 1, "type": "sprinkler", "max_secs": 1800, "enabled": true }
  ],
  "schedules": [
    { "id": 1, "name": "Manhã", "zone_id": 1, "days": [1,2,3,4,5], "start_time": "07:00", "dur_mins": 15, "enabled": true }
  ],
  "mqtt": {
    "enabled": false,
    "broker": "",
    "port": 1883,
    "prefix": "sprinqua",
    "username": "",
    "password": "",
    "mode": "active"
  },
  "smart_watering": {
    "enabled": true,
    "lat": 38.7169,
    "lon": -9.1395,
    "rain_threshold_mm": 2.0
  }
}
```

---

## 15. Manifest OrbitOS

```json
{
  "package_id": "org.orbit-os.app.sprinqua",
  "name": "Sprinqua",
  "permissions": ["SystemService/*", "GpioService/*", "AppHubService/*"]
}
```

---

## O que falta implementar

| Módulo | Fase | Valor | Complexidade |
|---|---|---|---|
| Pulse duration configurável na UI (backend já suporta `?secs=`) | — | Médio | Baixa |
| Filtros no histórico (por zona, por trigger) | — | Médio | Baixa |
| Estatísticas — total regado por zona/semana | — | Médio | Média |
| Smart Watering — Zimmerman (temp + humidade + precip do dia anterior) | Fase 2 | Alto | Média |
| Smart Watering — ETo com baseline automático via Archive API | Fase 3 | Alto | Média |
