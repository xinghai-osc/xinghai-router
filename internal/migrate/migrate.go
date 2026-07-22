package migrate

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var channelTypeToProvider = map[int]string{
	1:  "openai",
	2:  "openai",
	3:  "openai",
	4:  "ollama",
	5:  "openai",
	6:  "openai",
	7:  "openai",
	8:  "openai",
	9:  "openai",
	10: "openai",
	11: "openai",
	12: "openai",
	13: "openai",
	14: "anthropic",
	15: "openai",
	16: "openai",
	17: "openai",
	18: "openai",
	19: "openai",
	20: "openai",
	21: "openai",
	22: "openai",
	23: "openai",
	24: "openai",
	25: "openai",
	26: "openai",
	27: "openai",
	31: "openai",
	33: "openai",
	34: "openai",
	35: "openai",
	36: "openai",
	37: "openai",
	38: "openai",
	39: "openai",
	40: "openai",
	41: "openai",
	42: "openai",
	43: "openai",
	44: "openai",
	45: "openai",
	46: "openai",
	47: "openai",
	48: "openai",
	49: "openai",
	50: "openai",
	51: "openai",
	52: "openai",
	53: "openai",
	54: "openai",
	55: "openai",
	56: "openai",
	57: "openai",
	58: "openai",
	59: "anthropic",
}

type xUser struct {
	ID           int
	Username     string
	Password     string
	DisplayName  string
	Role         int
	Status       int
	Email        string
	Quota        int
	UsedQuota    int
	Group        string
	CreatedAt    int64
	Setting      string
	Remark       string
}

type xToken struct {
	ID             int
	UserID         int
	Key            string
	Status         int
	Name           string
	CreatedTime    int64
	AccessedTime   int64
	ExpiredTime    int64
	RemainQuota    int
	UnlimitedQuota bool
	UsedQuota      int
	Group          string
}

type xChannel struct {
	ID           int
	Type         int
	Key          string
	Name         string
	Status       int
	BaseURL      *string
	Models       string
	Group        string
	Priority     *int64
	Weight       *uint
	CreatedTime  int64
	ModelMapping *string
}

type xOption struct {
	Key   string
	Value string
}

type xAbility struct {
	Group     string
	Model     string
	ChannelID int
	Enabled   bool
	Priority  *int64
	Weight    uint
	Tag       *string
}

type xSubscriptionPlan struct {
	ID                    int
	Title                 string
	Description           string
	PriceAmount           float64
	Currency              string
	DurationUnit          string
	DurationValue         int
	CustomSeconds         int64
	Enabled               bool
	SortOrder             int
	MaxPurchasePerUser    int
	UpgradeGroup          string
	DowngradeGroup        string
	TotalAmount           int64
	QuotaResetPeriod      string
	QuotaResetCustomSeconds int64
	CreatedAt             int64
	UpdatedAt             int64
}

type xUserSubscription struct {
	ID                  int
	UserID              int
	PlanID              int
	PriceAmount         float64
	AmountTotal         int64
	AmountUsed          int64
	StartTime           int64
	EndTime             int64
	Status              string
	Source              string
	LastResetTime       int64
	NextResetTime       int64
	UpgradeGroup        string
	PrevUserGroup       string
	DowngradeGroup      string
	AllowWalletOverflow bool
	CreatedAt           int64
	UpdatedAt           int64
}

type xSubscriptionOrder struct {
	ID                    int
	UserID                int
	PlanID                int
	Money                 float64
	TradeNo               string
	PaymentMethod         string
	PaymentProvider       string
	GatewayID             string
	Status                string
	CreateTime            int64
	CompleteTime          int64
	UpgradeSubscriptionID int
	ProviderPayload       string
}

type xTopUp struct {
	ID              int
	UserID          int
	Amount          int64
	Money           float64
	TradeNo         string
	PaymentMethod   string
	PaymentProvider string
	GatewayID       string
	CreateTime      int64
	CompleteTime    int64
	Status          string
}

func quotaToBalance(quota int64) float64 {
	return float64(quota) / 500000.0 * 0.002
}

type Progress struct {
	Step     string `json:"step"`
	Detail   string `json:"detail"`
	Current  int    `json:"current"`
	Total    int    `json:"total"`
	Finished bool   `json:"finished"`
}

type ProgressFunc func(Progress)

type step struct {
	name   string
	action func() (string, error)
}

