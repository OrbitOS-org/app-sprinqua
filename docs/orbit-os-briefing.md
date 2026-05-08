# Orbit OS — Briefing Completo
> Documento de contexto para criação do post técnico (Hacker News / Dev.to)

---

## 1. O Projeto

**Orbit OS** é um sistema operativo para dispositivos embedded Linux e IoT, construído por uma equipa de 3 pessoas. O objetivo é ser "a plataforma que falta" para Linux embarcado — eliminando a necessidade de cada equipa reinventar a mesma infraestrutura (GPIO, OTA, remote access, networking, AI).

- Website: https://www.orbit-os.org
- Store: https://store.orbit-os.org
- Forum: https://forum.orbit-os.org
- Estado: Pre-launch público. Community Edition + SDK Go a lançar no próximo mês.

---

## 2. A Diferença Core vs Balena

A Balena usa **Docker/containers** como unidade de deployment. O Orbit OS **não usa Docker** — tem um runtime nativo próprio chamado **Gravity RT** e um formato de pacotes próprio `.orb`.

| | BalenaOS | Orbit OS |
|---|---|---|
| Runtime | Docker (BalenaEngine) | Gravity RT (nativo) |
| Pacotes | Imagens Docker | `.orb` (como APK Android) |
| Footprint | 512MB+ RAM recomendado | Viável em 128-256MB RAM |
| AI | Add-on | First-class (TFLite incluído) |
| Maturidade | Produção, anos de uso | Pre-launch |

---

## 3. Arquitetura Gravity RT — Detalhes Técnicos

### Stack completo

```
Hardware
└── Linux (distro base — corre sobre Linux existente)
    └── Gravity RT
        ├── System Server (.bin Go) — sempre ativo, leve
        ├── gRPC/UDS — comunicação com apps internas .orb
        ├── gRPC/TCP mTLS — comunicação com tools externas
        ├── JVM — lazy load, só arranca com .orb Java
        ├── Python VM — lazy load, só arranca com .orb Python
        └── TFLite (C++) — inferência AI, lazy load
```

### System Server
- Escrito em **Go** — binário estático, footprint mínimo, sempre ativo
- É o gestor central (process parent) de todos os `.orb`
- Se uma app crasha, o System Server relança automaticamente
- Gere ciclo de vida: start, stop, restart, update

### Comunicação — duas APIs gRPC distintas
- **UDS (Unix Domain Socket)** — para apps `.orb` internas. Só acessível localmente no device. Apps não escapam do device.
- **TCP mTLS** — para tools externas (SDK, CLI). Autenticação mútua obrigatória. Sem certificado válido = sem ligação.

### Runtimes — Lazy Loading
- JVM, Python VM e TFLite **não arrancam no boot**
- Só são iniciados quando existe um `.orb` que os precisa
- Crítico para devices com RAM limitada

### TFLite em C++
- Incluído por default no OS
- Acesso direto a NEON/hardware acceleration em ARM
- Zero overhead de runtime adicional
- Explica as apps de Edge AI na Store

### Lifecycle das Apps

Cada `.orb` segue um ciclo de vida controlado pelo System Server:
- **Start** — lançamento controlado pelo Gravity RT
- **Stop** — paragem limpa
- **Restart automático** — em caso de crash
- **Shutdown gracioso** — signal-based

O System Server atua como process supervisor:
- Deteta crash loops
- Políticas de restart configuráveis (futuro)
- O systemd monitoriza o processo boot raiz — se cair, o systemd levanta

### Observabilidade

**Logs — LogD**
- Sistema centralizado via LogD — arranca com o runtime no boot
- Logs estruturados: tag + nível + timestamp
- Apps enviam logs por UDS para o LogD
- API de subscrição gRPC — developer faz subscribe e recebe logs em streaming real-time
- Filtros por tag e nível de log (estilo logcat Android)

### Versionamento da API gRPC

A API do Gravity RT é definida via `.proto` e segue versionamento explícito:
- Compatibilidade backward sempre que possível
- Novas funcionalidades adicionadas sem breaking changes
- Breaking changes versionadas por namespace

Permite evolução contínua do runtime sem quebrar apps existentes.

---

## 4. Formato .orb

