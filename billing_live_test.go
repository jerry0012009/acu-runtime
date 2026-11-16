package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
)

// Opt-in read-only acceptance exercises the actual Runtime account handler.
func TestAccountBillingLiveAcceptance(t *testing.T) {
	path := os.Getenv("ACU_BILLING_LIVE_SESSION")
	if path == "" {
		t.Skip("explicit authenticated live acceptance only")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var auth struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(raw, &auth) != nil || auth.AccessToken == "" {
		t.Fatal("session unavailable")
	}
	host := &server{token: "acceptance-local"}
	host.account.view = accountView{LoggedIn: true, Site: "https://acu.best", Username: "wzr0012026"}
	host.account.userID, host.account.bearer, host.account.expires = 122, auth.AccessToken, time.Now().Add(time.Hour).Unix()
	days := os.Getenv("ACU_BILLING_LIVE_DAYS")
	if days == "" {
		days = "7"
	}
	if days != "1" && days != "7" && days != "30" {
		t.Fatal("unsupported live account range")
	}
	for i := 0; i < 2; i++ {
		start := time.Now()
		out := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/runtime/billing/account?days="+days+"&page=1", nil)
		req.Header.Set("X-ACU-Token", host.token)
		host.accountBilling(out, req)
		if out.Code != 200 {
			t.Fatalf("billing returned %d", out.Code)
		}
		var result struct {
			Overview struct {
				Count  int64  `json:"count"`
				Charge string `json:"chargeCny"`
				Exact  int64  `json:"exactChargeCount"`
				Legacy int64  `json:"legacyChargeCount"`
				Models []struct {
					Count  int64  `json:"count"`
					Charge string `json:"chargeCny"`
				} `json:"models"`
				Comparison struct {
					Percent *string `json:"savedPercent"`
					Count   int64   `json:"count"`
				} `json:"comparison"`
			} `json:"overview"`
			Warnings map[string]string `json:"warnings"`
			Summary  struct {
				Charge string `json:"user_charge_cny"`
				Exact  int64  `json:"exact_charge_count"`
				Legacy int64  `json:"legacy_charge_count"`
			} `json:"summary"`
		}
		if json.Unmarshal(out.Body.Bytes(), &result) != nil || result.Overview.Comparison.Percent == nil || result.Overview.Comparison.Count <= 0 || len(result.Warnings) > 0 {
			t.Fatalf("incomplete live account result: count=%d warnings=%v", result.Overview.Count, result.Warnings)
		}
		if result.Summary.Charge != result.Overview.Charge || result.Summary.Exact != result.Overview.Exact || result.Summary.Legacy != result.Overview.Legacy || result.Overview.Exact+result.Overview.Legacy != result.Overview.Count {
			t.Fatal("summary and overview billing totals disagree")
		}
		var count int64
		var charge float64
		for _, row := range result.Overview.Models {
			value, err := strconv.ParseFloat(row.Charge, 64)
			if err != nil {
				t.Fatal("invalid model charge")
			}
			count += row.Count
			charge += value
		}
		total, err := strconv.ParseFloat(result.Overview.Charge, 64)
		if err != nil || count != result.Overview.Count || math.Abs(total-charge) > 1e-8 {
			t.Fatal("model ranking and account totals disagree")
		}
		if err := os.WriteFile(os.Getenv("ACU_BILLING_LIVE_OUTPUT"), out.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		fmt.Printf("LIVE Runtime billing %sd read %d: %dms; records=%d comparable=%d\n", days, i+1, time.Since(start).Milliseconds(), result.Overview.Count, result.Overview.Comparison.Count)
	}
}
