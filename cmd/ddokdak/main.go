// Package main은 ddokdak 디스코드 봇의 엔트리포인트입니다.
package main

import (
	"log"

	"github.com/sleepysoong/ddokdak/internal/bot"
	"github.com/sleepysoong/ddokdak/internal/config"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("똑닥 디스코드 봇을 시작합니다...")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("설정 로드 실패: %v", err)
	}
	log.Printf("설정 로드 완료 - 모델: %s, 타임아웃: %s", cfg.AgyModel, cfg.AgyTimeout)

	b, err := bot.New(cfg)
	if err != nil {
		log.Fatalf("봇 생성 실패: %v", err)
	}

	if err := b.Start(); err != nil {
		log.Fatalf("봇 시작 실패: %v", err)
	}

	b.Wait()

	if err := b.Stop(); err != nil {
		log.Fatalf("봇 종료 실패: %v", err)
	}
}
