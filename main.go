package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/fioprotocol/fio-go"
	"github.com/fioprotocol/fio-go/eos"
	"github.com/go-redis/redis/v8"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"log"
	"math"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	circulatingTokens        float64
	mintedTokens             float64
	lockedTokens             float64
	bpRewards                float64
	bpBucketPool             float64
	url, redisUrl, redisPass string
	redisDb                  int
	redisTls                 bool
	refreshed                time.Time
	stakingSuf, stakingWhole []byte
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	var port string
	flag.StringVar(&redisUrl, "r", "127.0.0.1:6379", "redis url for storing historical APR data")
	flag.StringVar(&redisPass, "pass", "", "redis password for storing historical APR data")
	flag.IntVar(&redisDb, "db", redisDb, "redis DB to use")
	flag.StringVar(&url, "u", "", "url to connect to")
	flag.StringVar(&port, "p", "8080", "port to listen on")
	flag.BoolVar(&redisTls, "tls", false, "Enable TLS to redis db")
	flag.Parse()
	if url == "" {
		url = os.Getenv("URL")
		if url == "" {
			log.Fatal("no url specified, use '-u' or URL env var.")
		}
	}
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	if os.Getenv("REDIS") != "" {
		redisUrl = os.Getenv("REDIS")
	}
	if os.Getenv("REDIS_PASS") != "" {
		redisPass = os.Getenv("REDIS_PASS")
	}
	if strings.ToLower(os.Getenv("REDIS_TLS")) == "true" {
		redisTls = true
	}

	stakingSuf, stakingWhole = []byte("{}"), []byte("{}")

	abortChan := make(chan interface{}, 1)
	defer close(abortChan)
	go updateStats(abortChan)

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getFormat(r *http.Request) (asSuf bool, asJson bool, whole bool, stat string) {
	path := strings.Split(r.URL.Path, "/")
	stat = path[len(path)-1]
	q := r.URL.Query()
	if q["json"] != nil {
		asJson = true
	}
	switch path[len(path)-1] {
	case "suf":
		asSuf = true
		stat = path[len(path)-2]
	case "int":
		whole = true
		stat = path[len(path)-2]
	}
	return
}

func formatter(unsigned bool, pretty bool, whole bool, desc string, amount float64) string {
	num := strconv.FormatFloat(amount, 'f', 9, 64)
	if whole {
		num = strconv.FormatFloat(math.Round(amount), 'f', 0, 64)
	}
	if unsigned {
		num = strings.ReplaceAll(num, ".", "")
	}
	if pretty {
		return fmt.Sprintf(`{"%s":%s}`, desc, num)
	}
	return num
}

func handler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%+v\n", r)
	defer r.Body.Close()
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Origin, Accept")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	}
	w.Header().Set("access-control-allow-origin", "*")

	if strings.Contains(r.URL.Path, "staking") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("x-last-refreshed", refreshed.UTC().Format(time.RFC1123))
		// if we are going to send an empty response throw a 500 error.
		if len(stakingSuf) == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(stakingSuf)
			return
		}
		w.WriteHeader(http.StatusOK)
		if strings.HasSuffix(r.URL.Path, "suf") {
			_, _ = w.Write(stakingSuf)
			return
		}
		_, _ = w.Write(stakingWhole)
		return
	}

	if mintedTokens == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write(nil)
		if err != nil {
			log.Println(err)
		}
		return
	}
	asSuf, asJson, whole, stat := getFormat(r)
	var amount float64
	var desc string
	switch stat {
	case "minted", "supply":
		amount = mintedTokens
		desc = "total_supply"
	case "circulating":
		amount = circulatingTokens
		desc = "circulating_supply"
	case "locked":
		amount = lockedTokens
		desc = "locked_tokens"
	case "bprewards":
		amount = bpRewards
		desc = "bp_rewards"
	case "bpbucket":
		amount = bpBucketPool
		desc = "bp_bucket_pool"
	default:
		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write(nil)
		if err != nil {
			log.Println(err)
		}
		return
	}
	if asJson {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.Header().Set("x-last-refreshed", refreshed.UTC().Format(time.RFC1123))
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(formatter(asSuf, asJson, whole, desc, amount)))
	if err != nil {
		log.Println(err)
	}
	return
}

