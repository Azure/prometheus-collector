package main

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics"
)

func main() {
	// The regional endpoint to use. The region should match the region of the requested resources.
	// For global resources, the region should be 'global'
	endpoint := "https://eastus.metrics.monitor.azure.com"

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		fmt.Errorf("failed to get default credential: %v\n", err)
		return
	}

	client, err := azmetrics.NewClient(endpoint, cred, nil)
	if err != nil {
		fmt.Errorf("failed to create metrics client: %v\n", err)
		return
	}

	// This sample uses the Client to retrieve the "Ingress"
	// metric along with the "Average" aggregation type for multiple resources.
	// The query will execute over a timespan of 2 hours with a interval (granularity) of 5 minutes.
	resourceURI := "/subscriptions/b9842c7c-1a38-4385-8f39-a51314758bcf/resourceGroups/grace-addon/providers/microsoft.monitor/accounts/grace-addon"
	subscriptionID := "b9842c7c-1a38-4385-8f39-a51314758bcf"

	res, err := client.QueryResources(
		context.Background(),
		subscriptionID,
		"microsoft.monitor/accounts",
		[]string{"Ingress"},
		azmetrics.ResourceIDList{ResourceIDs: []string{resourceURI}},
		&azmetrics.QueryResourcesOptions{
			Aggregation: to.Ptr("average"),
			StartTime:   to.Ptr("2023-11-15"),
			EndTime:     to.Ptr("2023-11-16"),
			Interval:    to.Ptr("PT5M"),
		},
	)
	if err != nil {
		fmt.Errorf("failed to query resources: %v\n", err)
		return
	}

	// Print out results
	for _, result := range res.Values {
		for _, metric := range result.Values {
			fmt.Println(*metric.Name.Value + ": " + *metric.DisplayDescription)
			for _, timeSeriesElement := range metric.TimeSeries {
				for _, metricValue := range timeSeriesElement.Data {
					fmt.Printf("The ingress at %v is %v.\n", metricValue.TimeStamp.String(), *metricValue.Average)
				}
			}
		}
	}
}