- Equivalente ao `.apk` do Android
- Contém: código executável + manifest + recursos + assinatura criptográfica
- O **manifest** declara: linguagem (Go, Python, Java, C++), permissões, limites de recursos (CPU, RAM, IO)
- O Gravity RT lê o manifest e sabe como lançar cada app corretamente
- **Todos os .orb são assinados** — pacotes sem assinatura válida são rejeitados

### Dois domínios de confiança
- **Store-signed** — produção, fleet deployment, updates automáticos
- **Dev-signed** — desenvolvimento local, debuggável, sem fleet deployment

### Estrutura real de um .orb — Exemplo: Face Recognition (6MB)

```
.
├── bin
│   └── face_recognition_client      ← binário nativo compilado
├── data
│   ├── blaze_face_full_range.tflite ← modelo deteção de faces
│   ├── face_landmark.tflite         ← modelo landmarks faciais
│   └── mobilefacenet.tflite         ← modelo reconhecimento/identificação
├── icon.svg                         ← ícone da app na Store
├── lib                              ← bibliotecas dependentes
├── manifest.json                    ← declaração de permissões, runtime, limites
└── META-INF
    ├── CERT.crt                     ← certificado público
    ├── CERT.SF                      ← assinatura dos ficheiros
    ├── CERT.sig                     ← assinatura digital do pacote
    └── MANIFEST.MF                  ← inventário de ficheiros e hashes

4 directories, 10 files — 6MB total
```

**O que isto demonstra:**

- **6MB para uma app completa de reconhecimento facial com 3 modelos TFLite** — incluindo deteção, landmarks e identificação
- Os modelos TFLite estão dentro do `.orb` — a app é self-contained
- O TFLite runtime está no OS — não está incluído no `.orb`, daí o tamanho reduzido
- Estrutura de assinatura no `META-INF` — idêntica ao modelo APK Android, familiar para qualquer developer mobile
- `manifest.json` declara permissões, runtime e limites — Gravity RT lê antes de executar
- Binário nativo compilado em `bin/` — zero overhead de interpretação

### manifest.json real — Face Recognition

```json
{
  "package_id": "org.orbit-os.service.face-recognition",
  "version": "0.3.0",
  "name": "Edge AI - Face Recognition",
  "description": "Edge AI face recognition service for OrbitOS devices",
  "type": "binary",
  "architecture": "arm64",
  "entry_point": "face_recognition_client",
  "build_date": "2026-04-21T11:51:13Z",
  "git_commit": "d1d9709",
  "permissions": [
    "SystemService/*",
    "CameraService/*",
    "AiService/*",
    "AppHubService/*"
  ]
}
```

**O que cada campo comunica:**

- `package_id` — namespace reverso estilo Android (`org.orbit-os.service.face-recognition`) — convencional e familiar
- `type: binary` — Gravity RT sabe que é um binário nativo compilado, não Python nem JVM
- `architecture: arm64` — pacote específico para ARM64 (RPi 3/4/5)
- `entry_point` — o Gravity RT sabe exatamente o que executar sem ambiguidade
- `git_commit` — rastreabilidade total — qualquer build é reproduzível
- `permissions` — apenas 4 permissões explícitas:
  - `SystemService/*` — acesso a serviços base do sistema
  - `CameraService/*` — acesso à câmara
  - `AiService/*` — acesso ao TFLite runtime
  - `AppHubService/*` — comunicação com a Store

**O Gravity RT lê este manifest e só expõe estas 4 APIs à app — nada mais.**

Sem acesso à rede. Sem acesso a GPIO. Sem acesso a ficheiros fora do `/data/orb/<app-id>/`. Princípio do menor privilégio aplicado nativamente.

### Persistência de Dados

Cada `.orb` tem um diretório de dados persistente exclusivo:
```
/data/orb/<app-id>/
```
- Sobrevive a restarts da app
- Sobrevive a OTA updates do runtime
- Isolado por aplicação — uma app não acede aos dados de outra
- Quotas de armazenamento configuráveis por política (futuro)

**O runtime nunca apaga dados automaticamente — controlo total do developer.**

### Permissões

O manifest do `.orb` declara explicitamente todas as permissões necessárias. Modelo inspirado no Android — permissões explícitas, nunca implícitas.

**Exemplos de permissões:**
- GPIO access
- Network access
- I²C, SPI, UART
- AI Service (TFLite)
- BLE
- Firewall
- Package Manager

