// claude-burn server: receives per-day, per-machine coding-agent usage and
// serves an aggregated view (today / 7d / 30d + trend + 7-day sparkline).
// Single binary, SQLite storage, token auth.
package main

import (
	_ "embed"

	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "time/tzdata" // embed tz database so BURN_TZ works without OS tzdata

	_ "modernc.org/sqlite"
)

//go:embed install.sh
var installScript string

//go:embed dashboard.html
var dashboardHTML string

type dayRow struct {
	Date                string  `json:"date"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	CostUSD             float64 `json:"cost_usd"`
}

type pushBody struct {
	Machine string   `json:"machine"`
	Days    []dayRow `json:"days"`
}

type rangeSum struct {
	Tokens int64   `json:"tokens"`
	Cost   float64 `json:"cost_usd"`
}

type rangeStat struct {
	Tokens    int64    `json:"tokens"`    // everything incl. cache
	Prompted  int64    `json:"prompted"`  // input + output only (the "real work")
	Cost      float64  `json:"cost_usd"`
	ChangePct *float64 `json:"change_pct"` // vs the previous equal window; null if no baseline
}

type machineStat struct {
	Machine   string   `json:"machine"`
	UpdatedAt string   `json:"updated_at"`
	Stale     bool     `json:"stale"`
	Today     rangeSum `json:"today"`
	D7        rangeSum `json:"7d"`
	D30       rangeSum `json:"30d"`
}

type sparkPoint struct {
	Date   string  `json:"date"`
	Tokens int64   `json:"tokens"`
	Cost   float64 `json:"cost_usd"`
}

var (
	db       *sql.DB
	token    string
	loc      *time.Location
	staleMin int
)

func main() {
	token = os.Getenv("BURN_TOKEN")
	if token == "" {
		log.Fatal("BURN_TOKEN is required")
	}
	tz := envOr("BURN_TZ", "UTC")
	var err error
	if loc, err = time.LoadLocation(tz); err != nil {
		log.Printf("unknown BURN_TZ %q, falling back to UTC", tz)
		loc = time.UTC
	}
	staleMin, _ = strconv.Atoi(envOr("BURN_STALE_MIN", "180"))
	if staleMin <= 0 {
		staleMin = 180
	}
	dbPath := envOr("BURN_DB", "/data/burn.db")
	port := envOr("PORT", "8080")

	if db, err = sql.Open("sqlite", dbPath); err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := initDB(); err != nil {
		log.Fatalf("init db: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]any{"ok": true}) })
	mux.HandleFunc("/push", auth(handlePush))
	mux.HandleFunc("/stats", auth(handleStats))
	mux.HandleFunc("/install.sh", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/x-shellscript")
		_, _ = w.Write([]byte(installScript))
	})
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(dashboardHTML))
	})

	log.Printf("claude-burn server on :%s (tz=%s stale=%dm db=%s)", port, loc, staleMin, dbPath)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func initDB() error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS usage_daily (
	machine               TEXT    NOT NULL,
	day                   TEXT    NOT NULL,
	input_tokens          INTEGER NOT NULL,
	output_tokens         INTEGER NOT NULL,
	cache_creation_tokens INTEGER NOT NULL,
	cache_read_tokens     INTEGER NOT NULL,
	cost_usd              REAL    NOT NULL,
	PRIMARY KEY (machine, day)
);
CREATE TABLE IF NOT EXISTS machine_seen (
	machine   TEXT PRIMARY KEY,
	last_push TEXT NOT NULL
);`)
	return err
}

func auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handlePush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var b pushBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(b.Machine) == "" {
		http.Error(w, "machine required", http.StatusBadRequest)
		return
	}
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	for _, d := range b.Days {
		if strings.TrimSpace(d.Date) == "" {
			continue
		}
		if _, err := tx.Exec(`
INSERT INTO usage_daily (machine, day, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, cost_usd)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(machine, day) DO UPDATE SET
	input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens,
	cache_creation_tokens=excluded.cache_creation_tokens, cache_read_tokens=excluded.cache_read_tokens,
	cost_usd=excluded.cost_usd
`, b.Machine, d.Date, d.InputTokens, d.OutputTokens, d.CacheCreationTokens, d.CacheReadTokens, d.CostUSD); err != nil {
			log.Printf("push upsert: %v", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec(`INSERT INTO machine_seen (machine, last_push) VALUES (?, ?)
ON CONFLICT(machine) DO UPDATE SET last_push=excluded.last_push`, b.Machine, now); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "machine": b.Machine, "days": len(b.Days)})
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	day := func(off int) string { return now.In(loc).AddDate(0, 0, off).Format("2006-01-02") }

	ranges := map[string]rangeStat{
		"today": makeRange(day(0), day(0), day(-1), day(-1)),
		"7d":    makeRange(day(-6), day(0), day(-13), day(-7)),
		"30d":   makeRange(day(-29), day(0), day(-59), day(-30)),
	}

	machines, err := machineStats(day)
	if err != nil {
		log.Printf("stats machines: %v", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	spark := sparkline(day)

	writeJSON(w, map[string]any{
		"generated_at": now.UTC().Format(time.RFC3339),
		"ranges":       ranges,
		"spark":        spark,
		"machines":     machines,
	})
}

// makeRange sums the current window and computes % change vs the previous window (by cost).
func makeRange(curFrom, curTo, prevFrom, prevTo string) rangeStat {
	tok, prompted, cost := sumBetween(curFrom, curTo)
	_, _, prevCost := sumBetween(prevFrom, prevTo)
	var chg *float64
	if prevCost > 0 {
		v := math.Round((cost-prevCost)/prevCost*1000) / 10
		if math.Abs(v) <= 999 { // ignore absurd swings when the prior window is near-empty
			chg = &v
		}
	}
	return rangeStat{Tokens: tok, Prompted: prompted, Cost: cost, ChangePct: chg}
}

func sumBetween(from, to string) (int64, int64, float64) {
	var tok, prompted int64
	var cost float64
	db.QueryRow(`SELECT COALESCE(SUM(input_tokens+output_tokens+cache_creation_tokens+cache_read_tokens),0),
		COALESCE(SUM(input_tokens+output_tokens),0),
		COALESCE(SUM(cost_usd),0) FROM usage_daily WHERE day BETWEEN ? AND ?`, from, to).Scan(&tok, &prompted, &cost)
	return tok, prompted, cost
}

func machineBetween(from, to string) (map[string]rangeSum, error) {
	rows, err := db.Query(`SELECT machine,
		COALESCE(SUM(input_tokens+output_tokens+cache_creation_tokens+cache_read_tokens),0),
		COALESCE(SUM(cost_usd),0) FROM usage_daily WHERE day BETWEEN ? AND ? GROUP BY machine`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]rangeSum{}
	for rows.Next() {
		var name string
		var v rangeSum
		if err := rows.Scan(&name, &v.Tokens, &v.Cost); err != nil {
			return nil, err
		}
		m[name] = v
	}
	return m, rows.Err()
}

func machineStats(day func(int) string) ([]machineStat, error) {
	today, err := machineBetween(day(0), day(0))
	if err != nil {
		return nil, err
	}
	d7, err := machineBetween(day(-6), day(0))
	if err != nil {
		return nil, err
	}
	d30, err := machineBetween(day(-29), day(0))
	if err != nil {
		return nil, err
	}

	seen := map[string]string{}
	rows, err := db.Query(`SELECT machine, last_push FROM machine_seen`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, lp string
		if err := rows.Scan(&name, &lp); err != nil {
			return nil, err
		}
		seen[name] = lp
	}

	names := map[string]bool{}
	for n := range d30 {
		names[n] = true
	}
	for n := range seen {
		names[n] = true
	}

	staleCut := time.Now().Add(-time.Duration(staleMin) * time.Minute)
	out := []machineStat{}
	for n := range names {
		ms := machineStat{Machine: n, UpdatedAt: seen[n], Today: today[n], D7: d7[n], D30: d30[n]}
		if t, err := time.Parse(time.RFC3339, seen[n]); err == nil {
			ms.Stale = t.Before(staleCut)
		} else {
			ms.Stale = true
		}
		out = append(out, ms)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].D30.Cost > out[j].D30.Cost })
	return out, nil
}

func sparkline(day func(int) string) []sparkPoint {
	byDay := map[string]sparkPoint{}
	rows, err := db.Query(`SELECT day,
		COALESCE(SUM(input_tokens+output_tokens+cache_creation_tokens+cache_read_tokens),0),
		COALESCE(SUM(cost_usd),0) FROM usage_daily WHERE day BETWEEN ? AND ? GROUP BY day`, day(-6), day(0))
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p sparkPoint
			if err := rows.Scan(&p.Date, &p.Tokens, &p.Cost); err == nil {
				byDay[p.Date] = p
			}
		}
	}
	out := make([]sparkPoint, 7)
	for i := 0; i < 7; i++ {
		d := day(i - 6)
		if p, ok := byDay[d]; ok {
			out[i] = p
			out[i].Date = d
		} else {
			out[i] = sparkPoint{Date: d}
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
