// Package partner is the client of the partner insurers — the boundary between the quotation-api and
// the unstable external dependency of the challenge.
//
// This is where the circuit breaker will be born. Today, on purpose, there is no protection at all:
// no timeout, no retry, no breaker, no fallback. A call goes out, and whatever comes back (or does
// not come back) is what the API delivers. That emptiness is the exercise, not an oversight — see the
// note on Client.
package partner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/platform"
)

// Quote is the response of a partner insurer.
type Quote struct {
	Partner         string `json:"partner"`
	QuoteID         string `json:"quote_id"`
	PremiumCents    int64  `json:"premium_cents"`
	Currency        string `json:"currency"`
	CoverageCents   int64  `json:"coverage_cents"`
	ValidForSeconds int64  `json:"valid_for_seconds"`
}

// Client speaks HTTP with the partners.
//
// The http.Client is deliberately raw: a zero `Timeout` means waiting forever. A partner degrading to
// 6s holds the request goroutine for that entire time, and nothing here stops the next call from
// doing the same. Adding a timeout is the first thing the student will want to do — and it is exactly
// what the starter does not deliver ready-made.
type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{}}
}

// responseLimit cuts off absurd responses from a badly behaved partner.
const responseLimit = 1 << 20 // 1 MiB

// Quote asks a partner for a quote. The request is serialized by the client itself instead of being
// forwarded byte by byte from the original caller: a canonical body makes the same logical quote
// always produce the same request — which is what the partner answers consistently, and what later
// makes a cache key possible.
func (c *Client) Quote(ctx context.Context, p platform.Partner, request any) (Quote, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return Quote{}, &Error{Partner: p.Name, Reason: fmt.Sprintf("invalid request: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/quotes", bytes.NewReader(body))
	if err != nil {
		return Quote{}, &Error{Partner: p.Name, Reason: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")

	response, err := c.http.Do(req)
	if err != nil {
		return Quote{}, &Error{Partner: p.Name, Reason: err.Error()}
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, responseLimit))
		return Quote{}, &Error{
			Partner: p.Name,
			Status:  response.StatusCode,
			Reason:  fmt.Sprintf("partner replied %d", response.StatusCode),
		}
	}

	var quote Quote
	if err := json.NewDecoder(io.LimitReader(response.Body, responseLimit)).Decode(&quote); err != nil {
		return Quote{}, &Error{Partner: p.Name, Reason: fmt.Sprintf("unreadable response: %v", err)}
	}

	// The partner may omit its own name; the configuration is what rules.
	quote.Partner = p.Name
	return quote, nil
}

// Error identifies which partner failed and why. Without it the API could only say "something went
// wrong" — and the student needs to know whose fault it was to decide where the breaker goes.
type Error struct {
	Partner string
	Status  int
	Reason  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("partner %s: %s", e.Partner, e.Reason)
}