O Gravity RT no arranque de cada `.orb`:
1. Lê permissões declaradas no manifest
2. Valida contra políticas do device
3. Só expõe as APIs permitidas à app — sem acesso a nada mais

---

## 5. Segurança

### Validação de execução

Antes de executar qualquer `.orb`, o Gravity RT:
1. Verifica a assinatura criptográfica
2. Valida a chain de certificados
3. Classifica o domínio de confiança
4. Aplica políticas do device
5. Atribui sandbox se necessário

mTLS para conexões externas = autenticação mútua obrigatória. Sem portas abertas, sem confiança implícita.

### Autenticação TCP — dois modos

O modelo de autenticação para ligações TCP externas adapta-se ao contexto:

**Modo Produção (Dev Mode OFF):**
```
Certificado mTLS do SDK oficial
+ user/pass
= acesso ao device
```

**Modo Desenvolvimento (Dev Mode ON):**
```
Certificado mTLS do SDK oficial
= acesso ao device (sem user/pass)
```

### O certificado mTLS do SDK

O SDK oficial traz um certificado mTLS da cadeia oficial Orbit OS. Este certificado é usado pela CLI e por qualquer desenvolvimento externo que se conecte ao device via TCP.

Em Dev Mode, este certificado é suficiente — sem necessidade de user/pass. Isto elimina fricção durante o desenvolvimento sem comprometer a cadeia de confiança criptográfica.

### Dev Mode — decisão explícita e informada

- O Dev Mode é ativado manualmente nos Settings (estilo Android)
- O utilizador é **explicitamente informado** da implicação de segurança ao ativar
- Em produção o Dev Mode está sempre OFF — a camada de user/pass volta automaticamente
- É o mesmo modelo do Android USB Debugging — decisão consciente, nunca silenciosa

### Filosofia de segurança

> "Zero compromise on production security. Zero friction in development."

O contexto determina o nível de segurança — não existe um compromisso forçado entre os dois. Em desenvolvimento, máxima fluidez. Em produção, máxima proteção.

---

## 6. Developer Mode & Real-Time Remote Development

### O paradigma que muda tudo

O Orbit OS introduz um modelo de desenvolvimento completamente novo para embedded — **todo o desenvolvimento acontece no PC, contra hardware real, em tempo real.**

**Fluxo tradicional em embedded (lento, frustrante):**
```
Escreves código → Compilas → Fazes deploy → Corres → Vês o erro → Repetes
```

**Fluxo com Orbit OS (tempo real):**
```
Laptop
├── Escreves código no teu editor
├── SDK conecta via gRPC/mTLS ao device real
├── Código corre diretamente no hardware
├── Logs e resultados chegam ao laptop em tempo real
├── Iteras — mudas código, corres de novo instantaneamente
├── Testes passam, comportamento validado no hardware real
└── Só ENTÃO generates o .orb e publicas na Store
```

**O device é um target de execução remoto em tempo real — não um destino de deploy.**

### Porquê isto é revolucionário

É a mesma revolução que aconteceu noutros domínios:
- **Web** — browser DevTools, hot reload
- **Mobile** — React Native live reload
- **Cloud** — remote debugging, live logs

Mas **nunca tinha acontecido em embedded de forma nativa e integrada.** Sempre foi compile → flash → pray.

### Impacto na adoção

Remove a maior barreira de entrada para developers vindos do mundo web/cloud que querem trabalhar com hardware. O embedded sempre pareceu hostil porque o ciclo de feedback era lento. Com o Orbit OS o ciclo é **imediato** — abre o mercado a uma audiência muito maior do que developers embedded tradicionais.

### Detalhes técnicos
- Ativado nos Settings (estilo Android)
- Usa a **API gRPC externa** — permite chamar a API do device a partir de fora
- O SDK envia `.orb` gerados via gRPC em chunks (PackageService → método InstallPackage)
- **Não há SSH** — não é necessário
- Sem packaging, sem ciclo de deploy durante o desenvolvimento

---

## 7. OTA Updates — Como Funcionam

### Tecnologia base: OSTree

O Orbit OS usa **OSTree** para gerir updates do runtime — a mesma tecnologia usada no ChromeOS, Fedora Silverblue e Automotive Grade Linux. Domínios onde um update falhado é inaceitável.

