# ddokdak 리팩토링 계획 (전체 프로젝트)

> 상태: 슬롭 제거 완료(`4e71b8d`, `ac7a8c4` — main 브랜치 푸시됨).
> 본 문서는 그 위에 얹을 **구조적 리팩토링 로드맵**이다. 코드를 바꾸는 것이 아니라,
> 바꿀 대상·순서·검증 기준을 정의한다.

---

## 0. 현황 요약

| 항목 | 내용 |
|------|------|
| 언어/모듈 | Go 1.25, `github.com/sleepysoong/ddokdak` |
| 핵심 의존성 | `bwmarrin/discordgo` v0.29.0 (직접), `gorilla/websocket`, `golang.org/x/crypto`, `golang.org/x/sys` |
| 소스 파일 | 19개 (프로덕션 11 + 테스트 8) |
| 최대 파일 | `internal/handler/message.go` 247 LOC (그 다음 `usage/tracker.go` 169) |
| 아키텍처 | `cmd` 엔트리 → `bot`(조립) → `handler`(이벤트) / `command`(슬래시) / `session`(세션+큐) / `agy`(외부 CLI) / `usage`(사용량) / `store`(채널) / `config` / `downloader` |
| 테스트 | 패키지별 단위 테스트 존재, 통합/핸들러 단위 테스트 부재 |

---

## 1. 목표 및 원칙

1. **테스트 용이성** — `discordgo.Session`(구체 타입) 결합을 인터페이스로 격리해 핸들러를 실제 디스코드 없이 단위 테스트.
2. **단일 책임(SRP)** — 247 LOC `message.go`를 라우팅/세션생명주기/디바운스/응답파이프라인으로 분리.
3. **명시적 에러 처리** — `log.Printf` 후 조용히 무시하던 경로를 에러 전파/사용자 안내로 전환.
4. **생명주기 관리** — 세션별 고루틴을 취소 가능하게(`context`/stop 채널) 해 leak 제거.
5. **조립root 명시화** — `bot.New(cfg)` 내부 생성을 구성 루트로 분리, 의존성 주입(DI) 가능하게.
6. **컨벤션 유지** — 이미 Go 표준(godoc, `gofmt`)을 준수하므로 이를 해치지 않음.

---

## 2. 파일별 발견 문제

### `internal/handler/message.go` (247 LOC) — 최우선
- **관심사 혼재**: 라우팅(`HandleMessage`), 스레드 생성(`handleNewConversation`),
  세션 복구(`handleThreadMessage`), 첨부처리(`enqueueMessage`), 디바운스 고루틴
  (`startSessionProcessor`), AI 호출 오케스트레이션(`processAIResponse`), 타이핑(`showTyping`),
  전송(`sendResponse`/`sendErrorMessage`), 헬퍼(`isThreadChannel`/`truncateString`/`splitMessage`)가 한 파일.
- **고루틴 leak**: `startSessionProcessor`가 `for { select }` 무한 루프, 세션 종료/스레드 삭제 시
  중단 신호가 없음 → 세션이 메모리에서 제거돼도 고루틴 잔존.
- **취소 불가 타이머**: `time.NewTimer(time.Hour)` 초기화 후 즉시 `Stop()` — 의도 불명확.
- **구체 타입 결합**: `*discordgo.Session` 직접 사용 → 테스트 불가.
- **매직넘버**: `AutoArchiveDuration: 10080` 하드코딩(상수화 권장).

### `internal/bot/bot.go` (101 LOC)
- **조립 책임 혼재**: `New(cfg)`가 sessionManager/agyClient/store/usage/handlers를 모두 내부 생성.
  테스트에서 일부만 바꿔 끼우기 어려움.
- **Intent 하드코딩**: 필요 인텐트가 코드에 박힘(설정화 가능).

### `internal/session/manager.go` (100 LOC)
- **빈번한 전체 직렬화**: `Save()`가 대화 ID 갱신 시마다 호출되어 파일 전체를 덮어씀.
  배치/비동기 flush 고려.
- **재시작 복구 불완전**: 파일에서 세션을 복원하나, `handler`는 메모리 부재 시 새 세션 생성(대화 이력 단절 가능).

