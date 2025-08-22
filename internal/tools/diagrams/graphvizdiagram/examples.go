package graphvizdiagram

// getExamples returns example diagram definitions based on type
func (t *GraphvizDiagramTool) getExamples(diagramType string) (map[string]interface{}, error) {
	examples := make(map[string]interface{})

	// Always include basic patterns first
	examples["basic_patterns"] = t.getBasicPatterns()

	switch diagramType {
	case "aws":
		examples["aws"] = t.getAWSExamples()
	case "sequence":
		examples["sequence"] = t.getSequenceExamples()
	case "flow":
		examples["flow"] = t.getFlowExamples()
	case "class":
		examples["class"] = t.getClassExamples()
	case "k8s":
		examples["k8s"] = t.getKubernetesExamples()
	case "onprem":
		examples["onprem"] = t.getOnPremExamples()
	case "custom":
		examples["custom"] = t.getCustomExamples()
	case "all":
		examples["aws"] = t.getAWSExamples()
		examples["sequence"] = t.getSequenceExamples()
		examples["flow"] = t.getFlowExamples()
		examples["class"] = t.getClassExamples()
		examples["k8s"] = t.getKubernetesExamples()
		examples["onprem"] = t.getOnPremExamples()
		examples["custom"] = t.getCustomExamples()
	}

	return examples, nil
}

// getBasicPatterns returns fundamental diagram patterns
func (t *GraphvizDiagramTool) getBasicPatterns() map[string]interface{} {
	return map[string]interface{}{
		"description": "Fundamental patterns that all AI agents should learn first - use these JSON structures as templates",
		"simple_connection": map[string]interface{}{
			"description": "Basic two-node connection - simplest possible diagram",
			"json": map[string]interface{}{
				"name":      "Simple Connection",
				"direction": "LR",
				"nodes": []map[string]string{
					{"id": "server", "type": "aws.ec2", "label": "Web Server"},
					{"id": "database", "type": "aws.rds", "label": "Database"},
				},
				"connections": []map[string]string{
					{"from": "server", "to": "database"},
				},
			},
		},
		"chain_connection": map[string]interface{}{
			"description": "Multiple nodes in sequence - create a chain of connections",
			"json": map[string]interface{}{
				"name":      "Chain Connection",
				"direction": "LR",
				"nodes": []map[string]string{
					{"id": "lb", "type": "aws.elb", "label": "Load Balancer"},
					{"id": "web", "type": "aws.ec2", "label": "Web Server"},
					{"id": "db", "type": "aws.rds", "label": "Database"},
				},
				"connections": []map[string]string{
					{"from": "lb", "to": "web"},
					{"from": "web", "to": "db"},
				},
			},
		},
		"multiple_connections": map[string]interface{}{
			"description": "One node connecting to multiple targets - fan-out pattern",
			"json": map[string]interface{}{
				"name":      "Multiple Connections",
				"direction": "TB",
				"nodes": []map[string]string{
					{"id": "web", "type": "aws.ec2", "label": "Web Server"},
					{"id": "db", "type": "aws.rds", "label": "Database"},
					{"id": "cache", "type": "aws.elasticache", "label": "Cache"},
				},
				"connections": []map[string]string{
					{"from": "web", "to": "db"},
					{"from": "web", "to": "cache"},
				},
			},
		},
		"basic_cluster": map[string]interface{}{
			"description": "Grouping nodes in a cluster/subnet - use for logical grouping",
			"json": map[string]interface{}{
				"name":      "Basic Cluster",
				"direction": "LR",
				"nodes": []map[string]string{
					{"id": "web", "type": "aws.ec2", "label": "Web Server"},
					{"id": "db", "type": "aws.rds", "label": "Database"},
				},
				"connections": []map[string]string{
					{"from": "web", "to": "db"},
				},
				"clusters": []map[string]interface{}{
					{"name": "Production VPC", "nodes": []string{"web", "db"}},
				},
			},
		},
	}
}

