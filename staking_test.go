package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"
)

func TestUpdateApr(t *testing.T) {
	data := make([]testRewards, 0)
	e := json.Unmarshal([]byte(historicalData), &data)
	if e != nil {
		t.Fatal(e)
	}
	for h := range data {
		sufResult := data[h].StakingRewardsSuf
		if sufResult == nil {
			t.Fatal("nil result for staking table lookup")
		}

		todayRoe, err := sufResult.updateROE()
		if err != nil {
			t.Error(err)
			return
		}

		sufResult.HistoricalApr = &Apr{}
		// this replaces the redis lookup with our test data:
		prevRoe := sufResult.simHistorical(data[h].Date, 0, data, todayRoe)
		_ = sufResult.simHistorical(data[h].Date, 1, data, prevRoe)
		_ = sufResult.simHistorical(data[h].Date, 7, data, prevRoe)
		_ = sufResult.simHistorical(data[h].Date, 30, data, prevRoe)

		url = "http://dev:8887" // override URL for test server
		sufResult.Active, err = stakingActive(sufResult.CombinedTokenPool)
		if err != nil {
			return
		}
		wholeResult := sufResult.toWhole()

		// only print out the whole number version, if it's wrong both are.
		whole, err := json.MarshalIndent(wholeResult, "", "  ")
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Println("----------------------------------------------------")
		fmt.Print(data[h].Date, "\n\n")
		fmt.Println(string(whole))
		fmt.Print("\n\n")
		//whole, _ = json.Marshal(sufResult)
		//fmt.Printf("SET %d %q\n\n", data[h].Date, string(whole))
	}
}

type testRewards struct {
	Date int `json:"date"`
	StakingRewardsSuf *StakingRewardsSuf `json:"staking_rewards_suf"`
}

func (sufResult *StakingRewardsSuf) simHistorical(day, days int, data []testRewards, todayRoe *big.Float) *big.Float {
	if sufResult.HistoricalApr == nil {
		sufResult.HistoricalApr = &Apr{}
	}
	prevROE := new(big.Float)
	for j := range data {
		if data[j].Date == day - (days + 1) {
			sufResult.GlobalSrpCount = sufResult.OutstandingSrps

			var err error
			prevROE, err = data[j].StakingRewardsSuf.updateROE()
			if err != nil {
				return prevROE
			}
			//fmt.Println(prevROE.Float64())
			break
		}
	}
	notZero, _ := prevROE.Float64()
	if notZero > 0 {
		dd := new(big.Float).Quo(todayRoe, prevROE)
		d, _ := new(big.Float).Sub(dd, big.NewFloat(1)).Float64()
		apr := d * (365 / float64(days)) * 100
		//if apr < 0 {
		//	apr = 0
		//}
		switch days {
		case 1:
			sufResult.HistoricalApr.OneDay = apr
		case 7:
			sufResult.HistoricalApr.SevenDay = apr
		case 30:
			sufResult.HistoricalApr.ThirtyDay = apr
		}
	}
	return prevROE
}


