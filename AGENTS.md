항상 한글만을 사용하고 가능하면 Google의 Antigravity CLI에게 지시하여 코딩하고
Antigravity CLI를 사용할 수 없는 경우 (예: 한도 초과)에만 직접 코딩할 것.

작업이 끝날 때 마다 커밋하고 즉시 푸쉬할 것.

## Antigravity CLI에서 사용할 모델
"Claude Opus 4.6 (Thinking)"를 사용하다가 한도가 초과한 경우
중요한 작업은 "Gemini 3.1 Pro (High)"에 이외 작업은 "Gemini 3.5 Flash (High)"로 작업하십시오.

## Antigravity CLI를 사용하기 위해 다음을 shell에서 실행하십시오
```agy --model <model> --print-timeout 5m --log-file <file> --dangerously-skip-permissions -p <prompt>```

예) `agy --model "Gemini 3.1 Pro (High)" --print-timeout 5m --log-file debug2.log --dangerously-skip-permissions -p "넌 무슨 모델이니"`

# 만들고자 하는 프로젝트
Go 언어로 제작한 디스코드 봇

## 프로젝트 설명
지정한 채널 (/채널지정으로 지정함)에서 채팅을 치면 그 내용으로 쓰레드를 만들고
그 내용을 프롬프트로 `Antigravity CLI`를 호출하고 결과물을 보내줘.
그 쓰레드는 만료되지 않게 설정해야하며 그 쓰레드에서 대화한 내용은 다 한 세션이다.
(새로운 쓰레드를 만들 때 세션을 생성해야 함)

## 사용자에게 파일 전송하기
작업한 결과물이나 파일을 디스코드 사용자에게 직접 전송(첨부)해주고 싶다면, 응답 메시지에 `<ATTACH:/절대경로/또는/상대경로/파일명>` 형식의 태그를 포함하십시오. 
예를 들어 `코드.go` 파일을 보내고 싶다면 답변 어딘가에 `<ATTACH:코드.go>`라고 쓰면 봇이 해당 파일을 읽어서 디스코드 첨부파일로 함께 전송합니다.

# 프로젝트 개발
컨벤션 룰에 맞춰서 개발하여 변수, 함수, 클래스, 파일 이름, 폴더 이름까지 신경쓰고
항상 테스트 파일을 만들어 검증해
