// Package quotation holds the quoting rules and the HTTP contract of the quotation-api.
package quotation

import (
	"fmt"
	"strings"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/partner"
)

// Request is the body of POST /quotes: the risk to be quoted. The broker does NOT show up here — it
// comes in the X-Tenant-Id header, because the tenant is call context, not insurance data.
type Request struct {
	Driver   Driver  `json:"driver"`
	Vehicle  Vehicle `json:"vehicle"`
	Coverage string  `json:"coverage"`
}

type Driver struct {
	Document  string `json:"document"`
	BirthYear int    `json:"birth_year"`
}

type Vehicle struct {
	Plate      string `json:"plate"`
	Model      string `json:"model"`
	Year       int    `json:"year"`
	ValueCents int64  `json:"value_cents"`
}

// accepted coverages. Two are enough: the starter is a working skeleton, not a product.
var coverages = map[string]bool{"comprehensive": true, "third_party": true}

const defaultCoverage = "comprehensive"

// Normalize fills in whatever has a default and validates the rest. The validation is deliberately
// short — the challenge is not about insurance business rules, it is about what happens when a
// partner fails.
func (r *Request) Normalize() error {
	r.Coverage = strings.TrimSpace(r.Coverage)
	if r.Coverage == "" {
		r.Coverage = defaultCoverage
	}

	r.Driver.Document = strings.TrimSpace(r.Driver.Document)
	r.Vehicle.Plate = strings.ToUpper(strings.TrimSpace(r.Vehicle.Plate))
	r.Vehicle.Model = strings.TrimSpace(r.Vehicle.Model)

	switch {
	case r.Driver.Document == "":
		return fmt.Errorf("driver.document is required")
	case r.Driver.BirthYear < 1900:
		return fmt.Errorf("driver.birth_year is required and must be a valid year")
	case r.Vehicle.Plate == "":
		return fmt.Errorf("vehicle.plate is required")
	case r.Vehicle.Year < 1900:
		return fmt.Errorf("vehicle.year is required and must be a valid year")
	case r.Vehicle.ValueCents <= 0:
		return fmt.Errorf("vehicle.value_cents must be greater than zero")
	case !coverages[r.Coverage]:
		return fmt.Errorf("coverage must be comprehensive or third_party")
	}
	return nil
}

// partnerRequest is what the partner receives. The broker goes in the body because, in the challenge
// scenario, each broker has its own commercial terms: the same plate quoted by two brokers is worth
// different premiums. That is what makes per-tenant isolation a requirement rather than mere care —
// including in the cache key the student is going to build.
type partnerRequest struct {
	Broker string `json:"broker"`
	Request
}

// Response is the success body of POST /quotes.
type Response struct {
	TenantID  string          `json:"tenant_id"`
	Quotes    []partner.Quote `json:"quotes"`
	ElapsedMs int64           `json:"elapsed_ms"`
}
