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
하지만 AI 기능이 탑재됨
