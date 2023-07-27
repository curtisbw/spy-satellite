package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/portfoliotree/alphavantage"
	"github.com/portfoliotree/portfolio/calculations"
	"github.com/portfoliotree/portfolio/returns"
)

func main() {
	spyHoldingsBuf, err := os.ReadFile("spy_holdings.csv")
	if err != nil {
		log.Fatal(err)
	}

	var holdings []Holding
	if err := alphavantage.ParseCSV(bytes.NewReader(spyHoldingsBuf), &holdings, nil); err != nil {
		log.Fatal(err)
	}

	var (
		table      returns.Table
		mostRecent time.Time
		newest     Holding
	)
	for _, holding := range holdings {
		rs, err := holding.Returns()
		if err != nil {
			log.Fatal(err)
		}
		if mostRecent.IsZero() || rs.FirstTime().After(mostRecent) {
			mostRecent = rs.FirstTime()
			newest = holding
		}
		table = table.AddColumn(rs)
	}
	fmt.Println("most recent", mostRecent.Format(time.DateOnly), newest.Ticker)
	fmt.Println(table.LastTime().Format(time.DateOnly), table.FirstTime().Format(time.DateOnly))
}

func returnsFromQuotes(quotes []alphavantage.Quote) returns.List {
	rs := calculations.HoldingPeriodReturns(splitAndDividendAdjustedQuotes(quotes))
	list := make(returns.List, len(rs))
	for i, quote := range quotes[:len(rs)] {
		list[i].Time = truncateDate(quote.Time)
		list[i].Value = rs[i]
	}
	return list
}

func truncateDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func splitAndDividendAdjustedQuotes(list []alphavantage.Quote) []float64 {
	result := make([]float64, len(list))

	splitCoefficient := 1.0
	totalDividends := 0.0

	for i := len(result) - 1; i >= 0; i-- {
		q := list[i]

		if q.SplitCoefficient != 0 {
			splitCoefficient *= q.SplitCoefficient
		}

		totalDividends += q.DividendAmount

		result[i] = q.Close*splitCoefficient + totalDividends
	}

	return result
}

type Holding struct {
	Name          string  `column-name:"Name"`
	Ticker        string  `column-name:"Ticker"`
	Identifier    string  `column-name:"Identifier"`
	SEDOL         string  `column-name:"SEDOL"`
	Weight        float64 `column-name:"Weight"`
	SharesHeld    float64 `column-name:"Shares Held"`
	LocalCurrency string  `column-name:"Local Currency"`
}

func (h Holding) Returns() (returns.List, error) {
	path := filepath.Join("data", h.Ticker+".csv")
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	quotes, err := alphavantage.ParseQuotes(bytes.NewBuffer(buf), nil)
	if err != nil {
		return nil, err
	}
	return returnsFromQuotes(quotes), nil
}