func Run(ctx context.Context, sourceDSN, sourceDriver, targetDSN string, progress ProgressFunc) error {
	if progress != nil {
		progress(Progress{Step: "connect", Detail: "Connecting to source and target databases"})
	}

	src, err := sql.Open(sourceDriver, sourceDSN)
	if err != nil {
		return fmt.Errorf("open source database: %w", err)
	}
	defer src.Close()

	src.SetConnMaxLifetime(5 * time.Minute)
	src.SetMaxOpenConns(10)
	src.SetMaxIdleConns(5)

	if err := src.PingContext(ctx); err != nil {
		return fmt.Errorf("ping source database: %w", err)
	}

	target, err := pgxpool.New(ctx, targetDSN)
	if err != nil {
		return fmt.Errorf("connect target database: %w", err)
	}
	defer target.Close()

	if err := target.Ping(ctx); err != nil {
		return fmt.Errorf("ping target database: %w", err)
	}

	log.Println("Connected to source and target databases")

	var (
		userMap    map[int]int64
		groupMap   map[string]string
		channelMap map[int]string
		planMap    map[int]string
		subMap     map[int]string
	)

	steps := []step{
		{"users", func() (string, error) {
			var err error
			userMap, err = migrateUsers(ctx, src, target)
			return fmt.Sprintf("%d users", len(userMap)), err
		}},
		{"tokens", func() (string, error) {
			n, err := migrateTokens(ctx, src, target, userMap)
			return fmt.Sprintf("%d tokens", n), err
		}},
		{"groups", func() (string, error) {
			var err error
			groupMap, err = migrateGroups(ctx, src, target, userMap)
			return fmt.Sprintf("%d groups", len(groupMap)), err
		}},
		{"user_groups", func() (string, error) {
			n, err := migrateUserGroups(ctx, src, target, userMap, groupMap)
			return fmt.Sprintf("%d user-group assignments", n), err
		}},
		{"channels", func() (string, error) {
			var err error
			channelMap, err = migrateChannels(ctx, src, target, groupMap)
			return fmt.Sprintf("%d channels", len(channelMap)), err
		}},
		{"subscription_plans", func() (string, error) {
			var err error
			planMap, err = migrateSubscriptionPlans(ctx, src, target, groupMap)
			return fmt.Sprintf("%d subscription plans", len(planMap)), err
		}},
		{"user_subscriptions", func() (string, error) {
			var err error
			subMap, _, err = migrateUserSubscriptions(ctx, src, target, userMap, planMap)
			return "", err
		}},
		{"subscription_orders", func() (string, error) {
			n, err := migrateSubscriptionOrders(ctx, src, target, userMap, planMap, subMap)
			return fmt.Sprintf("%d subscription orders", n), err
		}},
		{"topups", func() (string, error) {
			n, err := migrateTopups(ctx, src, target, userMap)
			return fmt.Sprintf("%d topup records", n), err
		}},
		{"options", func() (string, error) {
			n, err := migrateOptions(ctx, src, target)
			return fmt.Sprintf("%d options", n), err
		}},
	}

	total := len(steps)
	for i, st := range steps {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if progress != nil {
			progress(Progress{Step: st.name, Current: i, Total: total})
		}
		detail, err := st.action()
		if err != nil {
			return fmt.Errorf("migrate %s: %w", st.name, err)
		}
		log.Printf("[%d/%d] %s: %s", i+1, total, st.name, detail)
	}

	if progress != nil {
		progress(Progress{Step: "done", Current: total, Total: total, Finished: true})
	}

	return nil
}

