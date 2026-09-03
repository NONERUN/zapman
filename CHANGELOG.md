# Changelog

Формат секций: Added / Changed / Removed / Breaking. Политика alpha — в [`AGENTS.md`](AGENTS.md).

## Unreleased

### Added

- WinForms GUI (`gui.bat`, `utils/gui.ps1`): стратегии, Start/Stop, служба, Game Filter, IPSet.
- [`AGENTS.md`](AGENTS.md) и [`PLAN.md`](PLAN.md) для агентов и целей alpha.

### Changed

- `service.bat`: в списке стратегий для Install Service больше нет `gui.bat`.
- Целевая раскладка: один `.bat` (проверка среды), остальное `.ps1`; переход на zapret2 (`winws2` + Lua) — [`PLAN.md`](PLAN.md).
- Обратная совместимость только у набора стратегий; имена стратегий можно менять — [`AGENTS.md`](AGENTS.md).
