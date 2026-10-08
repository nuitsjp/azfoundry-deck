//go:build !e2e

package foundry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement/v3"
)

// costRetryAfter is the header in which Cost Management tells how long the
// subscription's query quota stays exhausted. Azure SDK reads only Retry-After.
const costRetryAfter = "x-ms-ratelimit-microsoft.costmanagement-entity-retry-after"

// A retry right at the announced time can still hit the quota, so a second is added
// and the throttled query is retried a few times.
const costRetries = 3

// Cost sums the subscription's month-to-date actual cost with Azure Cost Management.
// A subscription without usage returns no rows, which is a zero cost of unknown currency.
// When the query quota is exhausted, it waits the announced time and retries, up to costRetries times.
func (s *azureSource) Cost(ctx context.Context, foundry Foundry) (Cost, error) {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return Cost{}, err
	}
	// Immediate retries of 429 only consume the exhausted quota, so 429 is left to the wait below.
	client, err := armcostmanagement.NewQueryClient(s.credential, &arm.ClientOptions{ClientOptions: policy.ClientOptions{
		Retry: policy.RetryOptions{StatusCodes: []int{http.StatusRequestTimeout, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout}},
	}})
	if err != nil {
		return Cost{}, err
	}
	query := armcostmanagement.QueryDefinition{
		Type:      to.Ptr(armcostmanagement.ExportTypeActualCost),
		Timeframe: to.Ptr(armcostmanagement.TimeframeTypeMonthToDate),
		Dataset: &armcostmanagement.QueryDataset{
			Aggregation: map[string]*armcostmanagement.QueryAggregation{
				"totalCost": {Name: to.Ptr("Cost"), Function: to.Ptr(armcostmanagement.FunctionTypeSum)},
			},
		},
	}
	scope := "/subscriptions/" + id.SubscriptionID
	resp, err := client.Usage(ctx, scope, query, nil)
	for retry := 0; retry < costRetries; retry++ {
		wait, ok := throttled(err)
		if !ok {
			break
		}
		select {
		case <-ctx.Done():
			return Cost{}, ctx.Err()
		case <-time.After(wait + time.Second):
		}
		resp, err = client.Usage(ctx, scope, query, nil)
	}
	if err != nil {
		return Cost{}, fmt.Errorf("query month-to-date cost: %w", err)
	}
	if resp.Properties == nil || len(resp.Properties.Rows) == 0 {
		return Cost{}, nil
	}
	amountAt, currencyAt := -1, -1
	for i, column := range resp.Properties.Columns {
		switch {
		case column == nil || column.Name == nil:
		case *column.Name == "Cost":
			amountAt = i
		case *column.Name == "Currency":
			currencyAt = i
		}
	}
	if amountAt < 0 || currencyAt < 0 || len(resp.Properties.Rows) != 1 {
		return Cost{}, fmt.Errorf("cost response has unexpected columns or rows")
	}
	row := resp.Properties.Rows[0]
	amount, okAmount := row[amountAt].(float64)
	currency, okCurrency := row[currencyAt].(string)
	if !okAmount || !okCurrency {
		return Cost{}, fmt.Errorf("cost response has unexpected values")
	}
	return Cost{Amount: amount, Currency: currency}, nil
}

// throttled reports the wait announced by a 429 from Cost Management.
func throttled(err error) (time.Duration, bool) {
	var responseErr *azcore.ResponseError
	if !errors.As(err, &responseErr) || responseErr.StatusCode != http.StatusTooManyRequests || responseErr.RawResponse == nil {
		return 0, false
	}
	seconds, parseErr := strconv.Atoi(responseErr.RawResponse.Header.Get(costRetryAfter))
	if parseErr != nil || seconds < 0 {
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}
