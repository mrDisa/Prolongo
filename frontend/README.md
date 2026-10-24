# web — React + TypeScript + Vite

Панель маркетолога: обзор, клиенты, карточка, модель риска, данные; светлая и тёмная темы.

ТЗ: [docs/ТЗ_FRONTEND.md](../docs/ТЗ_FRONTEND.md). Дизайн: https://claude.ai/artifact/CbLZXVsCTVxqpmovAUSBKT.

## Запуск с API

Для разработки Vite по умолчанию проксирует `/api` на настоящий стек `http://localhost:8080`. Запусти стек из корня репозитория (`docker compose up --build`), затем запусти фронтенд:

```bash
npm run dev
```

Чтобы временно использовать Prism, запусти `npm run mock` и задай `VITE_API_TARGET=http://localhost:4010` в `frontend/.env.local`. На Prism поток SSE не работает; для локальной имитации событий можно включить `VITE_FAKE_LIVE=1`.