### `internal/session/session.go` (112 LOC)
- **큐+디바운스 로직**이 세션 구조체에 섞임. 큐는 별도 타입(`MessageQueue`)으로 분리 권장.

### `internal/agy/client.go` (95 LOC)
- **외부 CLI 계약 취약**: stdout/log 파싱으로 `conversationID` 추출. 파싱 실패 시 조용히 빈 값.
- **타임아웃**: `--print-timeout`만 의존, `context` 타임아웃 미연동.
- **에러 매핑**: CLI 실패를 의미있는 에러로 변환 부족.

### `internal/command/{register,handler}.go` (81/139)
- **인터페이스 부재**: 커맨드 등록/핸들러가 `bot`에 직접 결합. `CommandHandler` 인터페이스로 추상화.

### 기타
- `config`/`store`/`downloader`/`usage`는 비교적 깔끔. `usage/dashboard`는 `stopChan`으로
  고루틴 취소가 이미 되어 있음(좋은 패턴, `message.go`에 이식).

---

## 3. 단계별 로드맵

### Phase 1 — 테스트 토대: 디스코드 의존성 격리 (기반)
- **목표**: 핸들러를 실제 디스코드 없이 단위 테스트 가능하게.
- **대상**: `internal/handler`, `internal/bot`.
- **작업**:
  1. 좁은 송신 인터페이스 정의:
     ```go
     type DiscordSender interface {
         ChannelMessageSend(channelID, content string) (*discordgo.Message, error)
         MessageThreadStartComplex(channelID, messageID string, data *discordgo.ThreadStart) (*discordgo.Channel, error)
         ChannelTyping(channelID string) error
         Channel(channelID string) (*discordgo.Channel, error)
     }
     ```
  2. `*discordgo.Session`이 이 인터페이스를 만족함을 컴파일 타임 검증(`var _ DiscordSender = (*discordgo.Session)(nil)`).
  3. `MessageHandler`가 `*discordgo.Session` 대신 `DiscordSender`(또는 세션이 이를 구현하므로 그대로 두되, 시그니처를 인터페이스로 받음)를 받도록 변경.
  4. 페이크 구현(`fakeSender`)으로 `HandleMessage`/`handleNewConversation` 테스트 추가.
- **검증**: 새 단위 테스트 GREEN, `go test ./internal/handler/...` 통과, 기존 테스트 회귀 없음.

### Phase 2 — `message.go` 관심사 분리
- **목표**: 247 LOC → 책임별 파일 분리(SRP).
- **대상**: `internal/handler/`.
- **작업**:
  - `router.go`: `HandleMessage`(봇/쓰레드/등록 채널 라우팅), `handleNewConversation`, `handleThreadMessage`.
  - `session_lifecycle.go`: 세션 생성/복구/마지막활동 갱신.
  - `processor.go`: `startSessionProcessor`(디바운스 고루틴) + 취소 신호 연동.
  - `pipeline.go`: `processAIResponse`, `showTyping`, `sendResponse`, `sendErrorMessage`, `enqueueMessage`.
  - `helpers.go`: `isThreadChannel`, `truncateString`, `splitMessage`(이미 순수 함수).
- **검증**: 분리 후 `go build ./...`, `go test ./...` 그린, 동작 동일(블랙박스 테스트로 입증).

### Phase 3 — 고루틴 생명주기 관리
- **목표**: 세션 종료 시 고루틴 leak 제거.
- **대상**: `session/manager.go`, `handler/processor.go`.
- **작업**:
  1. `Session`에 `context.Context` + `cancel` 보관, `RemoveSession` 시 `cancel()` 호출.
  2. `startSessionProcessor` 루프가 `<-ctx.Done()`에서 종료.
  3. `bot.Stop()`이 모든 활성 세션 취소 후 대기.
- **검증**: 테스트에서 세션 생성→제거 후 고루틴 수(`runtime.NumGoroutine`) 정체 없음 입증.