// getAWSExamples returns AWS-specific diagram examples
func (t *GraphvizDiagramTool) getAWSExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "AWS architecture diagram examples from basic to advanced",
		"three_tier_basic": map[string]interface{}{
			"description": "Basic 3-tier web application",
			"complexity":  "basic",
			"json": map[string]interface{}{
				"name":      "3-Tier Web Application",
				"direction": "LR",
				"nodes": []map[string]string{
					{"id": "alb", "type": "aws.alb", "label": "Application Load Balancer"},
					{"id": "web", "type": "aws.ec2", "label": "Web Server"},
					{"id": "db", "type": "aws.rds", "label": "Database"},
				},
				"connections": []map[string]string{
					{"from": "alb", "to": "web"},
					{"from": "web", "to": "db"},
				},
			},
		},
		"three_tier_with_vpc": map[string]interface{}{
			"description": "3-tier application with VPC clustering",
			"complexity":  "intermediate",
			"json": map[string]interface{}{
				"name":      "3-Tier with VPC",
				"direction": "TB",
				"nodes": []map[string]string{
					{"id": "alb", "type": "aws.alb", "label": "Application LB"},
					{"id": "web", "type": "aws.ec2", "label": "Web Server"},
					{"id": "db", "type": "aws.rds", "label": "Database"},
				},
				"connections": []map[string]string{
					{"from": "alb", "to": "web"},
					{"from": "web", "to": "db"},
				},
				"clusters": []map[string]interface{}{
					{"name": "Production VPC", "nodes": []string{"alb", "web", "db"}},
				},
			},
		},
		"scalable_web_app": map[string]interface{}{
			"description": "Scalable web application with multiple instances",
			"complexity":  "advanced",
			"json": map[string]interface{}{
				"name":      "Scalable Web Application",
				"direction": "TB",
				"nodes": []map[string]string{
					{"id": "cloudfront", "type": "aws.cloudfront", "label": "CloudFront CDN"},
					{"id": "alb", "type": "aws.alb", "label": "Application LB"},
					{"id": "web1", "type": "aws.ec2", "label": "Web Server 1"},
					{"id": "web2", "type": "aws.ec2", "label": "Web Server 2"},
					{"id": "web3", "type": "aws.ec2", "label": "Web Server 3"},
					{"id": "rds_primary", "type": "aws.rds", "label": "Primary DB"},
					{"id": "rds_replica", "type": "aws.rds", "label": "Read Replica"},
					{"id": "elasticache", "type": "aws.elasticache", "label": "Redis Cache"},
				},
				"connections": []map[string]string{
					{"from": "cloudfront", "to": "alb"},
					{"from": "alb", "to": "web1"},
					{"from": "alb", "to": "web2"},
					{"from": "alb", "to": "web3"},
					{"from": "web1", "to": "rds_primary"},
					{"from": "web2", "to": "rds_primary"},
					{"from": "web3", "to": "rds_primary"},
					{"from": "web1", "to": "elasticache"},
					{"from": "web2", "to": "elasticache"},
					{"from": "web3", "to": "elasticache"},
					{"from": "rds_primary", "to": "rds_replica"},
				},
				"clusters": []map[string]interface{}{
					{"name": "Production VPC", "nodes": []string{"alb", "web1", "web2", "web3", "rds_primary", "rds_replica", "elasticache"}},
				},
			},
		},
		"serverless_architecture": map[string]interface{}{
			"description": "Serverless application with Lambda and API Gateway",
			"complexity":  "intermediate",
			"json": map[string]interface{}{
				"name":      "Serverless Architecture",
				"direction": "LR",
				"nodes": []map[string]string{
					{"id": "user", "type": "generic.user", "label": "User"},
					{"id": "api", "type": "aws.apigateway", "label": "API Gateway"},
					{"id": "lambda", "type": "aws.lambda", "label": "Lambda Function"},
					{"id": "dynamodb", "type": "aws.dynamodb", "label": "DynamoDB"},
					{"id": "s3", "type": "aws.s3", "label": "S3 Bucket"},
				},
				"connections": []map[string]string{
					{"from": "user", "to": "api"},
					{"from": "api", "to": "lambda"},
					{"from": "lambda", "to": "dynamodb"},
					{"from": "lambda", "to": "s3"},
				},
			},
		},
		"microservices": map[string]interface{}{
			"description": "Microservices architecture with service mesh",
			"complexity":  "advanced",
			"json": map[string]interface{}{
				"name":      "Microservices Architecture",
				"direction": "TB",
				"nodes": []map[string]string{
					{"id": "alb", "type": "aws.alb", "label": "Application LB"},
					{"id": "apigw", "type": "aws.apigateway", "label": "API Gateway"},
					{"id": "user_svc", "type": "aws.ecs", "label": "User Service"},
					{"id": "order_svc", "type": "aws.ecs", "label": "Order Service"},
					{"id": "payment_svc", "type": "aws.ecs", "label": "Payment Service"},
					{"id": "user_db", "type": "aws.rds", "label": "User DB"},
					{"id": "order_db", "type": "aws.rds", "label": "Order DB"},
					{"id": "payment_db", "type": "aws.rds", "label": "Payment DB"},
				},
				"connections": []map[string]string{
					{"from": "alb", "to": "apigw"},
					{"from": "apigw", "to": "user_svc"},
					{"from": "apigw", "to": "order_svc"},
					{"from": "apigw", "to": "payment_svc"},
					{"from": "user_svc", "to": "user_db"},
					{"from": "order_svc", "to": "order_db"},
					{"from": "payment_svc", "to": "payment_db"},
					{"from": "order_svc", "to": "payment_svc"},
				},
				"clusters": []map[string]interface{}{
					{"name": "Microservices VPC", "nodes": []string{"alb", "apigw", "user_svc", "order_svc", "payment_svc", "user_db", "order_db", "payment_db"}},
				},
			},
		},
	}
}

