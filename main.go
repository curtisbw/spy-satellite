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
	err = alphavantage.ParseCSV(bytes.NewReader(spyHoldingsBuf), &holdings, nil)
	if err != nil {
		log.Fatal(err)
	}

	var (
		table   returns.Table
		weights []float64
	)
	for _, holding := range holdings {
		weights = append(weights, holding.Weight/100)
		rs, err := holding.Returns()
		if err != nil {
			log.Fatal(err)
		}
		rs = rs.Between(time.Now(), time.Now().AddDate(-1, 0, 0))
		table = table.AddColumn(rs)
	}
	risks := table.RisksFromStdDev()

	weightedAverageRisk := calculations.WeightedAverageRisk(weights, risks)
	annualizedWeightedAverageRisk := calculations.AnnualizeRisk(weightedAverageRisk, calculations.PeriodsPerYear)
	fmt.Printf("annualized weighted average risk: %f\n", annualizedWeightedAverageRisk)

	portfolioRisk := calculations.AnnualizeRisk(calculations.ExpectedRisk(risks, weights, table.CorrelationMatrix()), calculations.PeriodsPerYear)
	fmt.Printf("portfolio risk: %f\n", portfolioRisk)

	bets, err := calculations.NumberOfBets(annualizedWeightedAverageRisk, portfolioRisk)
	if err != nil {
		log.Println(err)
	}

	fmt.Printf("unique number of bets from %s to %s\n", table.FirstTime().Format(time.DateOnly), table.LastTime().Format(time.DateOnly))
	fmt.Printf("bets: %f\n", bets)
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