### Phase 4 — 의존성 주입 / 구성 루트
- **목표**: `bot.New(cfg)`의 내부 생성을 명시적 조립 루트로.
- **대상**: `internal/bot/bot.go`, `cmd/ddokdak/main.go`.
- **작업**:
  1. `New(cfg, deps BotDeps)` 형태로 주입点或 `BuildBot(cfg)` 조립 함수 분리.
  2. 테스트에서 페이크 의존성 주입 가능.
- **검증**: `bot_test.go`가 페이크로 `New` 호출 GREEN.

### Phase 5 — 에러 처리/로깅 중앙화
- **목표**: 조용한 무시 제거, 로거 일원화.
- **대상**: 전 모듈.
- **작업**:
  1. `internal/log`(또는 `slog` 표준) 래퍼 도입, `config.LogLevel`과 연동.
  2. `processAIResponse` 등에서 에러를 사용자 메시지로 전파(이미 `sendErrorMessage` 존재 — 호출 누락 경로 점검).
  3. `agy` 실패를 의미있는 에러 타입으로 매핑.
- **검증**: 에러 경로 단위 테스트 추가, 수동으로 잘못된 토큰/타임아웃 시나리오 확인.

### Phase 6 — 세션 영속화 견고화
- **목표**: 빈번 직렬화 완화 + 재시작 복구 일관성.
- **대상**: `session/manager.go`.
- **작업**:
  1. `Save()` 비동기 배치(딜레이 플러시) 또는 변경분만 기록.
  2. 기동 시 파일 복원 + 미존재 시 `agy` 대화 재개 전략 수립(현재는 새 세션).
- **검증**: 재시작 후 대화 이어짐 통합 테스트(가능 범위 내).

### Phase 7 — `agy` 클라이언트 견고성
- **목표**: CLI 계약 취약성 완화.
- **대상**: `internal/agy/client.go`.
- **작업**:
  1. `conversationID` 파싱 실패 시 명시적 에러(조용한 빈 값 금지).
  2. `context` 타임아웃을 `Execute`에 연동(`ctx`로 프로세스 취소).
  3. stdout/stderr 분리 캡처.
- **검증**: 파싱 실패 단위 테스트, 타임아웃 단위 테스트.

### Phase 8 — 통합/시나리오 테스트
- **목표**: 종단 동작 보증.
- **작업**:
  1. 페이크 디스코드 + 페이크 agy로 "메시지→스레드→AI응답→전송" 시나리오 테스트.
  2. 동시 메시지 디바운스 병합 검증.
- **검증**: 시나리오 테스트 GREEN.

---

## 4. 위험 및 고려사항

| 위험 | 완화 |
|------|------|
| `discordgo.Session`가 인터페이스 전부를 구현하지 않을 수 있음 | 필요 메서드만 인터페이스에 포함, 누락 시 어댑터 함수 |
| Phase 2 분리가 동작 회귀 유발 | 분리 전 블랙박스 테스트 선작성(RED→GREEN) |
| 세션 영속화 포맷 변경 시 기존 데이터 손실 | 마이그레이션/버전 필드 |
| 외부 CLI(`agy`) 동작 변화에 취약 | 계약을 문서화하고 파싱 단위 테스트로 방어 |
| 고루틴 취소 누락 시 데드락/leak | `ctx.Done()` 종료 경로 테스트 필수 |

---

## 5. 검증 계획 (전체)

- 모든 Phase 종료 시 `go build ./...` + `go vet ./...` + `go test ./...` 그린 유지.
- 신규 기능/리팩터마다 RED→GREEN 테스트 선작성.
- Phase 3, 8은 **수동 QA** 병행: `agy` 더미로 봇 기동 → 채널 메시지 → 스레드 생성 → 응답 수신 → 스레드 삭제 후 고루틴 정체 확인(tmux/`runtime.NumGoroutine` 덤프).
- 완료 기준: 위 8개 Phase 모두 적용 + 회귀 없음 + Oracle 리뷰 통과.

---

## 6. 추천 실행 순서

`Phase 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8`
(1·2·3이 테스트 가능성·안정성 토대이므로 선행, 5·6·7은 점진 적용, 8은 최종 보증)

각 Phase는 독립 커밋으로 분리하여 리뷰 용이성 확보.
