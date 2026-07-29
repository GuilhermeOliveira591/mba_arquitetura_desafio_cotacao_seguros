package quotation

import (
	"context"
	"sort"
	"time"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/partner"
	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/platform"
)

// Quoter is everything the service needs to know about a partner.
type Quoter interface {
	Quote(ctx context.Context, p platform.Partner, request any) (partner.Quote, error)
}

// Service aggregates the quotes from the partners.
type Service struct {
	partners []platform.Partner
	quoter   Quoter
}

func NewService(partners []platform.Partner, quoter Quoter) *Service {
	return &Service{partners: partners, quoter: quoter}
}

// Quote calls the partners SERIALLY and returns the quotes sorted from cheapest to most expensive.
//
// Two naive choices here are deliberate and are the heart of the challenge:
//
//  1. Serial, not parallel. The response time is the SUM of the time of the three partners. With
//     partner-slow at 1.5s, no quote comes out below that — and under load partner-degrading piles
//     up on top of it.
//  2. All or nothing. A single failing partner brings down the whole request, even if the other two
//     answered. There is no fallback, there is no partial response, there is no cache to serve the
//     previous quote.
//
// This is exactly the pathology the student is going to measure before and after. Do not "fix" this
// in the starter: the gap is the assignment.
func (s *Service) Quote(ctx context.Context, tenant string, request Request) (Response, error) {
	start := time.Now()
	forPartner := partnerRequest{Broker: tenant, Request: request}

	quotes := make([]partner.Quote, 0, len(s.partners))
	for _, p := range s.partners {
		quote, err := s.quoter.Quote(ctx, p, forPartner)
		if err != nil {
			return Response{}, err
		}
		quotes = append(quotes, quote)
	}

	sort.Slice(quotes, func(i, j int) bool {
		return quotes[i].PremiumCents < quotes[j].PremiumCents
	})

	return Response{
		TenantID:  tenant,
		Quotes:    quotes,
		ElapsedMs: time.Since(start).Milliseconds(),
	}, nil
}
