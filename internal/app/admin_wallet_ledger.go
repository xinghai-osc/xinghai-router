package app

import (
	"net/http"
	"strings"
)

func (s *Service) listAdminWalletLedger(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := listPage(r)
	args := make([]any, 0, 8)
	where := []string{"1=1"}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, strings.Replace(clause, "?", "$"+itoa(len(args)), 1))
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		needle := "%" + q + "%"
		startArg := len(args) + 1
		args = append(args, needle, needle, needle, needle, needle)
		where = append(where, `(u.email ilike $`+itoa(startArg)+` or u.name ilike $`+itoa(startArg+1)+` or u.id::text ilike $`+itoa(startArg+2)+` or coalesce(wl.request_id,'') ilike $`+itoa(startArg+3)+` or coalesce(wl.note,'') ilike $`+itoa(startArg+4)+`)`)
	}
	if kind := strings.TrimSpace(r.URL.Query().Get("kind")); kind != "" {
		add(`wl.kind=?`, kind)
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		add(`wl.settlement_status=?`, status)
	}
	if userID := strings.TrimSpace(r.URL.Query().Get("user_id")); userID != "" {
		add(`u.id::text=?`, userID)
	}
	if start := strings.TrimSpace(r.URL.Query().Get("start")); start != "" {
		add(`wl.created_at>=?`, start)
	}
	if end := strings.TrimSpace(r.URL.Query().Get("end")); end != "" {
		add(`wl.created_at<=?`, end)
	}
	filter := " where " + strings.Join(where, " and ")
	var total int
	countArgs := append([]any(nil), args...)
	if err := s.db.QueryRow(r.Context(), `select count(*) from wallet_ledger wl join users u on u.id=wl.user_id`+filter, countArgs...).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	pageArg := len(args) + 1
	offsetArg := len(args) + 2
	args = append(args, pageSize, offset)
	query := `select wl.id,wl.user_id,u.email,u.name,wl.amount,wl.balance_after,wl.kind,wl.request_id,wl.note,wl.created_at,wl.settlement_status,wl.settlement_date,wl.settled_at,coalesce(ws.error,'')
		from wallet_ledger wl join users u on u.id=wl.user_id
		left join wallet_settlements ws on ws.ledger_id=wl.id` + filter + ` order by wl.created_at desc,wl.id desc limit $` + itoa(pageArg) + ` offset $` + itoa(offsetArg)
	rows, err := s.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
		return
	}
	defer rows.Close()
	data := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var id, userID, email, name, kind, settlementStatus string
		var amount, balanceAfter, requestID, note, createdAt, settlementDate, settledAt, settlementError any
		if rows.Scan(&id, &userID, &email, &name, &amount, &balanceAfter, &kind, &requestID, &note, &createdAt, &settlementStatus, &settlementDate, &settledAt, &settlementError) != nil {
			continue
		}
		data = append(data, map[string]any{
			"id": id, "user_id": userID, "user_email": email, "user_name": name,
			"amount": amount, "balance_after": balanceAfter, "kind": kind, "request_id": requestID,
			"note": note, "created_at": createdAt, "settlement_status": settlementStatus,
			"settlement_date": settlementDate, "settled_at": settledAt, "settlement_error": settlementError,
		})
	}
	writePaged(w, data, total, page, pageSize)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	pos := len(digits)
	for value > 0 {
		pos--
		digits[pos] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[pos:])
}