const historicalData = `[
  {
    "date": 20220104,
    "staking_rewards_suf": {
      "last_global_srp_count": 6338500090562484,
      "staked_token_pool": 3169250045281242,
      "staking_rewards_suf_reserves_minted": 0,
      "global_srp_count": 6338500090562484,
      "last_combined_token_pool": 3172092407974025,
      "daily_staking_rewards_suf": 2842362692783,
      "rewards_token_pool": 2842362692783,
      "combined_token_pool": 3172092407974025
    }
  },
  {
    "date": 20220105,
    "staking_rewards_suf": {
      "last_global_srp_count": 6338500090562484,
      "staked_token_pool": 3168250045281242,
      "staking_rewards_suf_reserves_minted": 19905300937221,
      "global_srp_count": 6336500090562484,
      "last_combined_token_pool": 3172092407974025,
      "daily_staking_rewards_suf": 3320759687052,
      "rewards_token_pool": 8415458749831,
      "combined_token_pool": 3196570804968294
    }
  },
  {
    "date": 20220106,
    "staking_rewards_suf": {
      "last_global_srp_count": 6336500090562484,
      "staked_token_pool": 3168250045281242,
      "staking_rewards_suf_reserves_minted": 39775726328105,
      "global_srp_count": 6336500090562484,
      "last_combined_token_pool": 3196570804968294,
      "daily_staking_rewards_suf": 2526772886530,
      "rewards_token_pool": 12751046558425,
      "combined_token_pool": 3220776818167772
    }
  },
  {
    "date": 20220107,
    "staking_rewards_suf": {
      "last_global_srp_count": 6336500090562484,
      "staked_token_pool": 3050030045281242,
      "staking_rewards_suf_reserves_minted": 60654580156819,
      "global_srp_count": 6100060090562484,
      "last_combined_token_pool": 3220776818167772,
      "daily_staking_rewards_suf": 2662333203980,
      "rewards_token_pool": 17007753047161,
      "combined_token_pool": 3127692378485222
    }
  },
  {
    "date": 20220108,
    "staking_rewards_suf": {
      "last_global_srp_count": 6100060090562484,
      "staked_token_pool": 3050030045281242,
      "staking_rewards_suf_reserves_minted": 81441614168511,
      "global_srp_count": 6100060090562484,
      "last_combined_token_pool": 3127692378485222,
      "daily_staking_rewards_suf": 2695959010912,
      "rewards_token_pool": 21254344842401,
      "combined_token_pool": 3152726004292154
    }
  },
  {
    "date": 20220109,
    "staking_rewards_suf": {
      "last_global_srp_count": 6100060090562484,
      "staked_token_pool": 3050030045281242,
      "staking_rewards_suf_reserves_minted": 101902370822250,
      "global_srp_count": 6100060090562484,
      "last_combined_token_pool": 3152726004292154,
      "daily_staking_rewards_suf": 2616932952754,
      "rewards_token_pool": 25714562130504,
      "combined_token_pool": 3177646978233996
    }
  },
  {
    "date": 20220110,
    "staking_rewards_suf": {
      "last_global_srp_count": 6100060090562484,
      "staked_token_pool": 3050120045281242,
      "staking_rewards_suf_reserves_minted": 122723052274721,
      "global_srp_count": 6100240090562484,
      "last_combined_token_pool": 3177646978233996,
      "daily_staking_rewards_suf": 2866893517428,
      "rewards_token_pool": 30143841242707,
      "combined_token_pool": 3202986938798670
    }
  },
  {
    "date": 20220111,
    "staking_rewards_suf": {
      "last_global_srp_count": 6100240090562484,
      "staked_token_pool": 3050120045281242,
      "staking_rewards_suf_reserves_minted": 143210953666468,
      "global_srp_count": 6100240090562484,
      "last_combined_token_pool": 3202986938798670,
      "daily_staking_rewards_suf": 2580657350881,
      "rewards_token_pool": 34369703684413,
      "combined_token_pool": 3227700702632123
    }
  },
  {
    "date": 20220112,
    "staking_rewards_suf": {
      "last_global_srp_count": 6100240090562484,
      "staked_token_pool": 3050120045281242,
      "staking_rewards_suf_reserves_minted": 164021042609873,
      "global_srp_count": 6100240090562484,
      "last_combined_token_pool": 3227700702632123,
      "daily_staking_rewards_suf": 3047211617609,
      "rewards_token_pool": 39026169007736,
      "combined_token_pool": 3253167256898851
    }
  },
  {
    "date": 20220113,
    "staking_rewards_suf": {
      "last_global_srp_count": 6100240090562484,
      "staked_token_pool": 3050120045281242,
      "staking_rewards_suf_reserves_minted": 180832322882756,
      "global_srp_count": 6100240090562484,
      "last_combined_token_pool": 3253167256898851,
      "daily_staking_rewards_suf": 8949290270105,
      "rewards_token_pool": 53116967387349,
      "combined_token_pool": 3284069335551347
    }
  },
  {
    "date": 20220114,
    "staking_rewards_suf": {
      "last_global_srp_count": 6100240090562484,
      "staked_token_pool": 3050120045281242,
      "staking_rewards_suf_reserves_minted": 190624784524435,
      "global_srp_count": 6100240090562484,
      "last_combined_token_pool": 3284069335551347,
      "daily_staking_rewards_suf": 5611747154680,
      "rewards_token_pool": 64986962630245,
      "combined_token_pool": 3305731792435922
    }
  },
  {
    "date": 20220115,
    "staking_rewards_suf": {
      "last_global_srp_count": 6100240090562484,
      "staked_token_pool": 3050120045281242,
      "staking_rewards_suf_reserves_minted": 205493268806395,
      "global_srp_count": 6100240090562484,
      "last_combined_token_pool": 3305731792435922,
      "daily_staking_rewards_suf": 4790919156893,
      "rewards_token_pool": 74297650350498,
      "combined_token_pool": 3329910964438135
    }
  },
  {
    "date": 20220116,
    "staking_rewards_suf": {
      "last_global_srp_count": 6100240090562484,
      "staked_token_pool": 3050710045281242,
      "staking_rewards_suf_reserves_minted": 222230711670847,
      "global_srp_count": 6101420090562484,
      "last_combined_token_pool": 3329910964438135,
      "daily_staking_rewards_suf": 3490179215712,
      "rewards_token_pool": 81259467544865,
      "combined_token_pool": 3354200224496954
    }
  },
  {
    "date": 20220117,
    "staking_rewards_suf": {
      "last_global_srp_count": 6101420090562484,
      "staked_token_pool": 3359627545281242,
      "staking_rewards_suf_reserves_minted": 241444850219335,
      "global_srp_count": 6719255090562484,
      "last_combined_token_pool": 3354200224496954,
      "daily_staking_rewards_suf": 3147255928007,
      "rewards_token_pool": 86702405708672,
      "combined_token_pool": 3687774801209249
    }
  },
  {
    "date": 20220118,
    "staking_rewards_suf": {
      "last_global_srp_count": 6719255090562484,
      "staked_token_pool": 7972293061281045,
      "staking_rewards_suf_reserves_minted": 260908725809601,
      "global_srp_count": 15944586122562090,
      "last_combined_token_pool": 3687774801209249,
      "daily_staking_rewards_suf": 3265611272256,
      "rewards_token_pool": 92356885462655,
      "combined_token_pool": 8325558672553301
    }
  },
  {
    "date": 20220119,
    "staking_rewards_suf": {
      "last_global_srp_count": 15944586122562090,
      "staked_token_pool": 8051557419381045,
      "staking_rewards_suf_reserves_minted": 281631019820685,
      "global_srp_count": 16103114838762090,
      "last_combined_token_pool": 8325558672553301,
      "daily_staking_rewards_suf": 3879525988609,
      "rewards_token_pool": 97248506167924,
      "combined_token_pool": 8430436945369654
    }
  },
  {
    "date": 20220120,
    "staking_rewards_suf": {
      "last_global_srp_count": 16103114838762090,
      "staked_token_pool": 8164966340801357,
      "staking_rewards_suf_reserves_minted": 300258715074084,
      "global_srp_count": 16329932681602714,
      "last_combined_token_pool": 8430436945369654,
      "daily_staking_rewards_suf": 3808766907059,
      "rewards_token_pool": 103550051832975,
      "combined_token_pool": 8568775107708416
    }
  },
  {
    "date": 20220121,
    "staking_rewards_suf": {
      "last_global_srp_count": 16329932681602714,
      "staked_token_pool": 9109208327801356,
      "staking_rewards_suf_reserves_minted": 319291803238190,
      "global_srp_count": 18218416655602710,
      "last_combined_token_pool": 8568775107708416,
      "daily_staking_rewards_suf": 4099434003296,
      "rewards_token_pool": 109807630765106,
      "combined_token_pool": 9538307761804652
    }
  },
  {
    "date": 20220122,
    "staking_rewards_suf": {
      "last_global_srp_count": 18218416655602710,
      "staked_token_pool": 9112592621801356,
      "staking_rewards_suf_reserves_minted": 337985209166070,
      "global_srp_count": 18225185243602710,
      "last_combined_token_pool": 9538307761804652,
      "daily_staking_rewards_suf": 4427807888876,
      "rewards_token_pool": 116442598722806,
      "combined_token_pool": 9567020429690232
    }
  },
  {
    "date": 20220123,
    "staking_rewards_suf": {
      "last_global_srp_count": 18225185243602710,
      "staked_token_pool": 9119702474801356,
      "staking_rewards_suf_reserves_minted": 355843144544616,
      "global_srp_count": 18239404949602710,
      "last_combined_token_pool": 9567020429690232,
      "daily_staking_rewards_suf": 3565654703640,
      "rewards_token_pool": 122722510159024,
      "combined_token_pool": 9598268129504996
    }
  },
  {
    "date": 20220124,
    "staking_rewards_suf": {
      "last_global_srp_count": 18239404949602710,
      "staked_token_pool": 9186066419265648,
      "staking_rewards_suf_reserves_minted": 374889367806728,
      "global_srp_count": 18372132838531296,
      "last_combined_token_pool": 9598268129504996,
      "daily_staking_rewards_suf": 3572592218348,
      "rewards_token_pool": 128683224411620,
      "combined_token_pool": 9689639011483996
    }
  },
  {
    "date": 20220125,
    "staking_rewards_suf": {
      "last_global_srp_count": 18372132838531296,
      "staked_token_pool": 9221499503188296,
      "staking_rewards_suf_reserves_minted": 393911259224451,
      "global_srp_count": 18442999006376590,
      "last_combined_token_pool": 9689639011483996,
      "daily_staking_rewards_suf": 4055114027514,
      "rewards_token_pool": 135143854803063,
      "combined_token_pool": 9750554617215808
    }
  },
  {
    "date": 20220126,
    "staking_rewards_suf": {
      "last_global_srp_count": 18442999006376590,
      "staked_token_pool": 9223071466188296,
      "staking_rewards_suf_reserves_minted": 412501316288460,
      "global_srp_count": 18446142932376590,
      "last_combined_token_pool": 9750554617215808,
      "daily_staking_rewards_suf": 3979088961525,
      "rewards_token_pool": 141477772673065,
      "combined_token_pool": 9777050555149820
    }
  },
  {
    "date": 20220127,
    "staking_rewards_suf": {
      "last_global_srp_count": 18446142932376590,
      "staked_token_pool": 9257402395358804,
      "staking_rewards_suf_reserves_minted": 431501727260942,
      "global_srp_count": 18514804790717610,
      "last_combined_token_pool": 9777050555149820,
      "daily_staking_rewards_suf": 3429683916078,
      "rewards_token_pool": 146927956655136,
      "combined_token_pool": 9835832079274882
    }
  },
  {
    "date": 20220128,
    "staking_rewards_suf": {
      "last_global_srp_count": 18514804790717610,
      "staked_token_pool": 9276907195358814,
      "staking_rewards_suf_reserves_minted": 450909295323585,
      "global_srp_count": 18553814390717628,
      "last_combined_token_pool": 9835832079274882,
      "daily_staking_rewards_suf": 3402364672711,
      "rewards_token_pool": 152493069349126,
      "combined_token_pool": 9880309560031524
    }
  },
  {
    "date": 20220129,
    "staking_rewards_suf": {
      "last_global_srp_count": 18553814390717628,
      "staked_token_pool": 9410136633486892,
      "staking_rewards_suf_reserves_minted": 470383863254439,
      "global_srp_count": 18820273266973784,
      "last_combined_token_pool": 9880309560031524,
      "daily_staking_rewards_suf": 3360922515469,
      "rewards_token_pool": 157977059261030,
      "combined_token_pool": 10038497556002360
    }
  },
  {
    "date": 20220130,
    "staking_rewards_suf": {
      "last_global_srp_count": 18820273266973784,
      "staked_token_pool": 9343512733486892,
      "staking_rewards_suf_reserves_minted": 489892637526172,
      "global_srp_count": 18687025466973784,
      "last_combined_token_pool": 10038497556002360,
      "daily_staking_rewards_suf": 7204421240705,
      "rewards_token_pool": 167311783714533,
      "combined_token_pool": 10000717154727596
    }
  },
  {
    "date": 20220131,
    "staking_rewards_suf": {
      "last_global_srp_count": 18687025466973784,
      "staked_token_pool": 9792261601509292,
      "staking_rewards_reserves_minted": 502347705435116,
      "global_srp_count": 19584523203018584,
      "last_combined_token_pool": 10000717154727596,
      "daily_staking_rewards": 4085515242701,
      "rewards_token_pool": 176737809807585,
      "combined_token_pool": 10471347116751992
    }
  }
]`