#!/bin/bash

# agy TUI가 전달하는 실시간 JSON 메타데이터를 stdin으로 입력받습니다.
input=$(cat)

# jq를 사용하여 필요한 필드 파싱
CONV_ID=$(echo "$input" | jq -r '.conversation_id // "default"')
MODEL=$(echo "$input" | jq -r '.model.display_name // "Unknown"')
PCT=$(echo "$input" | jq -r '.context_window.used_percentage // 0' | cut -d. -f1)
INPUT_TOKENS=$(echo "$input" | jq -r '.context_window.current_usage.input_tokens // 0')
OUTPUT_TOKENS=$(echo "$input" | jq -r '.context_window.current_usage.output_tokens // 0')

# 디스코드 봇이 세션별로 개별 조회할 수 있도록 telemetry_<conversation_id>.json 파일에 기록
# (~/.gemini/antigravity-cli 디렉토리 하위에 저장)
CONF_DIR="${HOME}/.gemini/antigravity-cli"
mkdir -p "${CONF_DIR}"
echo "{\"model\": \"${MODEL}\", \"pct\": ${PCT}, \"input_tokens\": ${INPUT_TOKENS}, \"output_tokens\": ${OUTPUT_TOKENS}}" > "${CONF_DIR}/telemetry_${CONV_ID}.json"

# agy TUI 하단 상태표시줄에 출력될 문자열 반환
echo "[$MODEL] Context: ${PCT}% (In: ${INPUT_TOKENS} / Out: ${OUTPUT_TOKENS})"
