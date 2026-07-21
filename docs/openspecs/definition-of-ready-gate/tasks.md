# Tasks: Definition-of-Ready Gate

- [ ] 1.1 Khảo sát cơ chế pause/resume hiện có (boundary resolution) → quyết định tái dùng hay tạo `PauseJob` helper (phối hợp với `cli-spec-first-flow` task 4.1)
- [ ] 1.2 `steps/dor_check.go`: readiness evaluation thuần Go (REQ-001) + tests matrix
- [ ] 1.3 `prompts/steps/dor_check.md` + question generation JSON parse (REQ-002)
- [ ] 1.4 Pause `awaiting_clarification` + round tracking, max 2 rounds → ready_with_warnings (REQ-003)
- [ ] 1.5 Bypass: label hotfix / autonomy autonomous (REQ-004) + tests
- [ ] 1.5b CLI-mode DI: worker inject cả ExecutionEngine + LLMClient vào cli_analyze; fallback bypass khi LLMClient unavailable + test không-key-không-crash (REQ-004b)
- [ ] 1.6 DAG wiring: chèn node, remap dependsOn (REQ-M01) + snapshot tests — **chỉ api_native flow**; CLI mode nhúng readiness check vào cli_analyze như precondition (export `dorResult`, xem design §CLI mode), không thêm node
- [ ] 1.7 API answer clarification + resume trigger + tests
- [ ] 1.8 UI: clarifications panel + answer form + status badge
- [ ] 1.9 Integration: task thiếu AC → pause → answer → resume → coding nhận answers trong context
- [ ] 1.10 Update specs.md status + ARCHITECTURE.md
