# Changelog

Формат секций: Added / Changed / Removed / Breaking. Политика alpha — в [`AGENTS.md`](AGENTS.md).

## Unreleased

### Added

- WinForms GUI (`zapret.bat`, `utils/gui.ps1`): стратегии, Start/Stop, служба, Game Filter, IPSet.
- [`AGENTS.md`](AGENTS.md) и [`PLAN.md`](PLAN.md) для агентов и целей alpha.

### Changed

- Вход GUI: `zapret.bat` (проверка Windows PowerShell 3.0+, цель 5.1). `gui.bat` удалён.
- Целевая раскладка: один `.bat` (проверка среды), остальное `.ps1`; переход на zapret2 (`winws2` + Lua) — [`PLAN.md`](PLAN.md).
- Обратная совместимость только у набора стратегий; имена стратегий можно менять — [`AGENTS.md`](AGENTS.md).