// getSequenceExamples returns sequence diagram examples
func (t *GraphvizDiagramTool) getSequenceExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "Process flow and sequence diagrams",
		"user_authentication": map[string]interface{}{
			"description": "User authentication flow",
			"json": map[string]interface{}{
				"name":      "Authentication Flow",
				"direction": "LR",
				"nodes": []map[string]string{
					{"id": "user", "type": "generic.user", "label": "User"},
					{"id": "frontend", "type": "generic.server", "label": "Frontend"},
					{"id": "auth", "type": "aws.cognito", "label": "Auth Service"},
					{"id": "backend", "type": "aws.lambda", "label": "Backend API"},
				},
				"connections": []map[string]string{
					{"from": "user", "to": "frontend"},
					{"from": "frontend", "to": "auth"},
					{"from": "auth", "to": "backend"},
					{"from": "backend", "to": "frontend"},
					{"from": "frontend", "to": "user"},
				},
			},
		},
		"api_request_flow": map[string]interface{}{
			"description": "API request processing flow",
			"json": map[string]interface{}{
				"name":      "API Request Flow",
				"direction": "TB",
				"nodes": []map[string]string{
					{"id": "client", "type": "generic.user", "label": "Client"},
					{"id": "gateway", "type": "aws.apigateway", "label": "API Gateway"},
					{"id": "lambda", "type": "aws.lambda", "label": "Lambda"},
					{"id": "database", "type": "aws.dynamodb", "label": "Database"},
					{"id": "cache", "type": "aws.elasticache", "label": "Cache"},
				},
				"connections": []map[string]string{
					{"from": "client", "to": "gateway"},
					{"from": "gateway", "to": "lambda"},
					{"from": "lambda", "to": "cache"},
					{"from": "lambda", "to": "database"},
					{"from": "database", "to": "lambda"},
					{"from": "lambda", "to": "gateway"},
					{"from": "gateway", "to": "client"},
				},
			},
		},
	}
}

// getFlowExamples returns flowchart examples
func (t *GraphvizDiagramTool) getFlowExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "Decision trees and workflow diagrams",
		"deployment_pipeline": map[string]interface{}{
			"description": "CI/CD deployment pipeline",
			"json": map[string]interface{}{
				"name":      "Deployment Pipeline",
				"direction": "TB",
				"nodes": []map[string]string{
					{"id": "code", "type": "generic.git", "label": "Code Commit"},
					{"id": "build", "type": "aws.codebuild", "label": "Build"},
					{"id": "test", "type": "generic.server", "label": "Test"},
					{"id": "deploy_staging", "type": "aws.ecs", "label": "Deploy Staging"},
					{"id": "deploy_prod", "type": "aws.ecs", "label": "Deploy Production"},
				},
				"connections": []map[string]string{
					{"from": "code", "to": "build"},
					{"from": "build", "to": "test"},
					{"from": "test", "to": "deploy_staging"},
					{"from": "deploy_staging", "to": "deploy_prod"},
				},
			},
		},
	}
}

// getClassExamples returns class diagram examples
func (t *GraphvizDiagramTool) getClassExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "Object relationships and inheritance diagrams",
		"simple_inheritance": map[string]interface{}{
			"description": "Simple class inheritance",
			"json": map[string]interface{}{
				"name":      "Class Inheritance",
				"direction": "TB",
				"nodes": []map[string]string{
					{"id": "base", "type": "generic.server", "label": "BaseService"},
					{"id": "web", "type": "aws.ec2", "label": "WebService"},
					{"id": "api", "type": "aws.lambda", "label": "APIService"},
				},
				"connections": []map[string]string{
					{"from": "base", "to": "web"},
					{"from": "base", "to": "api"},
				},
			},
		},
	}
}