O OSTree mantém a **pasta system** onde reside a base do Gravity RT, garantindo:
- **Imutabilidade** — nenhuma app ou processo pode corromper o sistema base
- **Atomicidade** — o update aplica completamente ou não aplica. Nunca estado intermédio
- **Rollback instantâneo** — se algo correr mal, o snapshot anterior está intacto
- **Delta updates** — suporte planeado para o futuro, já suportado nativamente pelo OSTree

### Quem gera os .orbit

Os ficheiros `.orbit` são **sempre gerados pela equipa Orbit OS** e colocados na Store. Não é possível submeter `.orbit` externos — garante integridade e confiança do sistema base.

### Fluxo completo de um OTA update

```
Orbit OS Store
└── Nova versão do Gravity RT disponível
    └── Se device tem Auto OTA Update enabled
        └── Store envia .orbit para o device automaticamente
            └── gRPC call: SystemUpdate(OtaFile)
                └── Gravity RT recebe o .orbit
                    └── OSTree instala no repo interno
                        └── Mata o processo boot do Gravity RT
                            └── Toda a cadeia de filhos morre com ele
                                └── (todas as .orb apps terminam)
                                    └── Processo boot relança
                                        └── Nova versão ativa
                                            └── Apps relançam automaticamente
```

### Reboot "soft" — sem reiniciar o Linux

O update **não reinicia o sistema operativo Linux base** — apenas o Gravity RT e os seus processos filhos são terminados e relançados. É significativamente mais rápido do que um reboot completo do device.

O mecanismo é elegante: como o processo boot é o pai de toda a cadeia, matar o boot mata todos os filhos (apps .orb) automaticamente. Relancar o boot restaura tudo na nova versão.

### Diferença entre .orb e .orbit

| | `.orb` | `.orbit` |
|---|---|---|
| **O quê** | App individual | Runtime completo |
| **Conteúdo** | Código + manifest + recursos | Gravity RT + base system |
| **Gerado por** | Developers / equipa | Equipa Orbit OS apenas |
| **Distribuído via** | Store (apps) | Store (OTA system update) |
| **Impacto** | Só a app afetada | Todo o runtime reinicia |
| **Equivalente Android** | APK | OS update |

Separação clara entre aplicação e sistema — uma app com bug não pode comprometer o runtime.

### Auto OTA Update

Configurável por device na Store:
- **Enabled** — Store envia updates automaticamente quando disponíveis
- **Disabled** — update manual, controlado pelo operador

### No futuro
Hot-swap sem matar processos — transferência de estado das apps para a nova versão sem downtime. Complexidade adicional, mas o modelo atual já é adequado para produção.

---

## 8. SDK & Ecossistema de Clientes

### SDKs disponíveis

| Linguagem | Estado |
|---|---|
| Go | Disponível no lançamento |
| Kotlin | Disponível (usado na app mobile oficial) |
| Java | Beta em breve |
| Python | Planeado Q3 |
| C++ | Planeado Q4 |

### Como os SDKs são gerados

Os SDKs são gerados automaticamente a partir dos ficheiros **`.proto`** do gRPC. Isto significa:
- Consistência garantida entre todas as linguagens — mesma API, mesmo comportamento
- Adicionar suporte a uma nova linguagem é gerar os stubs gRPC
- Qualquer linguagem com suporte gRPC pode ter um SDK Orbit OS

### CLI Tool

Vem incluída com o SDK Go. Em modo Developer fala diretamente com a API gRPC externa do device.

**Funcionalidades:**
- Instalar `.orb` packages no device
- Remover packages
- Listar packages instalados
- Interagir com o device localmente

### App Mobile Android (Oficial)

Já existe uma app Android oficial com dois modos:

**1. Store online**
- Acesso à Orbit OS Store
- Instalar e gerir apps nos devices

**2. Controlo local de devices**
- Liga ao device via IP na rede local
- Autenticação: IP + user/pass + mTLS
- Três barreiras de segurança independentes:
  ```
  Acesso à rede local (IP)
  + Credenciais (user/pass)
  + mTLS (certificado)
  = Acesso ao device
  ```
- Permite fazer tudo o que a API expõe — instalar, remover, controlar, monitorizar

### SDKs Mobile para terceiros

