//go:build !e2e

package foundry

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/costmanagement/armcostmanagement/v3"
)

// Cost sums the subscription's month-to-date actual cost with Azure Cost Management.
// A subscription without usage returns no rows, which is a zero cost of unknown currency.
func (s *azureSource) Cost(ctx context.Context, foundry Foundry) (Cost, error) {
	id, err := arm.ParseResourceID(foundry.ID)
	if err != nil {
		return Cost{}, err
	}
	client, err := armcostmanagement.NewQueryClient(s.credential, nil)
	if err != nil {
		return Cost{}, err
	}
	resp, err := client.Usage(ctx, "/subscriptions/"+id.SubscriptionID, armcostmanagement.QueryDefinition{
		Type:      to.Ptr(armcostmanagement.ExportTypeActualCost),
		Timeframe: to.Ptr(armcostmanagement.TimeframeTypeMonthToDate),
		Dataset: &armcostmanagement.QueryDataset{
			Aggregation: map[string]*armcostmanagement.QueryAggregation{
				"totalCost": {Name: to.Ptr("Cost"), Function: to.Ptr(armcostmanagement.FunctionTypeSum)},
			},
		},
	}, nil)
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
