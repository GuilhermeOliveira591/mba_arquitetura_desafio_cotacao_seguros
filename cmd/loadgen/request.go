package main

import (
	"encoding/json"
	"fmt"

	"github.com/GuilhermeOliveira591/mba_arquitetura_desafio_cotacao_seguros/internal/quotation"
)

// quoteBodies builds the quotes loadgen rotates through during a run.
//
// It reuses quotation.Request instead of hand-writing JSON so that the generator cannot drift away
// from the contract it exercises: change a field in the API and this stops compiling, which is a
// better warning than a run where every request comes back 400.
//
// How many distinct quotes there are is a knob and not a detail. Today they only serve to keep the
// load from being a single repeated request; once the student has built the cache, this is the dial
// that controls the hit rate — `-distinct 1` is the best possible case (everything is a hit after
// the first one), a high value is the worst.
func quoteBodies(distinct int) ([][]byte, error) {
	models := []string{"Onix 1.0", "HB20 1.6", "Gol 1.0", "Argo 1.3", "Kwid 1.0"}

	bodies := make([][]byte, 0, distinct)
	for i := 0; i < distinct; i++ {
		body, err := json.Marshal(quotation.Request{
			Driver: quotation.Driver{
				Document:  fmt.Sprintf("%011d", 12345678900+i),
				BirthYear: 1970 + i%30,
			},
			Vehicle: quotation.Vehicle{
				Plate:      fmt.Sprintf("LDG%04d", i),
				Model:      models[i%len(models)],
				Year:       2015 + i%10,
				ValueCents: 4500000 + int64(i%20)*250000,
			},
			Coverage: "comprehensive",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to build the quote body: %w", err)
		}
		bodies = append(bodies, body)
	}
	return bodies, nil
}