Os `.proto` permitem a qualquer developer gerar o seu próprio SDK Kotlin/Swift e criar apps mobile personalizadas para interagir com devices Orbit OS. Exemplos de casos de uso:
- App para monitorizar uma estufa inteligente
- App para gerir câmaras de segurança edge
- App de controlo industrial personalizada
- Qualquer interface mobile para qualquer device Orbit OS

**A app oficial é a referência — o ecossistema pode crescer organicamente.**

### App-to-App Communication

Por agora não existe API nativa de comunicação entre `.orb`. Apps podem criar conexões TCP entre si se necessário, mas não é uma prioridade atual. Possível adição futura como API nativa no Gravity RT.

---

## 9. Store — Estado Atual

A Store já está online e funcional em https://store.orbit-os.org

### Apps disponíveis
- **IOFlow** — controlo e teste de periféricos (★ 4.0)
- **Settings** — configurações do device
- **Edge AI - Face Recognition** — visão computacional
- **Edge AI – Smart Image Detection** (★ 4.0)
- **MCP Server - Powering AI connections** (★ 5.0) — integração com AI via Model Context Protocol
- **RPI 4-Channel Relay Controller - Keyestudio** (★ 5.0) — controlo de relés físicos
- **Mochi MQTT Broker** (★ 4.0) — broker MQTT para IoT
- **PortFlux** — routing de tráfego
- **WireShield VPN** — segurança de rede
- **Serial Console** — debug
- **Webcam Preview** — acesso a câmara

> Nota: Apps criadas pela equipa como exemplos para demonstrar o ecossistema. Ratings reais de utilizadores.

### Funcionalidades da Store
- 1-click install
- Auto-update de apps
- OTA update automático do Gravity RT
- Portal de developer para submissão de apps
- Gestão de devices por conta

### Fleet Management — Deploy Multi-Device

Modelo semelhante ao Google Play Store para enterprise:

```
Botão "Install" na Store
└── Dropdown de devices da conta
    ├── Device 1 ✅ (já tem esta versão — desativado)
    ├── Device 2 ☐ (disponível para instalar)
    ├── Device 3 ☐ (disponível para instalar)
    └── "Select All" — instala em todos os elegíveis simultaneamente
```

- Deploy simultâneo para múltiplos devices
- Devices com a versão já instalada ficam desativados — evita reinstalações desnecessárias
- Controlo granular — todos os devices ou seleção individual

---

## 10. Hardware Suportado

- Raspberry Pi 2–5
- Arduino UNO Q
- BeagleBone Black / AI
- NVIDIA Jetson series
- Rock Pi 4 / 5
- Odroid N2+
- Banana Pi family
- Dispositivos x86_64

**Arquiteturas:** ARM64, ARM32, x86_64

---

## 11. Posicionamento Estratégico

### Diferenciadores reais
1. **Sem Docker** — footprint radical menor para edge real
2. **Real-time remote development** — desenvolves no PC contra hardware real, sem ciclos de deploy
3. **Edge AI first-class** — TFLite incluído por default, não como add-on
4. **MCP Server** — posicionamento no ecossistema AI moderno (Anthropic Model Context Protocol)
5. **Stack completo** — OS + Runtime + Store + OTA + SDK + CLI + App Mobile numa solução integrada
6. **SDK gerado dos .proto** — qualquer linguagem gRPC pode ter SDK, ecossistema mobile aberto a terceiros
7. **Modelo Android** — familiar para developers, prático para embedded

### Mercado alvo
- Dispositivos muito constrangidos (128-256MB RAM) onde Docker é impraticável
- IoT industrial com requisitos de segurança elevados
- Edge AI — câmaras inteligentes, visão computacional, inferência local

### Concorrência principal
- **BalenaOS** — containers Docker, mais madura, mais devices, mas mais pesada
- **Mender** — focado em OTA, não em runtime completo
- **Yocto** — muito mais baixo nível, sem gestão de apps

---

## 12. Detalhes Técnicos Adicionais

### Métricas Reais (RPi 4, 1.8GB RAM)

Gravity RT em execução normal:

```
CPU Usage:        2.3%
Memory Usage:     12.4%   →   139.1 MB / 1.80 GB
SoC Thermal:      50.2°C
Uptime:           181,955s (~2 dias)
```

**Tamanhos:**
- Instalador `.run`: **25 MB** (instala dependências via apt na primeira execução — JVM, etc. — só uma vez)
- OTA update `.orbit` full: **21 MB**

