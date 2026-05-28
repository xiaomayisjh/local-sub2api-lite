package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/repository/sqldialect"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *channelRepository) batchLoadAccountStatsPricingRules(ctx context.Context, channelIDs []int64) (map[int64][]service.AccountStatsPricingRule, error) {
	if len(channelIDs) == 0 {
		return map[int64][]service.AccountStatsPricingRule{}, nil
	}

	var (
		rows *sql.Rows
		err  error
	)
	if sqldialect.UsesSQLite() {
		rows, err = r.db.QueryContext(ctx,
			fmt.Sprintf(
				`SELECT id, channel_id, name, group_ids, account_ids, sort_order, created_at, updated_at
				 FROM channel_account_stats_pricing_rules
				 WHERE channel_id IN (%s)
				 ORDER BY channel_id, sort_order, id`,
				numberedPlaceholders(1, len(channelIDs)),
			),
			int64SliceArgs(channelIDs)...,
		)
	} else {
		rows, err = r.db.QueryContext(ctx,
			`SELECT id, channel_id, name, group_ids, account_ids, sort_order, created_at, updated_at
			 FROM channel_account_stats_pricing_rules
			 WHERE channel_id = ANY($1)
			 ORDER BY channel_id, sort_order, id`,
			pq.Array(channelIDs),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("batch load account stats pricing rules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	allRules, ruleIDs, err := scanAccountStatsPricingRules(rows)
	if err != nil {
		return nil, err
	}

	pricingMap, err := r.batchLoadAccountStatsModelPricing(ctx, ruleIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[int64][]service.AccountStatsPricingRule, len(channelIDs))
	for i := range allRules {
		allRules[i].Pricing = pricingMap[allRules[i].ID]
		result[allRules[i].ChannelID] = append(result[allRules[i].ChannelID], allRules[i])
	}
	return result, nil
}

func scanAccountStatsPricingRules(rows *sql.Rows) ([]service.AccountStatsPricingRule, []int64, error) {
	var allRules []service.AccountStatsPricingRule
	var ruleIDs []int64
	for rows.Next() {
		var rule service.AccountStatsPricingRule
		if sqldialect.UsesSQLite() {
			var groupIDsJSON, accountIDsJSON []byte
			if err := rows.Scan(
				&rule.ID, &rule.ChannelID, &rule.Name,
				&groupIDsJSON, &accountIDsJSON,
				&rule.SortOrder, &rule.CreatedAt, &rule.UpdatedAt,
			); err != nil {
				return nil, nil, fmt.Errorf("scan account stats pricing rule: %w", err)
			}
			rule.GroupIDs = decodeInt64JSON(groupIDsJSON)
			rule.AccountIDs = decodeInt64JSON(accountIDsJSON)
		} else {
			if err := rows.Scan(
				&rule.ID, &rule.ChannelID, &rule.Name,
				pq.Array(&rule.GroupIDs), pq.Array(&rule.AccountIDs),
				&rule.SortOrder, &rule.CreatedAt, &rule.UpdatedAt,
			); err != nil {
				return nil, nil, fmt.Errorf("scan account stats pricing rule: %w", err)
			}
		}
		ruleIDs = append(ruleIDs, rule.ID)
		allRules = append(allRules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate account stats pricing rules: %w", err)
	}
	return allRules, ruleIDs, nil
}

func (r *channelRepository) batchLoadAccountStatsModelPricing(ctx context.Context, ruleIDs []int64) (map[int64][]service.ChannelModelPricing, error) {
	if len(ruleIDs) == 0 {
		return make(map[int64][]service.ChannelModelPricing), nil
	}

	var (
		rows *sql.Rows
		err  error
	)
	if sqldialect.UsesSQLite() {
		rows, err = r.db.QueryContext(ctx,
			fmt.Sprintf(
				`SELECT id, rule_id, platform, models, billing_mode, input_price, output_price,
				        cache_write_price, cache_read_price, image_output_price, per_request_price, created_at, updated_at
				 FROM channel_account_stats_model_pricing
				 WHERE rule_id IN (%s)
				 ORDER BY rule_id, id`,
				numberedPlaceholders(1, len(ruleIDs)),
			),
			int64SliceArgs(ruleIDs)...,
		)
	} else {
		rows, err = r.db.QueryContext(ctx,
			`SELECT id, rule_id, platform, models, billing_mode, input_price, output_price,
			        cache_write_price, cache_read_price, image_output_price, per_request_price, created_at, updated_at
			 FROM channel_account_stats_model_pricing
			 WHERE rule_id = ANY($1)
			 ORDER BY rule_id, id`,
			pq.Array(ruleIDs),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("batch load account stats model pricing: %w", err)
	}
	defer func() { _ = rows.Close() }()

	pricingMap := make(map[int64][]service.ChannelModelPricing, len(ruleIDs))
	var allPricingIDs []int64
	for rows.Next() {
		var p service.ChannelModelPricing
		var ruleID int64
		var modelsJSON []byte
		if err := rows.Scan(
			&p.ID, &ruleID, &p.Platform, &modelsJSON, &p.BillingMode,
			&p.InputPrice, &p.OutputPrice, &p.CacheWritePrice, &p.CacheReadPrice,
			&p.ImageOutputPrice, &p.PerRequestPrice, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan account stats model pricing: %w", err)
		}
		if err := json.Unmarshal(modelsJSON, &p.Models); err != nil {
			p.Models = []string{}
		}
		allPricingIDs = append(allPricingIDs, p.ID)
		pricingMap[ruleID] = append(pricingMap[ruleID], p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate account stats model pricing: %w", err)
	}

	if len(allPricingIDs) > 0 {
		intervalsMap, err := r.batchLoadAccountStatsIntervals(ctx, allPricingIDs)
		if err != nil {
			return nil, err
		}
		for ruleID, pricings := range pricingMap {
			for i := range pricings {
				pricings[i].Intervals = intervalsMap[pricings[i].ID]
			}
			pricingMap[ruleID] = pricings
		}
	}
	return pricingMap, nil
}

func (r *channelRepository) loadAccountStatsPricingRules(ctx context.Context, channelID int64) ([]service.AccountStatsPricingRule, error) {
	result, err := r.batchLoadAccountStatsPricingRules(ctx, []int64{channelID})
	if err != nil {
		return nil, err
	}
	return result[channelID], nil
}

func replaceAccountStatsPricingRulesTx(ctx context.Context, exec dbExec, channelID int64, rules []service.AccountStatsPricingRule) error {
	if _, err := exec.ExecContext(ctx,
		`DELETE FROM channel_account_stats_pricing_rules WHERE channel_id = $1`, channelID,
	); err != nil {
		return fmt.Errorf("delete old account stats pricing rules: %w", err)
	}

	for i := range rules {
		rules[i].ChannelID = channelID
		if err := createAccountStatsPricingRuleTx(ctx, exec, &rules[i]); err != nil {
			return fmt.Errorf("insert account stats pricing rule: %w", err)
		}
	}
	return nil
}

func createAccountStatsPricingRuleTx(ctx context.Context, exec dbExec, rule *service.AccountStatsPricingRule) error {
	var groupIDsArg any = pq.Array(rule.GroupIDs)
	var accountIDsArg any = pq.Array(rule.AccountIDs)
	if sqldialect.UsesSQLite() {
		groupIDsJSON, err := marshalInt64JSON(rule.GroupIDs)
		if err != nil {
			return err
		}
		accountIDsJSON, err := marshalInt64JSON(rule.AccountIDs)
		if err != nil {
			return err
		}
		groupIDsArg = groupIDsJSON
		accountIDsArg = accountIDsJSON
	}

	err := exec.QueryRowContext(ctx,
		`INSERT INTO channel_account_stats_pricing_rules (channel_id, name, group_ids, account_ids, sort_order)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at, updated_at`,
		rule.ChannelID, rule.Name, groupIDsArg, accountIDsArg, rule.SortOrder,
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert account stats pricing rule: %w", err)
	}

	for j := range rule.Pricing {
		if err := createAccountStatsModelPricingTx(ctx, exec, rule.ID, &rule.Pricing[j]); err != nil {
			return err
		}
	}
	return nil
}

func createAccountStatsModelPricingTx(ctx context.Context, exec dbExec, ruleID int64, pricing *service.ChannelModelPricing) error {
	modelsJSON, err := json.Marshal(pricing.Models)
	if err != nil {
		return fmt.Errorf("marshal models: %w", err)
	}
	billingMode := pricing.BillingMode
	if billingMode == "" {
		billingMode = service.BillingModeToken
	}
	platform := pricing.Platform
	err = exec.QueryRowContext(ctx,
		`INSERT INTO channel_account_stats_model_pricing (rule_id, platform, models, billing_mode, input_price, output_price, cache_write_price, cache_read_price, image_output_price, per_request_price)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id, created_at, updated_at`,
		ruleID, platform, modelsJSON, billingMode,
		pricing.InputPrice, pricing.OutputPrice, pricing.CacheWritePrice, pricing.CacheReadPrice,
		pricing.ImageOutputPrice, pricing.PerRequestPrice,
	).Scan(&pricing.ID, &pricing.CreatedAt, &pricing.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert account stats model pricing: %w", err)
	}
	for i := range pricing.Intervals {
		iv := &pricing.Intervals[i]
		iv.PricingID = pricing.ID
		if err := createAccountStatsIntervalTx(ctx, exec, iv); err != nil {
			return err
		}
	}
	return nil
}

func createAccountStatsIntervalTx(ctx context.Context, exec dbExec, iv *service.PricingInterval) error {
	return exec.QueryRowContext(ctx,
		`INSERT INTO channel_account_stats_pricing_intervals
		 (pricing_id, min_tokens, max_tokens, tier_label, input_price, output_price, cache_write_price, cache_read_price, per_request_price, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id, created_at, updated_at`,
		iv.PricingID, iv.MinTokens, iv.MaxTokens, iv.TierLabel,
		iv.InputPrice, iv.OutputPrice, iv.CacheWritePrice, iv.CacheReadPrice,
		iv.PerRequestPrice, iv.SortOrder,
	).Scan(&iv.ID, &iv.CreatedAt, &iv.UpdatedAt)
}

func (r *channelRepository) batchLoadAccountStatsIntervals(ctx context.Context, pricingIDs []int64) (map[int64][]service.PricingInterval, error) {
	if len(pricingIDs) == 0 {
		return map[int64][]service.PricingInterval{}, nil
	}

	var (
		rows *sql.Rows
		err  error
	)
	if sqldialect.UsesSQLite() {
		rows, err = r.db.QueryContext(ctx,
			fmt.Sprintf(
				`SELECT id, pricing_id, min_tokens, max_tokens, tier_label,
				        input_price, output_price, cache_write_price, cache_read_price,
				        per_request_price, sort_order, created_at, updated_at
				 FROM channel_account_stats_pricing_intervals
				 WHERE pricing_id IN (%s)
				 ORDER BY pricing_id, sort_order, id`,
				numberedPlaceholders(1, len(pricingIDs)),
			),
			int64SliceArgs(pricingIDs)...,
		)
	} else {
		rows, err = r.db.QueryContext(ctx,
			`SELECT id, pricing_id, min_tokens, max_tokens, tier_label,
			        input_price, output_price, cache_write_price, cache_read_price,
			        per_request_price, sort_order, created_at, updated_at
			 FROM channel_account_stats_pricing_intervals
			 WHERE pricing_id = ANY($1)
			 ORDER BY pricing_id, sort_order, id`,
			pq.Array(pricingIDs),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("batch load account stats pricing intervals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanPricingIntervals(rows, len(pricingIDs), "account stats pricing interval")
}

func marshalInt64JSON(values []int64) (string, error) {
	if values == nil {
		values = []int64{}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal int64 json: %w", err)
	}
	return string(raw), nil
}

func decodeInt64JSON(raw []byte) []int64 {
	if len(raw) == 0 {
		return nil
	}
	var values []int64
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	return values
}
