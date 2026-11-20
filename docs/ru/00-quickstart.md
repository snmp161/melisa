# Быстрый старт

От нуля до диагноза за 2 минуты.

## 1. Установка

```bash
# Linux amd64 (для ARM64 используйте melisa-linux-arm64)
curl -fSLO https://github.com/snmp161/melisa/releases/latest/download/melisa-linux-amd64
chmod +x melisa-linux-amd64
sudo mv melisa-linux-amd64 /usr/local/bin/melisa

# Или сборка из исходников
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o melisa ./cmd/melisa/
sudo mv melisa /usr/local/bin/
```

Готовые Linux-пакеты доступны в [GitHub Releases](https://github.com/snmp161/melisa/releases): DEB для Debian/Ubuntu и RPM для AlmaLinux/Rocky Linux/RHEL, оба варианта для amd64 и arm64. Пакеты устанавливают бинарник в `/usr/bin/melisa`.

## 2. Первый запуск

```bash
# Быстрая проверка (10 секунд, BCC tools не обязательны)
sudo melisa collect --profile quick -o report.json
```

Вы увидите:

```
melisa v0.4.1 | profile=quick | duration=10s

Tier 1 (procfs)  ████████████████████████████████████████ 8/8   2.1s
Tier 2 (BCC)     ████████████████████████████████████████ 4/4  10.3s

Health Score:  68 / 100  ⚠️
Anomalies:     cpu_utilization CRITICAL (98.7%)
               load_average WARNING (3.2x CPUs)
Recommendations: 2

Report saved to report.json
```

## 3. Чтение результатов

```bash
# Оценка здоровья (0-100)
jq '.summary.health_score' report.json

# Что не так?
jq '.summary.anomalies[]' report.json

# Как починить
jq '.summary.recommendations[] | {type, title, commands}' report.json
```

## 4. Применение исправлений

Рекомендации содержат готовые команды:

```bash
# Пример: melisa рекомендует включить BBR
sysctl -w net.core.default_qdisc=fq
sysctl -w net.ipv4.tcp_congestion_control=bbr
```

## 5. Проверка результата

```bash
# Запустить снова и сравнить
sudo melisa collect --profile quick -o after.json
melisa diff report.json after.json
```

Diff покажет что улучшилось, что ухудшилось, и дельту health score.

## Что дальше?

| Цель | Команда |
|------|---------|
| Полный анализ (все 67 BCC tools) | `sudo melisa collect --profile standard -o report.json` |
| Глубокий анализ (стеки, memleak) | `sudo melisa collect --profile deep -o report.json` |
| Только сеть | `sudo melisa collect --profile standard --focus network -o net.json` |
| Конкретный процесс | `sudo melisa collect --profile standard --pid 12345 -o app.json` |
| Контейнер | `sudo melisa collect --profile standard --cgroup /sys/fs/cgroup/.../nginx.service -o cg.json` |
| Установить BCC tools | `sudo melisa install` |
| Использовать с Claude/Cursor | `melisa mcp` → [гайд по MCP](13-ai-integration.md) |

## Профили сбора

| Профиль | Время | Что запускается | Для чего |
|---------|-------|-----------------|----------|
| **quick** | 10с | Tier 1 + 4 основных BCC | Проверка здоровья, CI |
| **standard** | 30с | Все Tier 1 + все 67 BCC | Обычная диагностика |
| **deep** | 60с | Всё + memleak, biostacks | Поиск корневой причины |

## Справка по CLI

```
melisa collect   Сбор метрик и генерация JSON-отчёта
  --profile       quick|standard|deep (по умолчанию: standard)
  --focus         cpu,memory,disk,network,stacks
  --pid           Фильтр по PID
  --cgroup        Фильтр по cgroup
  --duration      Переопределить длительность (например, 15s, 1m)
  --ai-prompt     Включить AI-промпт в отчёт
  --output, -o    Файл вывода (- для stdout)

melisa diff      Сравнение двух отчётов
melisa install   Установка BCC tools
melisa mcp       MCP сервер (stdio JSON-RPC)
melisa capabilities  Доступные инструменты и возможности ядра
```

---

*Далее: [Введение — Как работает melisa](00-introduction.md)*
