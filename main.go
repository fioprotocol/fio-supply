package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/fioprotocol/fio-go"
	"github.com/fioprotocol/fio-go/eos"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	circulatingTokens  float64
	mintedTokens  float64
	lockedTokens  float64
	bpRewards     float64
	bpBucketPool  float64
	url           string
	refreshed     time.Time
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	var port string
	flag.StringVar(&url, "u", "", "url to connect to")
	flag.StringVar(&port, "p", "8080", "port to listen on")
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
	if unsigned{
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
	w.Header().Set("access-control-allow-origin", "*")

	if mintedTokens == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(nil)
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
		w.Write(nil)
		return
	}
	if asJson {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.Header().Set("x-last-refreshed", refreshed.UTC().Format(time.RFC1123))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(formatter(asSuf, asJson, whole, desc, amount)))
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

		var rewards float64
		rewards, err = getBpRewards("bprewards", api)
		if err != nil {
			log.Println(err)
			lastErr = err
		} else if rewards > 0 {
			bpRewards = rewards
		}
		refreshed = time.Now()
		return lastErr
	}

	// immediately upon startup, then every round
	err := update()
	if err != nil {
		log.Println(err)
	}

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
			if err!= nil {
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
		return float64(r)/1_000_000_000.0, nil
	}
}