func updateStats(abort chan interface{}) {
	log.Println("update stats starting")
	update := func() error {
		var lastErr error
		api, _, err := fio.NewConnection(nil, url)
		if err != nil {
			return err
		}
		var m, p, l, c uint64
		c, m, l, err = api.GetCirculatingSupply()
		if err != nil {
			log.Println(err)
			lastErr = err
			return err
		}
		circulatingTokens = float64(c) / 1_000_000_000.0
		mintedTokens = float64(m) / 1_000_000_000.0
		lockedTokens = float64(l) / 1_000_000_000.0

		p, err = api.GetLockedBpRewards()
		if err != nil {
			log.Println(err)
			lastErr = err
			return err
		}
		bpBucketPool = float64(p) / 1_000_000_000.0

		var bpr float64
		bpr, err = getBpRewards("bprewards", api)
		if err != nil {
			log.Println(err)
			lastErr = err
		} else if bpr > 0 {
			bpRewards = bpr
		}
		stakingWhole, stakingSuf, err = UpdateApr()
		if err != nil {
			log.Println(err)
			stakingSuf, stakingWhole = []byte("{}"), []byte("{}")
		}
		refreshed = time.Now()
		return lastErr
	}

	// immediately upon startup, then every round
	err := update()
	if err != nil {
		log.Println(err)
	}

	go func() {
		for {
			time.Sleep(time.Minute)
			if time.Now().Add(-time.Hour).After(refreshed) {
				log.Fatalf("Last refresh was at %v, more than one hour ago. Giving up.", refreshed)
			}
		}
	}()

	tick := time.NewTicker(126 * time.Second)
	var queued bool
	p := message.NewPrinter(language.AmericanEnglish)
	for {
		select {
		case <-tick.C:
			if queued {
				log.Println("warning: update() has been running for more than 126 seconds.")
				continue
			}
			queued = true
			log.Println("updating stats")
			err = update()
			if err != nil {
				log.Println(err)
			}
			log.Print(p.Sprintf("rewards %f\n", bpRewards))
			log.Print(p.Sprintf("bucket %f\n", bpBucketPool))
			log.Print(p.Sprintf("minted %f\n", mintedTokens))
			log.Print(p.Sprintf("circulating %f\n", mintedTokens-bpRewards-bpBucketPool-lockedTokens))
			queued = false
		case <-abort:
			return
		}
	}
}

type rewards struct {
	Rewards uint64 `json:"rewards"`
}

func getBpRewards(table string, api *fio.API) (pool float64, err error) {
	result := make(chan uint64, 1)
	errc := make(chan error, 1)
	go func(r chan uint64, e chan error) {
		gtr := &eos.GetTableRowsResp{}
		gtr, err = api.GetTableRows(eos.GetTableRowsRequest{
			Code:  "fio.treasury",
			Scope: "fio.treasury",
			Table: table,
			JSON:  true,
		})
		if err != nil {
			e <- err
			return
		}
		rew := make([]rewards, 0)
		err = json.Unmarshal(gtr.Rows, &rew)
		if err != nil {
			e <- err
			return
		}
		if len(rew) == 0 {
			e <- errors.New("no result for get table rows")
		}
		r <- rew[0].Rewards
	}(result, errc)

	select {
	case <-time.After(2 * time.Second):
		return 0, errors.New("timeout waiting for getBpBucketPool")
	case e := <-errc:
		return 0, e
	case r := <-result:
		return float64(r) / 1_000_000_000.0, nil
	}
}

type Apr struct {
	OneDay    interface{} `json:"1day"`
	SevenDay  interface{} `json:"7day"`
	ThirtyDay interface{} `json:"30day"`
}

type StakingRewards struct {
	StakedTokenPool              float64 `json:"staked_token_pool"`
	OutstandingSrps              float64 `json:"outstanding_srps"`
	RewardsTokenPool             float64 `json:"rewards_token_pool"`
	CombinedTokenPool            float64 `json:"combined_token_pool"`
	StakingRewardsReservesMinted float64 `json:"staking_rewards_reserves_minted"`
	Roe                          float64 `json:"roe"`
	Active                       bool    `json:"active"`
	HistoricalApr                *Apr    `json:"historical_apr"`
}