func migrateUsers(ctx context.Context, src *sql.DB, target *pgxpool.Pool) (map[int]int64, error) {
	rows, err := src.QueryContext(ctx, `select id,username,password,display_name,role,status,email,quota,used_quota,` +
		"`group`" + `,created_at,COALESCE(setting,''),COALESCE(remark,'') from users`)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	userMap := make(map[int]int64)
	var count int

	for rows.Next() {
		var u xUser
		if err := rows.Scan(&u.ID, &u.Username, &u.Password, &u.DisplayName, &u.Role, &u.Status, &u.Email, &u.Quota, &u.UsedQuota, &u.Group, &u.CreatedAt, &u.Setting, &u.Remark); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}

		if u.Email == "" {
			u.Email = fmt.Sprintf("user%d@migrated.local", u.ID)
		}

		name := u.DisplayName
		if name == "" {
			name = u.Username
		}
		if name == "" {
			name = fmt.Sprintf("user%d", u.ID)
		}

		var role string
		switch u.Role {
		case 10, 100:
			role = "admin"
		default:
			role = "user"
		}

		enabled := u.Status == 1

		id := int64(u.ID)

		var passwordHash *string
		if u.Password != "" {
			passwordHash = &u.Password
		}

		createdAt := time.Unix(u.CreatedAt, 0)

		err = target.QueryRow(ctx, `insert into users(id,email,name,role,password_hash,enabled,created_at)
			values($1,$2,$3,$4,$5,$6,$7) on conflict (email) do update set email=excluded.email returning id`,
			id, strings.ToLower(strings.TrimSpace(u.Email)), name, role, passwordHash, enabled, createdAt).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("insert user %d: %w", u.ID, err)
		}

		balance := float64(u.Quota) / 500000.0 * 0.002
		if balance < 0 {
			balance = 0
		}
		_, err = target.Exec(ctx, `insert into user_wallets(user_id,balance,reserved,updated_at)
			values($1,$2,0,now()) on conflict (user_id) do nothing`, id, balance)
		if err != nil {
			return nil, fmt.Errorf("insert wallet for user %d: %w", u.ID, err)
		}

		userMap[u.ID] = id
		count++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	log.Printf("  users: %d rows processed", count)
	return userMap, nil
}

func migrateTokens(ctx context.Context, src *sql.DB, target *pgxpool.Pool, userMap map[int]int64) (int, error) {
	rows, err := src.QueryContext(ctx, `select id,user_id,`+"`key`"+`,status,name,created_time,accessed_time,
		expired_time,remain_quota,unlimited_quota,used_quota,`+"`group`"+` from tokens`)
	if err != nil {
		return 0, fmt.Errorf("query tokens: %w", err)
	}
	defer rows.Close()

	var count int

	for rows.Next() {
		var t xToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Key, &t.Status, &t.Name, &t.CreatedTime, &t.AccessedTime, &t.ExpiredTime, &t.RemainQuota, &t.UnlimitedQuota, &t.UsedQuota, &t.Group); err != nil {
			return 0, fmt.Errorf("scan token: %w", err)
		}

		targetUserID, ok := userMap[t.UserID]
		if !ok {
			continue
		}

		if t.Key == "" {
			continue
		}

		id, err := newUUID()
		if err != nil {
			return 0, fmt.Errorf("generate uuid: %w", err)
		}

		secretHash := hashSecret(t.Key)
		prefix := t.Key
		if len(prefix) > 8 {
			prefix = prefix[:8]
		}

		var expiresAt *time.Time
		if t.ExpiredTime > 0 {
			tm := time.Unix(t.ExpiredTime, 0)
			expiresAt = &tm
		}
		var revokedAt *time.Time
		if t.Status != 1 {
			tm := time.Now()
			revokedAt = &tm
		}
		var lastUsedAt *time.Time
		if t.AccessedTime > 0 {
			tm := time.Unix(t.AccessedTime, 0)
			lastUsedAt = &tm
		}
		createdAt := time.Unix(t.CreatedTime, 0)

		_, err = target.Exec(ctx, `insert into api_keys(id,user_id,name,key_prefix,secret_hash,expires_at,revoked_at,last_used_at,created_at)
			values($1,$2,$3,$4,$5,$6,$7,$8,$9) on conflict (secret_hash) do nothing`,
			id, targetUserID, t.Name, prefix, secretHash, expiresAt, revokedAt, lastUsedAt, createdAt)
		if err != nil {
			return 0, fmt.Errorf("insert token %d: %w", t.ID, err)
		}

		count++
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("rows iteration: %w", err)
	}

	log.Printf("  tokens: %d rows processed", count)
	return count, nil
}

