# Project Memory

## Project Overview

- TDX Workbench is a local A-share market-data and research workspace.
- Go services expose TDX market data; the React frontend consumes the Web gateway APIs.

## Tech Stack

- Go 1.25 (the Docker builders bootstrap it through `GOTOOLCHAIN=auto`)
- React 19, TypeScript and Vite
- Docker Compose for the multi-service local deployment

## Architecture

- `apps/web`: Web UI and API gateway
- `services/market-service`: market-data API
- `packages/tdx-core`: TDX protocol client and data models

## Conventions

- `protocol.Price` values use milli-yuan units and must be divided by 1000 for display.
- Quote `Kline.Volume` uses lots and must be multiplied by 100 to display shares.

## Decisions

- Frontend quote consumers normalize the current `Kline` response and the legacy `K` response through shared helpers in `apps/web/frontend/src/lib/format.ts`.
- Selection tracking treats signals created before 09:30 Asia/Shanghai as executable at the signal-day open and counts that session as D1. Later signals start their observation window on the next trading session.
- Fixed close data sync runs at 16:00 on weekdays; fixed D1/D5/D10 tracking runs at 17:15 after the close-data jobs have had time to finish.

## Pitfalls

- `/api/quote` serializes the Go field as `Kline`, with OHLC, volume, and amount nested inside it. Do not read only legacy `K`, `TotalHand`, or top-level `Amount` fields.
- Quote rate is encoded as a signed 16-bit hundredth-percent value. Decode through `int16`; treating it as unsigned turns small negative values into `655.xx%`.
- Tracking calculations must ignore placeholder or malformed daily bars: date and OHLC must be positive, and high/low must contain open and close.
- `000001` is ambiguous: `AddPrefix` maps the bare code to `sz000001` (平安银行) because `isSZStock` (leading `0`) is checked before `isSHIndex` (leading `000`). Reach 上证指数 only via the explicit `sh000001` prefix. Index K-lines must go through the index API (`KindIndex`) — the stock path (`KindStock`) misparses the 4 extra up/down-count bytes per bar.
