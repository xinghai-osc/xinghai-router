package app

import (
	"context"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"
)

func (s *Service) notifyQuotaExhausted(parent context.Context, userID, category, eventKey string) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(eventKey) == "" || !s.loadSystemConfig(parent).emailVerificationEnabled() {
		return
	}
	userIDValue, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	var email string
	err = s.db.QueryRow(ctx, `with claimed as (
		insert into quota_exhaustion_notifications(user_id,category,event_key)
		values($1,$2,$3) on conflict do nothing returning user_id
	) select u.email from claimed join users u on u.id=claimed.user_id where u.enabled and u.email<>''`, userIDValue, category, eventKey).Scan(&email)
	cancel()
	if err != nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
		defer cancel()
		label := "账户余额"
		if category == "subscription" {
			label = "订阅额度"
		}
		site := html.EscapeString(s.siteName(ctx))
		subject := fmt.Sprintf("%s 额度已用尽 / Quota exhausted", s.siteName(ctx))
		body := fmt.Sprintf(`<div style="max-width:520px;margin:0 auto;padding:32px;font-family:-apple-system,'Segoe UI',sans-serif;color:#1a1a2e">
		<h2 style="margin:0 0 16px;font-size:20px">%s</h2>
		<p>您的%s已用尽，请登录账户查看详情并充值或续订。</p>
		<p>Your %s has been exhausted. Sign in to review your account and add balance or renew your subscription.</p>
		</div>`, site, label, strings.ToLower(label))
		if err := s.sendEmail(ctx, email, subject, body); err != nil {
			log.Printf("quota exhaustion email failed for user %s: %v", userID, err)
		}
	}()
}