func migrateGroups(ctx context.Context, src *sql.DB, target *pgxpool.Pool, userMap map[int]int64) (map[string]string, error) {
	groupNames := make(map[string]bool)

	rows, err := src.QueryContext(ctx, `select distinct `+"`group`"+` from users where `+"`group`"+` != ''`)
	if err != nil {
		return nil, fmt.Errorf("query user groups: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			continue
		}
		for _, name := range strings.FieldsFunc(g, func(r rune) bool {
			return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' || r == '\t' || r == ' '
		}) {
			name = strings.TrimSpace(name)
			if name != "" {
				groupNames[name] = true
			}
		}
	}

	chRows, err := src.QueryContext(ctx, `select distinct `+"`group`"+` from channels where `+"`group`"+` != ''`)
	if err == nil {
		defer chRows.Close()
		for chRows.Next() {
			var g string
			if err := chRows.Scan(&g); err != nil {
				continue
			}
			for _, name := range strings.Split(g, ",") {
				name = strings.TrimSpace(name)
				if name != "" {
					groupNames[name] = true
				}
			}
		}
	}

	groupMap := make(map[string]string)
	for name := range groupNames {
		id, err := newUUID()
		if err != nil {
			return nil, fmt.Errorf("generate uuid: %w", err)
		}
		_, err = target.Exec(ctx, `insert into groups(id,name) values($1,$2) on conflict (name) do nothing`,
			id, name)
		if err != nil {
			return nil, fmt.Errorf("insert group %s: %w", name, err)
		}
		groupMap[name] = id
	}

	if err := target.QueryRow(ctx, `select id from groups where name='default'`).Scan(new(string)); err != nil {
		id, err := newUUID()
		if err == nil {
			target.Exec(ctx, `insert into groups(id,name) values($1,'default') on conflict (name) do nothing`, id)
			groupMap["default"] = id
		}
	}

	log.Printf("  groups: %d unique names", len(groupMap))
	return groupMap, nil
}

func migrateChannels(ctx context.Context, src *sql.DB, target *pgxpool.Pool, groupMap map[string]string) (map[int]string, error) {
	rows, err := src.QueryContext(ctx, `select id,type,`+"`key`"+`,name,status,base_url,models,
		`+"`group`"+`,priority,weight,created_time,model_mapping from channels`)
	if err != nil {
		return nil, fmt.Errorf("query channels: %w", err)
	}
	defer rows.Close()

	channelMap := make(map[int]string)
	var count int

	for rows.Next() {
		var c xChannel
		if err := rows.Scan(&c.ID, &c.Type, &c.Key, &c.Name, &c.Status, &c.BaseURL, &c.Models,
			&c.Group, &c.Priority, &c.Weight, &c.CreatedTime, &c.ModelMapping); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}

		id, err := newUUID()
		if err != nil {
			return nil, fmt.Errorf("generate uuid: %w", err)
		}

		provider, ok := channelTypeToProvider[c.Type]
		if !ok {
			provider = "openai"
		}

		baseURL := ""
		if c.BaseURL != nil {
			baseURL = *c.BaseURL
		}

		var models []string
		if c.Models != "" {
			for _, m := range strings.Split(c.Models, ",") {
				m = strings.TrimSpace(m)
				if m != "" {
					models = append(models, m)
				}
			}
		}
		modelsJSON, _ := json.Marshal(models)

		enabled := c.Status == 1

		priority := 100
		if c.Priority != nil {
			priority = int(*c.Priority)
		}

		weight := 100
		if c.Weight != nil {
			weight = int(*c.Weight)
		}

		createdAt := time.Unix(c.CreatedTime, 0)

		_, err = target.Exec(ctx, `insert into channels(id,name,base_url,api_key,models,enabled,priority,weight,provider,created_at,updated_at)
			values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10) on conflict (name) do update set
			base_url=excluded.base_url,models=excluded.models,enabled=excluded.enabled,
			priority=excluded.priority,weight=excluded.weight`,
			id, c.Name, baseURL, c.Key, modelsJSON, enabled, priority, weight, provider, createdAt)
		if err != nil {
			return nil, fmt.Errorf("insert channel %d: %w", c.ID, err)
		}

		channelMap[c.ID] = id

		for _, g := range strings.Split(c.Group, ",") {
			g = strings.TrimSpace(g)
			if g == "" {
				continue
			}
			gid, ok := groupMap[g]
			if !ok {
				continue
			}
			target.Exec(ctx, `insert into channel_groups(channel_id,group_id) values($1,$2) on conflict do nothing`, id, gid)
		}

		count++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	log.Printf("  channels: %d rows processed", count)
	return channelMap, nil
}

