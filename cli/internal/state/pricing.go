package state

import "strings"

// ModelPrice is the public list price of a Claude model in USD per million
// tokens, split by input and output. It is the basis for the agent-routing
// economics that feed the Token Savings Meter: the binary derives cost/saved
// from these prices so callers only supply the model and the token counts.
//
// Keep in sync with the Claude pricing page. Prices below are list prices
// (USD / 1M tokens) as of the current model lineup:
//
//	haiku  (Haiku 4.5)  $1 / $5
//	sonnet (Sonnet 4.6) $3 / $15
//	opus   (Opus 4.8)   $5 / $25
//	fable  (Fable 5)    $10 / $50
type ModelPrice struct {
	InputPerMTok  float64 `json:"inputPerMTok"`
	OutputPerMTok float64 `json:"outputPerMTok"`
}

// modelPrices is keyed by the normalized model family (see normalizeModelKey),
// so both short aliases ("haiku") and full ids ("claude-haiku-4-5") resolve.
var modelPrices = map[string]ModelPrice{
	"haiku":  {InputPerMTok: 1.00, OutputPerMTok: 5.00},
	"sonnet": {InputPerMTok: 3.00, OutputPerMTok: 15.00},
	"opus":   {InputPerMTok: 5.00, OutputPerMTok: 25.00},
	"fable":  {InputPerMTok: 10.00, OutputPerMTok: 50.00},
}

// LookupModelPrice resolves a model name — a short alias ("haiku") or a full id
// ("claude-haiku-4-5") — to its list price. ok is false for an unknown model.
func LookupModelPrice(model string) (price ModelPrice, ok bool) {
	price, ok = modelPrices[normalizeModelKey(model)]
	return price, ok
}

// normalizeModelKey maps any Claude model spelling to its pricing family.
func normalizeModelKey(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(m, "haiku"):
		return "haiku"
	case strings.Contains(m, "sonnet"):
		return "sonnet"
	case strings.Contains(m, "opus"):
		return "opus"
	case strings.Contains(m, "fable"), strings.Contains(m, "mythos"):
		return "fable"
	default:
		return m
	}
}

// CostUSD is the dollar cost of a request at this model's list price.
func (p ModelPrice) CostUSD(tokensIn, tokensOut int) float64 {
	return (float64(tokensIn)*p.InputPerMTok + float64(tokensOut)*p.OutputPerMTok) / 1_000_000
}
