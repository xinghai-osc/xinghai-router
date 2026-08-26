package app

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) notifyAdminsOfPurchase(parent context.Context, kind, orderNo, userID, amount, planName string) {
	if strings.TrimSpace(orderNo) == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
		defer cancel()
		if err := s.sendPurchaseNotifications(ctx, kind, orderNo, userID, amount, planName); err != nil {
			log.Printf("purchase notification email failed: %v", err)
		}
	}()
}

func (s *Service) sendPurchaseNotifications(ctx context.Context, kind, orderNo, userID, amount, planName string) error {
	cfg := s.loadSystemConfig(ctx)
	if !cfg.emailVerificationEnabled() {
		return nil
	}
	var userEmail string
	if kind == "subscription" {
		if err := s.db.QueryRow(ctx, `select u.id::text,u.email,p.name from subscription_orders o join users u on u.id=o.user_id join subscription_plans p on p.id=o.plan_id where o.order_no=$1`, orderNo).Scan(&userID, &userEmail, &planName); err != nil {
			return fmt.Errorf("load subscription purchaser: %w", err)
		}
	} else if err := s.db.QueryRow(ctx, `select email from users where id=$1`, userID).Scan(&userEmail); err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("load purchaser: %w", err)
	}
	rows, err := s.db.Query(ctx, `select email from users where role='admin' and enabled and email <> '' order by id`)
	if err != nil {
		return fmt.Errorf("load administrators: %w", err)
	}
	defer rows.Close()
	admins := []string{}
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return fmt.Errorf("scan administrator: %w", err)
		}
		admins = append(admins, email)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read administrators: %w", err)
	}
	if len(admins) == 0 {
		return nil
	}

	title := "账户充值"
	if kind == "subscription" {
		title = "订阅购买"
	}
	subject := fmt.Sprintf("%s %s通知 / %s notification", s.siteName(ctx), title, title)
	body := fmt.Sprintf(`<div style="max-width:520px;margin:0 auto;padding:32px;font-family:-apple-system,'Segoe UI',sans-serif;color:#1a1a2e">
	<h2 style="margin:0 0 8px;font-size:20px">%s</h2>
	<p style="margin:0 0 24px;color:#666;font-size:14px">有用户完成了%s / A user completed a %s</p>
	<table style="width:100%%;border-collapse:collapse;font-size:14px;color:#444">
	<tr><td style="padding:8px 0;color:#999;width:120px">用户邮箱 / User</td><td style="padding:8px 0">%s</td></tr>
	<tr><td style="padding:8px 0;color:#999">订单号 / Order</td><td style="padding:8px 0">%s</td></tr>
	<tr><td style="padding:8px 0;color:#999">金额 / Amount</td><td style="padding:8px 0">%s</td></tr>
	%s</table>
	</div>`, html.EscapeString(s.siteName(ctx)), title, strings.ToLower(title), html.EscapeString(userEmail), html.EscapeString(orderNo), html.EscapeString(amount), purchasePlanRow(planName))
	for _, admin := range admins {
		if err := s.sendEmail(ctx, admin, subject, body); err != nil {
			log.Printf("purchase notification email to %s failed: %v", admin, err)
		}
	}
	return nil
}

func purchasePlanRow(planName string) string {
	if planName == "" {
		return ""
	}
	return fmt.Sprintf(`<tr><td style="padding:8px 0;color:#999">套餐 / Plan</td><td style="padding:8px 0">%s</td></tr>`, html.EscapeString(planName))
}
