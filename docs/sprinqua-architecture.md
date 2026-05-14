# Sprinqua — Arquitetura e Estado de Implementação

> Atualizado: 2026-05-08

---

## Estado geral

| Módulo | Estado |
|---|---|
| Setup Wizard (3 passos) | ✅ Implementado |
| Dashboard + controlo manual | ✅ Implementado |
| i18n (EN / PT / DE / ES / FR / IT) | ✅ Implementado |
| Board registry (Keyestudio 4ch, Waveshare 8ch, Waveshare 3ch) | ✅ Implementado |
| Zone Engine (ON/OFF/Pulse + safety timer) | ✅ Implementado |
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
| MQTT — cliente publicação/subscrição HA | ✅ Implementado |
| MaxSecs enforcement no engine | ⬜ Não implementado |
| Pulse duration configurável | ⬜ Não implementado |
| Filtros e estatísticas no histórico | ⬜ Não implementado |

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
  scheduler/
    scheduler.go       — Goroutine de agendamento, NextRunFor, SetEngine, SetHistory
  weather/
    weather.go         — Fetch Open-Meteo (precipitação diária), cache 1h por localização
  zone/
    engine.go          — Controlo de relés, exclusive mode, safety timer, estado das zonas
  web/
    server.go          — HTTP server, registo no AppHub, funcMap de templates, cookie lang
    handlers.go        — Todos os handlers HTTP
    templates/
      wizard.html         — Wizard completo + step1 (board) + seletor de idioma
      step2.html          — Configuração de zonas
      step3.html          — Teste de relés (conclui wizard)
      step4.html          — (legado, não usado)
      dashboard.html      — Dashboard completo
      zones_fragment.html — Fragment HTMX para polling/refresh de zonas
      schedule.html       — Página de programas
      schedule_list.html  — Fragment HTMX com lista + gráfico semanal
      schedule_form.html  — Página de criação/edição de programa (inclui campo Nome)
      history.html        — Histórico de ativações + gráfico 24h + entradas saltadas
      settings.html       — Settings: idioma, hora, zona exclusiva, MQTT, Smart Watering, hardware
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
| `waveshare-8ch` | Waveshare RPi 8-Channel Relay | 8 | 5,6,13,16,19,20,21,26 | false |
| `waveshare-3ch` | Waveshare RPi 3-Channel Relay | 3 | 26,20,21 | false |

`ActiveLow: false` — GPIO HIGH = relé ON, GPIO LOW = relé OFF.

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
- `NextRunFor(sched)` — calcula a próxima data/hora de execução (até 7 dias à frente)
- **Smart Watering check**: antes de executar, verifica `weather.FetchToday(lat, lon)`; se `rain >= threshold` chama `hist.Skip()` e aborta

---

## 9. Histórico de Ativações

Ficheiro: `internal/history/history.go`

- `Entry` — ID, ZoneID, ZoneName, Channel, Trigger (manual/schedule/pulse), StartedAt, EndedAt, DurSecs, **Skipped bool**
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

Ficheiro: `internal/weather/weather.go`

- API: **Open-Meteo** (gratuito, sem API key) — `precipitation_sum` diária
- Cache em memória: 1 hora por coordenada
- `FetchToday(lat, lon) (*Result, error)` — retorna `RainMM float64`

**Config** (`SmartWateringConfig` em `config.go`):
- `Enabled bool`
- `Lat`, `Lon float64`
- `RainThresholdMM float64` (default: 2mm se 0)
- `EffectiveThreshold()` — retorna 2.0 se não configurado

**Settings UI**
- Toggle + mapa Leaflet/OSM (clicar no mapa define lat/lon)
- Campos lat/lon + threshold editáveis manualmente
- Card de status HTMX (`GET /api/weather`): ☀️ verde (permitido) ou 🌧️ âmbar (saltado), com mm previstos e limiar; botão ↻ de refresh

---

## 11. Gráfico semanal (Gantt)

Renderizado em `schedule_list.html` (atualiza com toggle/delete via HTMX).

- **Zoom adaptativo**: eixo X escala ao intervalo real dos programas ativos (± 30 min de buffer, mínimo 60 min)
- **Ruler dinâmico**: 5 marcas calculadas em Go e passadas ao template (`Ruler [5]string`)
- **Lane packing temporal**: programas não sobrepostos partilham a mesma linha
- **Legenda**: pills coloridos por zona
- **Tooltip**: ao passar o rato mostra `zona · hora · duração`

---

## 12. Configuração persistida

Ficheiro: `config.json` no working directory da app

```json
{
  "setup_done": true,
  "board": "waveshare-8ch",
  "time_format": "24h",
  "exclusive_mode": true,
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
    "password": ""
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

## 13. Manifest OrbitOS

```json
{
  "package_id": "org.orbit-os.app.sprinqua",
  "name": "Sprinqua",
  "permissions": ["SystemService/*", "GpioService/*", "AppHubService/*"]
}
```

---

## O que falta implementar

| Módulo | Valor | Complexidade |
|---|---|---|
| MQTT cliente — publicar estado das zonas no Home Assistant | Alto | Alta |
| MaxSecs enforcement — auto-off em ativações manuais | Médio | Baixa |
| Pulse duration configurável (atualmente fixo a 5 min) | Médio | Baixa |
| Filtros no histórico (por zona, por trigger) | Médio | Baixa |
| Estatísticas — total regado por zona/semana | Médio | Média |
| Smart Watering — ajuste proporcional de duração (em vez de skip total) | Médio | Média |
| Smart Watering — ET / temperatura / vento (dados adicionais Open-Meteo) | Baixo | Média |
