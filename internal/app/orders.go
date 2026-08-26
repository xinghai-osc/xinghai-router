package app

import (
	"net/http"
	"strconv"
	"strings"
)

// accountOrder is a combined view over top-up and subscription orders.
type accountOrder struct {
	OrderNo         string `json:"order_no"`
	OrderType       string `json:"order_type"`
	PlanName        string `json:"plan_name"`
	PaymentType     string `json:"payment_type"`
	Amount          string `json:"amount"`
	Status          string `json:"status"`
	ProviderTradeNo string `json:"provider_trade_no,omitempty"`
	PeriodKind      string `json:"period_kind"`
	PaidAt          any    `json:"paid_at"`
	CreatedAt       any    `json:"created_at"`
}

type adminOrder struct {
	OrderNo         string `json:"order_no"`
	OrderType       string `json:"order_type"`
	PlanName        string `json:"plan_name"`
	PaymentType     string `json:"payment_type"`
	Amount          string `json:"amount"`
	Status          string `json:"status"`
	ProviderTradeNo string `json:"provider_trade_no,omitempty"`
	PeriodKind      string `json:"period_kind"`
	PaidAt          any    `json:"paid_at"`
	CreatedAt       any    `json:"created_at"`
	UserID          string `json:"user_id"`
	UserEmail       string `json:"user_email"`
	UserName        string `json:"user_name"`
}

func (s *Service) listAdminOrders(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := listPage(r)
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	where := ""
	args := []any{}
	if term != "" {
		where = " where o.order_no ilike $1"
		args = append(args, "%"+term+"%")
	}
	countArgs := append([]any(nil), args...)
	var total int
	if err := s.db.QueryRow(r.Context(), `select count(*) from (
		select order_no from payment_orders
		union all select order_no from subscription_orders
	) o`+where, countArgs...).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not count orders")
		return
	}
	args = append(args, pageSize, offset)
	rows, err := s.db.Query(r.Context(), `select o.order_no,o.order_type,o.payment_type,o.amount,o.status,o.provider_trade_no,o.plan_name,o.period_kind,o.paid_at,o.created_at,o.user_id,u.email,u.name from (
		select po.order_no,'payment' as order_type,po.payment_type,po.amount::text as amount,po.status,coalesce(po.provider_trade_no,'') as provider_trade_no,'' as plan_name,'' as period_kind,po.paid_at,po.created_at,po.user_id from payment_orders po
		union all
		select so.order_no,'subscription' as order_type,so.payment_type,so.amount::text as amount,so.status,coalesce(so.provider_trade_no,'') as provider_trade_no,p.name as plan_name,so.period_kind,so.paid_at,so.created_at,so.user_id from subscription_orders so join subscription_plans p on p.id=so.plan_id
	) o join users u on u.id=o.user_id`+where+` order by o.created_at desc limit $`+strconv.Itoa(len(args)-1)+` offset $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load orders")
		return
	}
	defer rows.Close()
	orders := []adminOrder{}
	for rows.Next() {
		var order adminOrder
		if err := rows.Scan(&order.OrderNo, &order.OrderType, &order.PaymentType, &order.Amount, &order.Status, &order.ProviderTradeNo, &order.PlanName, &order.PeriodKind, &order.PaidAt, &order.CreatedAt, &order.UserID, &order.UserEmail, &order.UserName); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load orders")
			return
		}
		orders = append(orders, order)
	}
	if rows.Err() != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load orders")
		return
	}
	writePaged(w, orders, total, page, pageSize)
}

// accountOrders lists every historical order for the signed-in user, mixing
// wallet top-ups and subscription purchases, newest first.
func (s *Service) accountOrders(w http.ResponseWriter, r *http.Request) {
	account := accountFromContext(r)
	rows, err := s.db.Query(r.Context(), `select o.order_no,o.order_type,o.payment_type,o.amount,o.status,o.provider_trade_no,o.plan_name,o.period_kind,o.paid_at,o.created_at from (
		select po.order_no,'payment' as order_type,po.payment_type,po.amount::text as amount,po.status,coalesce(po.provider_trade_no,'') as provider_trade_no,'' as plan_name,'' as period_kind,po.paid_at,po.created_at from payment_orders po where po.user_id=$1
		union all
		select so.order_no,'subscription' as order_type,so.payment_type,so.amount::text as amount,so.status,coalesce(so.provider_trade_no,'') as provider_trade_no,p.name as plan_name,so.period_kind,so.paid_at,so.created_at from subscription_orders so join subscription_plans p on p.id=so.plan_id where so.user_id=$1
	) o order by o.created_at desc limit 100`, account.userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load orders")
		return
	}
	defer rows.Close()
	orders := []accountOrder{}
	for rows.Next() {
		var order accountOrder
		if err = rows.Scan(&order.OrderNo, &order.OrderType, &order.PaymentType, &order.Amount, &order.Status, &order.ProviderTradeNo, &order.PlanName, &order.PeriodKind, &order.PaidAt, &order.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "could not load orders")
			return
		}
		orders = append(orders, order)
	}
	if rows.Err() != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load orders")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": orders})
}