**O que 139MB significa na prática:**
- Runtime completo com System Server, LogD, e todos os serviços base
- Deixa ~1.6GB livres para apps no RPi 4
- Em devices com 512MB RAM, o runtime ocupa ~27% — viável
- Significativamente menor do que qualquer solução baseada em Docker

### Instalador
- Ficheiro `.run` único — **25 MB**
- Basta executá-lo no device — monta as pastas e cria o runtime automaticamente
- Instala dependências via apt na primeira execução (JVM, etc.) — só acontece uma vez
- O utilizador precisa de saber o IP do device (necessário para transferir o `.run` e posteriormente para se ligar)

### Networking & Descoberta
- Sem discovery automático — o utilizador conhece o IP do device
- Ligação via IP direto na rede local

### Ownership
- Sempre ownership único — Edge devices não têm necessidade de multi-tenancy

### Sistema de Logs (LogD)
- Similar ao modelo Android `logd` / `logcat`
- O LogD arranca com o runtime no boot
- A lib de logs dos SDKs envia logs por UDS para o LogD
- Logs têm tag, nível de log, etc.
- Developer vê logs em tempo real durante desenvolvimento

### Hardware APIs — tudo gRPC
Todas as APIs de hardware são expostas via gRPC pelo Gravity RT:
- GPIO, PWM, I²C, SPI, UART
- WiFi, Ethernet, Bluetooth (BLE)
- AI (TFLite)
- Firewall
- Package Manager
- E qualquer hardware custom via customização enterprise

---

## 13. Modelo de Negócio

### Três pilares de receita

**1. Customização Enterprise (B2B)**
Empresas com hardware especial — sensores proprietários, periféricos custom — precisam de interfaces internas do Gravity RT desenvolvidas à medida para expor APIs específicas. A equipa Orbit OS desenvolve essas integrações.
- O Orbit OS é a montra pública
- O serviço de customização é a receita

**2. White Label OS**
O Gravity RT como plataforma privada para clientes empresariais:
- Ecossistema completamente separado e isolado
- Package format próprio do cliente
- Certificados proprietários
- Store privada — incompatível com o ecossistema Orbit público
- Essentially "Gravity RT as a Platform"

Qualquer empresa pode ter o seu próprio OS de edge, com a sua própria Store, o seu próprio ecossistema — construído sobre o Gravity RT.

**3. Hardware Marketplace**
A equipa tem capacidade de desenhar hardware. Modelo de dois tiers para o mesmo produto:

```
Mesma APP na Store
├── DIY tier
│   └── Hobista compra RPi + periféricos
│       └── Instala a app
│           └── Funciona
└── Premium tier
    └── Device feito à medida
        └── Em caixa profissional
            └── Plug-and-play
                └── Vendido no Marketplace
```

**Exemplos de produtos concretos:**
- **Sistema de rega inteligente** — RPi + relés + app DIY **vs** device em caixa plug-and-play
- **Controlo de acessos facial** — câmara + RPi + app de reconhecimento facial com AI local **vs** device dedicado com caixa profissional pronto a instalar

### O Flywheel

```
Apps na Store atraem utilizadores
└── Utilizadores DIY validam o produto no mercado
    └── Marketplace vende versão premium certificada
        └── Receita financia mais apps e hardware
            └── Mais apps atraem mais utilizadores
```

### Proteção competitiva
O White Label protege o modelo a longo prazo — mesmo que um concorrente copie a ideia, não tem o ecossistema, a comunidade, nem a capacidade de hardware integrada.

---

## 14. Estratégia de Hardware Marketplace

A equipa tem capacidade própria de desenhar hardware. O modelo assenta em **plataformas de hardware** — o mesmo hardware base serve múltiplos produtos com software diferente, maximizando margem e minimizando complexidade de produção.

Para cada produto existe sempre um tier DIY e um tier Premium:
- **DIY** — hobista monta com RPi + periféricos + app da Store
- **Premium** — device em caixa profissional, plug-and-play, vendido no Marketplace

---

### Hardware 1 — Câmara + Processamento AI Local

```
Mesmo hardware base
├── App reconhecimento facial → controlo de acessos pessoas
└── App reconhecimento matrículas → controlo de acessos viaturas
```

Dois produtos, um hardware. AI corre localmente — sem cloud, sem subscrição, sem latência. Privacidade total.

