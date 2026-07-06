# 똑닥 (DDokDak) 🤖

Go 언어로 제작한 AI 디스코드 봇입니다. 지정된 채널에서 사용자의 메시지를 받아 자동으로 쓰레드를 생성하고, [Antigravity CLI](https://github.com/google/anthropic-tools)를 통해 AI 응답을 제공합니다.

## ✨ 주요 기능

- **`/채널지정`** — AI 대화를 활성화할 채널을 지정합니다.
- **`/채널해제`** — 지정된 AI 대화 채널을 해제합니다.
- **자동 쓰레드 생성** — 지정된 채널에서 메시지를 보내면 자동으로 쓰레드가 생성됩니다.
- **세션 기반 대화** — 쓰레드 내 모든 대화는 하나의 세션으로 관리되어 맥락이 유지됩니다.
- **쓰레드 만료 방지** — 생성된 쓰레드는 자동 보관되지 않도록 최대 기간(7일)으로 설정됩니다.
- **긴 응답 분할 전송** — 디스코드 메시지 길이 제한(2000자)을 초과하는 응답은 자동으로 분할하여 전송합니다.

## 🏗️ 프로젝트 구조

```
ddokdak/
├── cmd/
│   └── ddokdak/
│       └── main.go              # 엔트리포인트
├── internal/
│   ├── agy/
│   │   ├── client.go            # Antigravity CLI 클라이언트
│   │   └── client_test.go
│   ├── bot/
│   │   ├── bot.go               # 봇 코어 (초기화, 실행, 종료)
│   │   └── bot_test.go
│   ├── command/
│   │   ├── handler.go           # 슬래시 커맨드 처리
│   │   ├── handler_test.go
│   │   └── register.go          # 슬래시 커맨드 등록
│   ├── config/
│   │   ├── config.go            # 환경변수 기반 설정 관리
│   │   └── config_test.go
│   ├── handler/
│   │   ├── message.go           # 메시지 이벤트 처리
│   │   └── message_test.go
│   ├── session/
│   │   ├── manager.go           # 세션 매니저
│   │   ├── manager_test.go
│   │   └── session.go           # 세션 모델
│   └── store/
│       ├── channel.go           # 채널 저장소 (인메모리)
│       └── channel_test.go
├── go.mod
├── go.sum
├── .env.example
├── README.md
├── 아키텍쳐.md
├── AGENTS.md
└── GEMINI.md
```

## 🚀 시작하기

### 사전 요구 사항

- **Go 1.21+**
- **Antigravity CLI (agy)** — 시스템 PATH에 설치되어 있어야 합니다.
- **디스코드 봇 토큰** — [디스코드 개발자 포털](https://discord.com/developers/applications)에서 생성

### 봇 권한 설정

디스코드 개발자 포털에서 다음 권한을 활성화하세요:

**Bot Permissions:**
- Send Messages
- Create Public Threads
- Send Messages in Threads
- Read Message History
- Use Slash Commands

**Privileged Gateway Intents:**
- Message Content Intent

### 설치 및 실행

```bash
# 저장소 클론
git clone https://github.com/sleepysoong/ddokdak.git
cd ddokdak

# 환경변수 설정
cp .env.example .env
# .env 파일을 편집하여 DISCORD_TOKEN 설정

# 의존성 설치
go mod download

# 빌드
go build -o ddokdak ./cmd/ddokdak/

# 실행
export DISCORD_TOKEN="your-bot-token-here"
./ddokdak
```

### 환경변수

| 변수명 | 필수 | 기본값 | 설명 |
|--------|------|--------|------|
| `DISCORD_TOKEN` | ✅ | - | 디스코드 봇 토큰 |
| `AGY_MODEL` | ❌ | `Claude Opus 4.6 (Thinking)` | Antigravity CLI 기본 모델 |
| `AGY_FALLBACK_MODEL` | ❌ | `Gemini 3.1 Pro (High)` | 대체 모델 |
| `AGY_TIMEOUT` | ❌ | `5m` | agy 명령 타임아웃 |
| `LOG_LEVEL` | ❌ | `info` | 로그 레벨 (debug, info, warn, error) |

## 🧪 테스트

```bash
# 전체 테스트 실행
go test ./... -v

# 레이스 컨디션 검사 포함
go test ./... -race -v

# 특정 패키지 테스트
go test ./internal/session/ -v
```

## 📖 사용 방법

1. 봇을 디스코드 서버에 초대합니다.
2. `/채널지정` 커맨드로 AI 대화를 활성화할 채널을 지정합니다.
3. 지정된 채널에서 메시지를 보내면 자동으로 쓰레드가 생성되고 AI가 응답합니다.
4. 쓰레드 내에서 계속 대화하면 맥락이 유지됩니다.
5. `/채널해제` 커맨드로 AI 대화 기능을 해제할 수 있습니다.

## 🛠️ 기술 스택

- **Go 1.26** — 메인 프로그래밍 언어
- **discordgo v0.29.0** — Discord API 라이브러리
- **Antigravity CLI (agy)** — AI 모델 호출

## 📄 라이선스

이 프로젝트는 MIT 라이선스 하에 배포됩니다.