func migrateOptions(ctx context.Context, src *sql.DB, target *pgxpool.Pool) (int, error) {
	rows, err := src.QueryContext(ctx, `select `+"`key`"+`,value from `+"`options`")
	if err != nil {
		return 0, fmt.Errorf("query options: %w", err)
	}
	defer rows.Close()

	optionMap := make(map[string]string)
	for rows.Next() {
		var o xOption
		if err := rows.Scan(&o.Key, &o.Value); err != nil {
			continue
		}
		optionMap[o.Key] = o.Value
	}

	var count int

	if ratioJSON, ok := optionMap["ModelRatio"]; ok {
		var ratios map[string]float64
		if json.Unmarshal([]byte(ratioJSON), &ratios) == nil {
			for model, ratio := range ratios {
				pid, err := newUUID()
				if err != nil {
					continue
				}
				price := ratio * 10
				cached := ratio * 5
				output := ratio * 20
				_, err = target.Exec(ctx, `insert into pricing_rules(id,model,input_per_million,cached_input_per_million,output_per_million,multiplier,enabled,created_at,updated_at)
					values($1,$2,$3,$4,$5,$6,true,now(),now()) on conflict (model) do nothing`,
					pid, model, price, cached, output, 1.0)
				if err == nil {
					count++
				}
			}
		}
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("rows iteration: %w", err)
	}

	log.Printf("  pricing rules: %d imported", count)
	return count, nil
}

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func migrateUserGroups(ctx context.Context, src *sql.DB, target *pgxpool.Pool, userMap map[int]int64, groupMap map[string]string) (int, error) {
	rows, err := src.QueryContext(ctx, `select id,`+"`group`"+` from users where `+"`group`"+` != ''`)
	if err != nil {
		return 0, fmt.Errorf("query user groups: %w", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var id int
		var g string
		if err := rows.Scan(&id, &g); err != nil {
			continue
		}
		targetUserID, ok := userMap[id]
		if !ok {
			continue
		}
		for _, name := range strings.FieldsFunc(g, func(r rune) bool {
			return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' || r == '\t' || r == ' '
		}) {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			gid, ok := groupMap[name]
			if !ok {
				continue
			}
			_, err := target.Exec(ctx, `insert into user_groups(user_id,group_id) values($1,$2) on conflict do nothing`,
				targetUserID, gid)
			if err != nil {
				return 0, fmt.Errorf("insert user_group for user %d: %w", id, err)
			}
			count++
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("rows iteration: %w", err)
	}
	log.Printf("  user-group assignments: %d rows inserted", count)
	return count, nil
}

func migrateSubscriptionPlans(ctx context.Context, src *sql.DB, target *pgxpool.Pool, groupMap map[string]string) (map[int]string, error) {
	rows, err := src.QueryContext(ctx, `select id,title,COALESCE(description,''),price_amount,
		COALESCE(currency,'CNY'),duration_unit,duration_value,COALESCE(custom_seconds,0),
		enabled,COALESCE(sort_order,0),COALESCE(max_purchase_per_user,0),
		COALESCE(upgrade_group,''),COALESCE(downgrade_group,''),
		COALESCE(total_amount,0),COALESCE(quota_reset_period,'never'),COALESCE(quota_reset_custom_seconds,0),
		created_at,updated_at from subscription_plans`)
	if err != nil {
		return nil, fmt.Errorf("query subscription plans: %w", err)
	}
	defer rows.Close()

	planMap := make(map[int]string)
	var count int

	for rows.Next() {
		var p xSubscriptionPlan
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.PriceAmount, &p.Currency,
			&p.DurationUnit, &p.DurationValue, &p.CustomSeconds, &p.Enabled, &p.SortOrder,
			&p.MaxPurchasePerUser, &p.UpgradeGroup, &p.DowngradeGroup, &p.TotalAmount,
			&p.QuotaResetPeriod, &p.QuotaResetCustomSeconds, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan subscription plan %d: %w", p.ID, err)
		}

		id, err := newUUID()
		if err != nil {
			return nil, fmt.Errorf("generate uuid: %w", err)
		}

		billingPeriod := "month"
		switch p.DurationUnit {
		case "year":
			billingPeriod = "year"
		case "month":
			billingPeriod = "month"
		}

		creditAmount := quotaToBalance(p.TotalAmount)

		var groupID *string
		if p.UpgradeGroup != "" {
			if gid, ok := groupMap[p.UpgradeGroup]; ok {
				groupID = &gid
			}
		}

		createdAt := time.Unix(p.CreatedAt, 0)
		updatedAt := time.Unix(p.UpdatedAt, 0)

		_, err = target.Exec(ctx, `insert into subscription_plans(id,name,description,price,currency,billing_period,
			credit_amount,group_id,sort_order,enabled,created_at,updated_at)
			values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) on conflict (name) do nothing`,
			id, p.Title, p.Description, p.PriceAmount, p.Currency, billingPeriod,
			creditAmount, groupID, p.SortOrder, p.Enabled, createdAt, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("insert subscription plan %d: %w", p.ID, err)
		}

		planMap[p.ID] = id
		count++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	log.Printf("  subscription plans: %d rows processed", count)
	return planMap, nil
}

func migrateUserSubscriptions(ctx context.Context, src *sql.DB, target *pgxpool.Pool, userMap map[int]int64, planMap map[int]string) (map[int]string, int, error) {
	rows, err := src.QueryContext(ctx, `select id,user_id,plan_id,COALESCE(price_amount,0),
		COALESCE(amount_total,0),COALESCE(amount_used,0),start_time,end_time,status,
		COALESCE(source,'order'),COALESCE(last_reset_time,0),COALESCE(next_reset_time,0),
		COALESCE(upgrade_group,''),COALESCE(prev_user_group,''),COALESCE(downgrade_group,''),
		COALESCE(allow_wallet_overflow,true),created_at,updated_at from user_subscriptions`)
	if err != nil {
		return nil, 0, fmt.Errorf("query user subscriptions: %w", err)
	}
	defer rows.Close()

	subMap := make(map[int]string)
	var count int
	for rows.Next() {
		var s xUserSubscription
		if err := rows.Scan(&s.ID, &s.UserID, &s.PlanID, &s.PriceAmount,
			&s.AmountTotal, &s.AmountUsed, &s.StartTime, &s.EndTime, &s.Status,
			&s.Source, &s.LastResetTime, &s.NextResetTime,
			&s.UpgradeGroup, &s.PrevUserGroup, &s.DowngradeGroup,
			&s.AllowWalletOverflow, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan user subscription %d: %w", s.ID, err)
		}

		targetUserID, ok := userMap[s.UserID]
		if !ok {
			continue
		}
		targetPlanID, ok := planMap[s.PlanID]
		if !ok {
			continue
		}

		id, err := newUUID()
		if err != nil {
			return nil, 0, fmt.Errorf("generate uuid: %w", err)
		}

		var periodStart, periodEnd, cancelledAt *time.Time
		if s.StartTime > 0 {
			tm := time.Unix(s.StartTime, 0)
			periodStart = &tm
		}
		if s.EndTime > 0 {
			tm := time.Unix(s.EndTime, 0)
			periodEnd = &tm
		}
		if s.Status == "cancelled" {
			tm := time.Now()
			cancelledAt = &tm
		}

		createdAt := time.Unix(s.CreatedAt, 0)

		_, err = target.Exec(ctx, `insert into user_subscriptions(id,user_id,plan_id,status,
			current_period_start,current_period_end,cancelled_at,created_at,updated_at)
			values($1,$2,$3,$4,$5,$6,$7,$8,$9) on conflict (id) do nothing`,
			id, targetUserID, targetPlanID, s.Status,
			periodStart, periodEnd, cancelledAt, createdAt, createdAt)
		if err != nil {
			return nil, 0, fmt.Errorf("insert user subscription %d: %w", s.ID, err)
		}

		subMap[s.ID] = id
		count++
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows iteration: %w", err)
	}

	log.Printf("  user subscriptions: %d rows processed", count)
	return subMap, count, nil
}

func migrateSubscriptionOrders(ctx context.Context, src *sql.DB, target *pgxpool.Pool, userMap map[int]int64, planMap map[int]string, subMap map[int]string) (int, error) {
	rows, err := src.QueryContext(ctx, `select id,user_id,plan_id,COALESCE(money,0),
		COALESCE(trade_no,''),COALESCE(payment_method,''),COALESCE(payment_provider,''),
		COALESCE(gateway_id,''),status,create_time,COALESCE(complete_time,0),
		COALESCE(upgrade_subscription_id,0),COALESCE(provider_payload,'')
		from subscription_orders`)
	if err != nil {
		return 0, fmt.Errorf("query subscription orders: %w", err)
	}
	defer rows.Close()

	orderStatusMap := map[string]string{
		"pending": "pending",
		"success": "paid",
		"failed":  "failed",
		"expired": "expired",
	}

	var count int
	for rows.Next() {
		var o xSubscriptionOrder
		if err := rows.Scan(&o.ID, &o.UserID, &o.PlanID, &o.Money,
			&o.TradeNo, &o.PaymentMethod, &o.PaymentProvider, &o.GatewayID,
			&o.Status, &o.CreateTime, &o.CompleteTime,
			&o.UpgradeSubscriptionID, &o.ProviderPayload); err != nil {
			return 0, fmt.Errorf("scan subscription order %d: %w", o.ID, err)
		}

		targetUserID, ok := userMap[o.UserID]
		if !ok {
			continue
		}
		targetPlanID, ok := planMap[o.PlanID]
		if !ok {
			continue
		}

		id, err := newUUID()
		if err != nil {
			return 0, fmt.Errorf("generate uuid: %w", err)
		}

		status, ok := orderStatusMap[o.Status]
		if !ok {
			status = "pending"
		}

		var paidAt *time.Time
		if o.CompleteTime > 0 && status == "paid" {
			tm := time.Unix(o.CompleteTime, 0)
			paidAt = &tm
		}

		createdAt := time.Unix(o.CreateTime, 0)

		_, err = target.Exec(ctx, `insert into subscription_orders(id,order_no,user_id,plan_id,
			provider,payment_type,amount,status,provider_trade_no,paid_at,created_at,updated_at)
			values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) on conflict (order_no) do nothing`,
			id, o.TradeNo, targetUserID, targetPlanID,
			"epay", o.PaymentMethod, o.Money, status, o.TradeNo, paidAt, createdAt, createdAt)
		if err != nil {
			return 0, fmt.Errorf("insert subscription order %d: %w", o.ID, err)
		}

		count++
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("rows iteration: %w", err)
	}

	log.Printf("  subscription orders: %d rows processed", count)
	return count, nil
}

func migrateTopups(ctx context.Context, src *sql.DB, target *pgxpool.Pool, userMap map[int]int64) (int, error) {
	rows, err := src.QueryContext(ctx, `select id,user_id,amount,money,trade_no,
		COALESCE(payment_method,''),COALESCE(payment_provider,''),
		COALESCE(gateway_id,''),create_time,complete_time,status from top_ups where status = 'success'`)
	if err != nil {
		return 0, fmt.Errorf("query topups: %w", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var t xTopUp
		if err := rows.Scan(&t.ID, &t.UserID, &t.Amount, &t.Money, &t.TradeNo,
			&t.PaymentMethod, &t.PaymentProvider, &t.GatewayID,
			&t.CreateTime, &t.CompleteTime, &t.Status); err != nil {
			return 0, fmt.Errorf("scan topup %d: %w", t.ID, err)
		}

		targetUserID, ok := userMap[t.UserID]
		if !ok {
			continue
		}

		amount := t.Money
		if amount <= 0 {
			continue
		}

		createdAt := time.Unix(t.CreateTime, 0)

		ledgerID, err := newUUID()
		if err != nil {
			return 0, fmt.Errorf("generate uuid: %w", err)
		}

		var currentBalance float64
		err = target.QueryRow(ctx,
			`select balance from user_wallets where user_id=$1`, targetUserID).Scan(&currentBalance)
		if err != nil {
			continue
		}
		balanceAfter := currentBalance + amount

		_, err = target.Exec(ctx, `insert into wallet_ledger(id,user_id,amount,balance_after,kind,note,created_at)
			values($1,$2,$3,$4,'topup',$5,$6) on conflict (id) do nothing`,
			ledgerID, targetUserID, amount, balanceAfter, fmt.Sprintf("migrated topup #%d: %s", t.ID, t.TradeNo), createdAt)
		if err != nil {
			return 0, fmt.Errorf("insert ledger entry for topup %d: %w", t.ID, err)
		}

		_, err = target.Exec(ctx, `update user_wallets set balance=$1,updated_at=$2 where user_id=$3`,
			balanceAfter, createdAt, targetUserID)
		if err != nil {
			return 0, fmt.Errorf("update wallet for topup %d: %w", t.ID, err)
		}

		count++
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("rows iteration: %w", err)
	}

	log.Printf("  topup records: %d rows processed", count)
	return count, nil
}
