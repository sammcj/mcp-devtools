package pricing

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/sirupsen/logrus"
)

// Client wraps the AWS Pricing API client
type Client struct {
	client *pricing.Client
	logger *logrus.Logger
}

// NewClient creates a new AWS Pricing API client
// Returns error if AWS credentials are not available
func NewClient(ctx context.Context, logger *logrus.Logger) (*Client, error) {
	// Load AWS configuration from environment/credentials
	// Use us-east-1 as Pricing API is only available in limited regions
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Verify credentials are available
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("AWS credentials not available: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"region":    cfg.Region,
		"has_token": creds.SessionToken != "",
	}).Debug("AWS Pricing client initialised (using us-east-1 for Pricing API)")

	return &Client{
		client: pricing.NewFromConfig(cfg),
		logger: logger,
	}, nil
}

// DescribeServices lists all available AWS services with pricing
func (c *Client) DescribeServices(ctx context.Context) ([]types.Service, error) {
	c.logger.Debug("Fetching AWS services with pricing")

	var allServices []types.Service
	params := &pricing.DescribeServicesInput{
		MaxResults: aws.Int32(100),
	}

	// Paginate through all services
	for {
		resp, err := c.client.DescribeServices(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("failed to describe services: %w", err)
		}

		allServices = append(allServices, resp.Services...)

		if resp.NextToken == nil {
			break
		}
		params.NextToken = resp.NextToken
	}

	c.logger.WithField("service_count", len(allServices)).Debug("Services fetched")
	return allServices, nil
}

// GetProducts fetches pricing products for a specific service with filters
func (c *Client) GetProducts(ctx context.Context, serviceCode string, filters []types.Filter, maxResults int32) ([]string, error) {
	c.logger.WithFields(logrus.Fields{
		"service": serviceCode,
		"filters": len(filters),
	}).Debug("Fetching service pricing")

	if maxResults <= 0 {
		maxResults = 10
	}

	params := &pricing.GetProductsInput{
		ServiceCode: aws.String(serviceCode),
		Filters:     filters,
		MaxResults:  aws.Int32(maxResults),
	}

	resp, err := c.client.GetProducts(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get products: %w", err)
	}

	c.logger.WithField("product_count", len(resp.PriceList)).Debug("Products fetched")
	return resp.PriceList, nil
}

// GetServiceAttributes fetches available filter attributes for a service
func (c *Client) GetServiceAttributes(ctx context.Context, serviceCode string) ([]string, error) {
	c.logger.WithField("service", serviceCode).Debug("Fetching service attributes")

	resp, err := c.client.DescribeServices(ctx, &pricing.DescribeServicesInput{
		ServiceCode: aws.String(serviceCode),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe service: %w", err)
	}

	if len(resp.Services) == 0 {
		return nil, fmt.Errorf("service %s not found", serviceCode)
	}

	attributes := resp.Services[0].AttributeNames
	c.logger.WithField("attribute_count", len(attributes)).Debug("Attributes fetched")
	return attributes, nil
}