---

### Hardware 2 — Pi Zero + Relay

```
Hardware simples e barato
└── Abertura de portões
    └── Integração Home Assistant nativa
```

Pi Zero mantém o custo final muito acessível. A comunidade Home Assistant é massiva — centenas de milhares de utilizadores que partilham projetos e têm apetência para hardware bem feito.

---

### Hardware 3 — Controlador 6 Canais Relay

```
Mesmo hardware base do sistema de rega
├── Sistema de rega inteligente
├── Controlador Modbus RTU (RS485)
├── Controlador Modbus TCP
└── Controlador MQTT
```

Quatro produtos, um hardware. Modbus é o protocolo industrial mais usado no mundo — abre o mercado industrial imediatamente. Cada protocolo é uma app diferente na Store.

---

### Hardware 4 — Monitor de Energia com Edge AI

```
2x PZEM (ou medidor equivalente)
├── Fase 1 — Medição (disponível no lançamento)
│   ├── Medição de consumo monofásico
│   ├── Medição de produção solar
│   ├── Balanço consumo vs produção em tempo real
│   └── Integração Home Assistant nativa
└── Fase 2 — Edge AI (via OTA update)
    ├── Deteção de anomalias de consumo
    ├── Deteção de harmónicos e micro-cortes
    ├── Previsão de falhas em equipamentos
    ├── Deteção de picos anormais
    └── Alertas inteligentes locais
```

**Casos de uso AI concretos:**
- *"O teu frigorífico está a consumir 40% acima do normal — possível avaria"*
- *"Detetado padrão de micro-cortes — problema na instalação elétrica"*
- *"Produção solar abaixo do esperado para as condições atuais"*

**O que torna este produto único:**

A AI corre **localmente no device** — sem cloud, sem subscrição, sem dados a sair de casa. Para a comunidade HA isso é um argumento de venda crítico — são muito sensíveis a privacidade e dependência de cloud.

**O modelo Tesla aplicado a hardware IoT:**
O device que o cliente compra hoje para medir consumo, amanhã via OTA update passa a ter AI de deteção de problemas. Sem comprar hardware novo. O produto melhora depois de comprado — valor percebido enorme.

**Expansão futura:**
- Versão trifásica
- Integração com tarifas dinâmicas de eletricidade
- Dashboard de produção/consumo em tempo real

---

### O Flywheel do Marketplace

```
Apps na Store → validação DIY pela comunidade
└── Comunidade partilha projetos (Reddit, HA forums, YouTube)
    └── Visibilidade orgânica
        └── Marketplace vende versão Premium em caixa
            └── Receita → mais hardware → mais apps
                └── Atrai clientes White Label enterprise
```

---

## 15. Plano de Lançamento

1. **Próximo mês** — instalador público + SDK Go
2. **Q3** — SDK Python
3. **Q4** — SDK C++
4. **Paralelo** — atrair hardware makers e developers para a Store

---

## 16. Narrativa para o Post Técnico

**Hook:** *"Every embedded team rebuilds the same infrastructure. We decided to fix that — without Docker."*

**Estrutura sugerida para Hacker News (Show HN):**
1. O problema — reinvenção constante de infraestrutura em embedded
2. Porquê não Docker — footprint, overhead, inadequação para edge real
3. A solução — analogia Android, Gravity RT, .orb
4. Arquitetura interna — gRPC/UDS/mTLS, lazy loading, System Server em Go
5. **Real-time remote development** — desenvolves no PC, corres no hardware real, só no fim generates o .orb
6. **OTA com OSTree** — updates atómicos, rollback instantâneo, reboot soft sem reiniciar Linux
7. **SDK gerado dos .proto** — CLI, app Android oficial, SDKs mobile para terceiros
8. Edge AI first-class — TFLite por default
9. Stack completo — Store, auto OTA, 1-click install
10. Call to action — SDK Go a lançar, beta testers, Store

**Títulos sugeridos:**
- *"Show HN: Orbit OS – an Android-like runtime for embedded Linux, without Docker"*
- *"Show HN: Orbit OS – develop on your laptop, run on real hardware, no Docker"*
- *"Show HN: We built an edge OS where you develop in real-time against physical hardware"*

---

*Documento criado com base na conversa de análise do projeto Orbit OS — Abril 2026*