type StakingRewardsSuf struct {
	StakedTokenPool              uint64  `json:"staked_token_pool"`                  // provided by nodeos
	OutstandingSrps              uint64  `json:"outstanding_srps"`                   // copied from global_srp_count
	GlobalSrpCount               uint64  `json:"global_srp_count,omitempty"`         // provided by nodeos, then omitted
	RewardsTokenPool             uint64  `json:"rewards_token_pool"`                 // provided by nodeos
	CombinedTokenPool            uint64  `json:"combined_token_pool"`                // provided by nodeos
	LastCombinedTokenPool        uint64  `json:"last_combined_token_pool,omitempty"` // provided by nodeos, them ommited
	StakingRewardsReservesMinted uint64  `json:"staking_rewards_reserves_minted"`    // provided by nodeos
	Roe                          float64 `json:"roe"`                                // calculated
	Active                       bool    `json:"active"`                             // calculated
	HistoricalApr                *Apr    `json:"historical_apr"`                     // calculated
}

func UpdateApr() (asWhole, asSuf []byte, err error) {
	api, _, err := fio.NewConnection(nil, url)
	if err != nil {
		return
	}
	gtr, err := api.GetTableRowsOrder(fio.GetTableRowsOrderRequest{
		Code:    "fio.staking",
		Scope:   "fio.staking",
		Table:   "staking",
		Limit:   25,
		KeyType: "i64",
		Index:   "1",
		JSON:    true,
		Reverse: false,
	})
	if err != nil {
		return
	}
	asWhole = make([]byte, 0)
	asSuf = make([]byte, 0)
	sufResults := make([]*StakingRewardsSuf, 0)
	err = json.Unmarshal(gtr.Rows, &sufResults)
	if err != nil {
		return
	}
	if len(sufResults) == 0 {
		err = errors.New("no staking results found")
		return
	}
	sufResult := sufResults[0]

	/*
		ROE = = [ Tokens in Combined Token Pool / Global SRPs ] FIO
	*/
	// using a string to avoid overflow on cast to float:
	combinedTokenPool, ok := new(big.Float).SetString(fmt.Sprint(sufResult.CombinedTokenPool))
	if !ok {
		return nil, nil, fmt.Errorf("could not convert %d to big float for combined token pool", sufResult.CombinedTokenPool)
	}
	sufResult.LastCombinedTokenPool = 0 // omit from json
	globalSrps, ok := new(big.Float).SetString(fmt.Sprint(sufResult.GlobalSrpCount))
	if !ok {
		return nil, nil, fmt.Errorf("could not convert %s to big float for global srp count", sufResult.GlobalSrpCount)
	}
	sufResult.OutstandingSrps = sufResult.GlobalSrpCount
	sufResult.GlobalSrpCount = 0 // omit from json
	todayRoe := new(big.Float).Quo(combinedTokenPool, globalSrps)
	sufResult.Roe, _ = todayRoe.Float64()

	/*
		([ROE on DayX] / [ROE on DayY] - 1) * (365 / Dur) * 100
	*/

	sufResult.HistoricalApr = &Apr{}
	minus1 := time.Now().UTC().Add(-24 * time.Hour)
	r1, err := fetchHistoricRoe(minus1.Format("20060102"))
	if err != nil {
		log.Println("1 day ROE lookup:", err)
	} else {
		d1 := new(big.Float).Quo(todayRoe, r1)
		d, _ := new(big.Float).Sub(d1, big.NewFloat(1)).Float64()
		sufResult.HistoricalApr.OneDay = d * 365 * 100
	}

	minus7 := time.Now().UTC().Add(-7 * 24 * time.Hour)
	r7, err := fetchHistoricRoe(minus7.Format("20060102"))
	if err != nil {
		log.Println("7 day ROE lookup:", err)
	} else {
		d7 := new(big.Float).Quo(todayRoe, r7)
		d, _ := new(big.Float).Sub(d7, big.NewFloat(1)).Float64()
		sufResult.HistoricalApr.SevenDay = d * (365 / 7) * 100
	}

	minus30 := time.Now().UTC().Add(-30 * 24 * time.Hour)
	r30, err := fetchHistoricRoe(minus30.Format("20060102"))
	if err != nil {
		log.Println("30 day ROE lookup:", err)
	} else {
		d30 := new(big.Float).Quo(todayRoe, r30)
		d, _ := new(big.Float).Sub(d30, big.NewFloat(1)).Float64()
		sufResult.HistoricalApr.ThirtyDay = d * (365 / 30) * 100
	}

	// is staking activated yet?
	gi, err := api.GetInfo()
	if err != nil {
		return
	}
	switch gi.ChainID.String() {
	case fio.ChainIdMainnet:
		activatesAt, e := time.Parse(time.RFC3339, "2022-02-22T00:00:00Z")
		if e != nil {
			log.Println(e)
		}
		if sufResult.CombinedTokenPool > 1_000_000_000_000_000 && time.Now().UTC().After(activatesAt) {
			sufResult.Active = true
		}
	default:
		if sufResult.CombinedTokenPool > 1_000_000_000_000_000 {
			sufResult.Active = true
		}
	}

	// update whole fio struct:

	wholeResult := &StakingRewards{
		Roe:           sufResult.Roe,
		Active:        sufResult.Active,
		HistoricalApr: sufResult.HistoricalApr, // pointer
	}
	wholeResult.StakedTokenPool, _ = new(big.Float).Quo(
		new(big.Float).SetUint64(sufResult.StakedTokenPool),
		big.NewFloat(1_000_000_000.0),
	).Float64()
	wholeResult.OutstandingSrps, _ = new(big.Float).Quo(
		new(big.Float).SetUint64(sufResult.OutstandingSrps),
		big.NewFloat(1_000_000_000.0),
	).Float64()
	wholeResult.RewardsTokenPool, _ = new(big.Float).Quo(
		new(big.Float).SetUint64(sufResult.RewardsTokenPool),
		big.NewFloat(1_000_000_000.0),
	).Float64()
	wholeResult.CombinedTokenPool, _ = new(big.Float).Quo(
		new(big.Float).SetUint64(sufResult.CombinedTokenPool),
		big.NewFloat(1_000_000_000.0),
	).Float64()
	wholeResult.StakingRewardsReservesMinted, _ = new(big.Float).Quo(
		new(big.Float).SetUint64(sufResult.StakingRewardsReservesMinted),
		big.NewFloat(1_000_000_000.0),
	).Float64()

	suf, err := json.Marshal(sufResult)
	if err != nil {
		return
	}
	whole, err := json.Marshal(wholeResult)
	if err != nil {
		return
	}
	err = persistStake(time.Now().UTC().Format("20060102"), suf)
	if err != nil {
		log.Println(err)
	}

	return whole, suf, nil
}