// getKubernetesExamples returns Kubernetes diagram examples
func (t *GraphvizDiagramTool) getKubernetesExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "Kubernetes architecture diagrams",
		"simple_deployment": map[string]interface{}{
			"description": "Basic Kubernetes deployment",
			"json": map[string]interface{}{
				"name":      "K8s Deployment",
				"direction": "TB",
				"nodes": []map[string]string{
					{"id": "ingress", "type": "k8s.ingress", "label": "Ingress"},
					{"id": "service", "type": "k8s.service", "label": "Service"},
					{"id": "deployment", "type": "k8s.deployment", "label": "Deployment"},
					{"id": "pod1", "type": "k8s.pod", "label": "Pod 1"},
					{"id": "pod2", "type": "k8s.pod", "label": "Pod 2"},
					{"id": "pod3", "type": "k8s.pod", "label": "Pod 3"},
				},
				"connections": []map[string]string{
					{"from": "ingress", "to": "service"},
					{"from": "service", "to": "deployment"},
					{"from": "deployment", "to": "pod1"},
					{"from": "deployment", "to": "pod2"},
					{"from": "deployment", "to": "pod3"},
				},
				"clusters": []map[string]interface{}{
					{"name": "Kubernetes Cluster", "nodes": []string{"ingress", "service", "deployment", "pod1", "pod2", "pod3"}},
				},
			},
		},
	}
}

// getOnPremExamples returns on-premises diagram examples
func (t *GraphvizDiagramTool) getOnPremExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "On-premises infrastructure diagrams",
		"traditional_architecture": map[string]interface{}{
			"description": "Traditional 3-tier on-premises architecture",
			"json": map[string]interface{}{
				"name":      "On-Premises Architecture",
				"direction": "TB",
				"nodes": []map[string]string{
					{"id": "firewall", "type": "generic.firewall", "label": "Firewall"},
					{"id": "lb", "type": "generic.server", "label": "Load Balancer"},
					{"id": "app1", "type": "generic.server", "label": "App Server 1"},
					{"id": "app2", "type": "generic.server", "label": "App Server 2"},
					{"id": "primary_db", "type": "generic.database", "label": "Primary DB"},
					{"id": "backup_db", "type": "generic.database", "label": "Backup DB"},
				},
				"connections": []map[string]string{
					{"from": "firewall", "to": "lb"},
					{"from": "lb", "to": "app1"},
					{"from": "lb", "to": "app2"},
					{"from": "app1", "to": "primary_db"},
					{"from": "app2", "to": "primary_db"},
					{"from": "primary_db", "to": "backup_db"},
				},
				"clusters": []map[string]interface{}{
					{"name": "Data Center", "nodes": []string{"firewall", "lb", "app1", "app2", "primary_db", "backup_db"}},
				},
			},
		},
	}
}

// getCustomExamples returns custom diagram examples
func (t *GraphvizDiagramTool) getCustomExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "Custom diagrams with mixed providers",
		"hybrid_cloud": map[string]interface{}{
			"description": "Hybrid cloud architecture",
			"json": map[string]interface{}{
				"name":      "Hybrid Cloud Architecture",
				"direction": "TB",
				"nodes": []map[string]string{
					{"id": "legacy_app", "type": "generic.server", "label": "Legacy Application"},
					{"id": "local_db", "type": "generic.database", "label": "Local Database"},
					{"id": "api_gateway", "type": "aws.apigateway", "label": "API Gateway"},
					{"id": "lambda", "type": "aws.lambda", "label": "Lambda Functions"},
					{"id": "s3", "type": "aws.s3", "label": "S3 Storage"},
				},
				"connections": []map[string]string{
					{"from": "legacy_app", "to": "api_gateway"},
					{"from": "api_gateway", "to": "lambda"},
					{"from": "lambda", "to": "s3"},
					{"from": "lambda", "to": "local_db"},
				},
				"clusters": []map[string]interface{}{
					{"name": "On-Premises", "nodes": []string{"legacy_app", "local_db"}},
					{"name": "AWS Cloud", "nodes": []string{"api_gateway", "lambda", "s3"}},
				},
			},
		},
		"multi_cloud": map[string]interface{}{
			"description": "Multi-cloud architecture",
			"json": map[string]interface{}{
				"name":      "Multi-Cloud Architecture",
				"direction": "LR",
				"nodes": []map[string]string{
					{"id": "user", "type": "generic.user", "label": "User"},
					{"id": "aws_compute", "type": "aws.ec2", "label": "EC2 Instance"},
					{"id": "aws_storage", "type": "aws.s3", "label": "S3 Bucket"},
					{"id": "gcp_compute", "type": "gcp.compute", "label": "Compute Engine"},
					{"id": "gcp_storage", "type": "gcp.storage", "label": "Cloud Storage"},
				},
				"connections": []map[string]string{
					{"from": "user", "to": "aws_compute"},
					{"from": "user", "to": "gcp_compute"},
					{"from": "aws_compute", "to": "aws_storage"},
					{"from": "gcp_compute", "to": "gcp_storage"},
					{"from": "aws_compute", "to": "gcp_compute"},
				},
				"clusters": []map[string]interface{}{
					{"name": "AWS Region", "nodes": []string{"aws_compute", "aws_storage"}},
					{"name": "GCP Region", "nodes": []string{"gcp_compute", "gcp_storage"}},
				},
			},
		},
	}
}
