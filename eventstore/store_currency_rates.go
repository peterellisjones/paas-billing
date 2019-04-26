package eventstore

import (
	"encoding/json"

	"github.com/alphagov/paas-billing/eventio"
)

var _ eventio.CurrencyRateReader = &EventStore{}

func (s *EventStore) GetCurrencyRates(filter eventio.TimeRangeFilter) ([]eventio.CurrencyRate, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryJSON(tx, `
        with
        valid_currency_rates as (
            select
                *,
                tstzrange(valid_from, lead(valid_from, 1, 'infinity') over (
                    partition by code order by valid_from rows between current row and 1 following
                )) as valid_for
            from
                currency_rates
        )
        select
            vcr.code,
            vcr.valid_from,
            vcr.rate
        from
            valid_currency_rates vcr
        where
            vcr.valid_for && tstzrange($1, $2)
        group by
            vcr.code,
            vcr.valid_from,
            vcr.rate
        order by
            valid_from
    `, filter.RangeStart, filter.RangeStop)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	currencyRates := []eventio.CurrencyRate{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var currencyRate eventio.CurrencyRate
		if err := json.Unmarshal(b, &currencyRate); err != nil {
			return nil, err
		}
		currencyRates = append(currencyRates, currencyRate)

	}
	return currencyRates, nil
}
