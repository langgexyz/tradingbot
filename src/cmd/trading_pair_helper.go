package cmd

import (
	"strings"
	"tradingbot/src/cex"
)

// CreateTradingPair 创建交易对
func CreateTradingPair(base, quote string) cex.TradingPair {
	return cex.TradingPair{
		Base:  strings.ToUpper(base),
		Quote: strings.ToUpper(quote),
	}
}