func persistStake(key string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rdb := &redis.Client{}
	if redisTls {
		rdb = redis.NewClient(&redis.Options{
			Addr:     redisUrl,
			Password: redisPass,
			DB:       redisDb,
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		})
	} else {
		rdb = redis.NewClient(&redis.Options{
			Addr:     redisUrl,
			Password: redisPass,
			DB:       redisDb,
		})
	}
	err := rdb.Ping(ctx).Err()
	if err != nil {
		return err
	}
	err = rdb.Set(ctx, key, data, 0).Err()
	if err != nil {
		return err
	}
	return nil
}

func fetchHistoricRoe(key string) (*big.Float, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rdb := &redis.Client{}
	if redisTls {
		rdb = redis.NewClient(&redis.Options{
			Addr:     redisUrl,
			Password: redisPass,
			DB:       redisDb,
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		})
	} else {
		rdb = redis.NewClient(&redis.Options{
			Addr:     redisUrl,
			Password: redisPass,
			DB:       redisDb,
		})
	}
	s, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			log.Println("no historic ROE information found for", key)
		}
		return big.NewFloat(0), err
	}
	result := &StakingRewardsSuf{}
	err = json.Unmarshal([]byte(s), result)
	if err != nil {
		return big.NewFloat(0), err
	}

	return big.NewFloat(result.Roe), nil
}
