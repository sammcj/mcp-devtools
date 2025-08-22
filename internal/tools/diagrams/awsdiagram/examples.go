package awsdiagram

// getExamples returns example diagram definitions based on type
func (t *AWSDiagramTool) getExamples(diagramType string) (map[string]interface{}, error) {
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
func (t *AWSDiagramTool) getBasicPatterns() map[string]interface{} {
	return map[string]interface{}{
		"description": "Fundamental patterns that all AI agents should learn first",
		"simple_connection": map[string]interface{}{
			"description": "Basic two-node connection",
			"text_dsl": `diagram "Simple Connection" {
  node a = aws.ec2 "Server A"
  node b = aws.rds "Database B"
  
  a -> b
}`,
			"json_format": map[string]interface{}{
				"diagram": map[string]string{
					"name":      "Simple Connection",
					"direction": "LR",
				},
				"nodes": []map[string]string{
					{"id": "a", "type": "aws.ec2", "label": "Server A"},
					{"id": "b", "type": "aws.rds", "label": "Database B"},
				},
				"connections": []map[string]string{
					{"from": "a", "to": "b"},
				},
			},
		},
		"chain_connection": map[string]interface{}{
			"description": "Multiple nodes in sequence",
			"text_dsl": `diagram "Chain Connection" {
  node lb = aws.elb "Load Balancer"
  node web = aws.ec2 "Web Server"
  node db = aws.rds "Database"
  
  lb -> web -> db
}`,
		},
		"multiple_connections": map[string]interface{}{
			"description": "One node connecting to multiple targets",
			"text_dsl": `diagram "Multiple Connections" {
  node web = aws.ec2 "Web Server"
  node db = aws.rds "Database"
  node cache = aws.elasticache "Cache"
  
  web -> db
  web -> cache
}`,
		},
		"basic_cluster": map[string]interface{}{
			"description": "Grouping nodes in a cluster",
			"text_dsl": `diagram "Basic Cluster" {
  cluster vpc "Production VPC" {
    node web = aws.ec2 "Web Server"
    node db = aws.rds "Database"
  }
  
  web -> db
}`,
		},
	}
}

// getAWSExamples returns AWS-specific diagram examples
func (t *AWSDiagramTool) getAWSExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "AWS architecture diagram examples from basic to advanced",
		"three_tier_basic": map[string]interface{}{
			"description": "Basic 3-tier web application",
			"complexity":  "basic",
			"text_dsl": `diagram "3-Tier Web Application" {
  node alb = aws.alb "Application Load Balancer"
  node web = aws.ec2 "Web Server"
  node db = aws.rds "Database"
  
  alb -> web -> db
}`,
		},
		"three_tier_with_vpc": map[string]interface{}{
			"description": "3-tier application with VPC clustering",
			"complexity":  "intermediate",
			"text_dsl": `diagram "3-Tier with VPC" direction=TB {
  cluster vpc "Production VPC" {
    cluster public "Public Subnet" {
      node alb = aws.alb "Application LB"
    }
    
    cluster private "Private Subnet" {
      node web = aws.ec2 "Web Server"
      node db = aws.rds "Database"
    }
  }
  
  alb -> web -> db
}`,
		},
		"scalable_web_app": map[string]interface{}{
			"description": "Scalable web application with multiple instances",
			"complexity":  "advanced",
			"text_dsl": `diagram "Scalable Web Application" direction=TB {
  node cloudfront = aws.cloudfront "CloudFront CDN"
  
  cluster vpc "Production VPC" {
    cluster public "Public Subnet" {
      node alb = aws.alb "Application LB"
    }
    
    cluster private_web "Web Tier" {
      node web1 = aws.ec2 "Web Server 1"
      node web2 = aws.ec2 "Web Server 2"
      node web3 = aws.ec2 "Web Server 3"
    }
    
    cluster private_data "Data Tier" {
      node rds_primary = aws.rds "Primary DB"
      node rds_replica = aws.rds "Read Replica"
      node elasticache = aws.elasticache "Redis Cache"
    }
  }
  
  cloudfront -> alb
  alb -> [web1, web2, web3]
  [web1, web2, web3] -> rds_primary
  [web1, web2, web3] -> elasticache
  rds_primary -> rds_replica
}`,
		},
		"serverless_architecture": map[string]interface{}{
			"description": "Serverless application with Lambda and API Gateway",
			"complexity":  "intermediate",
			"text_dsl": `diagram "Serverless Architecture" {
  node user = user "User"
  node api = aws.apigateway "API Gateway"
  node lambda = aws.lambda "Lambda Function"
  node dynamodb = aws.dynamodb "DynamoDB"
  node s3 = aws.s3 "S3 Bucket"
  
  user -> api -> lambda
  lambda -> dynamodb
  lambda -> s3
}`,
		},
		"microservices": map[string]interface{}{
			"description": "Microservices architecture with service mesh",
			"complexity":  "advanced",
			"text_dsl": `diagram "Microservices Architecture" direction=TB {
  cluster vpc "Microservices VPC" {
    cluster ingress "Ingress Layer" {
      node alb = aws.alb "Application LB"
      node apigw = aws.apigateway "API Gateway"
    }
    
    cluster services "Services Layer" {
      node user_svc = aws.ecs "User Service"
      node order_svc = aws.ecs "Order Service"
      node payment_svc = aws.ecs "Payment Service"
    }
    
    cluster data "Data Layer" {
      node user_db = aws.rds "User DB"
      node order_db = aws.rds "Order DB"
      node payment_db = aws.rds "Payment DB"
    }
  }
  
  alb -> apigw
  apigw -> [user_svc, order_svc, payment_svc]
  user_svc -> user_db
  order_svc -> order_db
  payment_svc -> payment_db
  order_svc -> payment_svc
}`,
		},
	}
}

// getSequenceExamples returns sequence diagram examples
func (t *AWSDiagramTool) getSequenceExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "Process flow and sequence diagrams",
		"user_authentication": map[string]interface{}{
			"description": "User authentication flow",
			"text_dsl": `diagram "Authentication Flow" direction=LR {
  node user = user "User"
  node frontend = generic.server "Frontend"
  node auth = aws.cognito "Auth Service"
  node backend = aws.lambda "Backend API"
  
  user -> frontend
  frontend -> auth
  auth -> backend
  backend -> frontend
  frontend -> user
}`,
		},
		"api_request_flow": map[string]interface{}{
			"description": "API request processing flow",
			"text_dsl": `diagram "API Request Flow" direction=TB {
  node client = user "Client"
  node gateway = aws.apigateway "API Gateway"
  node lambda = aws.lambda "Lambda"
  node database = aws.dynamodb "Database"
  node cache = aws.elasticache "Cache"
  
  client -> gateway
  gateway -> lambda
  lambda -> cache
  lambda -> database
  database -> lambda
  lambda -> gateway
  gateway -> client
}`,
		},
	}
}

// getFlowExamples returns flowchart examples
func (t *AWSDiagramTool) getFlowExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "Decision trees and workflow diagrams",
		"deployment_pipeline": map[string]interface{}{
			"description": "CI/CD deployment pipeline",
			"text_dsl": `diagram "Deployment Pipeline" direction=TB {
  node start = generic.start "Start"
  node code = generic.server "Code Commit"
  node build = aws.codebuild "Build"
  node test = generic.server "Test"
  node deploy_staging = aws.ecs "Deploy Staging"
  node deploy_prod = aws.ecs "Deploy Production"
  node end = generic.stop "End"
  
  start -> code -> build -> test
  test -> deploy_staging -> deploy_prod -> end
}`,
		},
	}
}

// getClassExamples returns class diagram examples
func (t *AWSDiagramTool) getClassExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "Object relationships and inheritance diagrams",
		"simple_inheritance": map[string]interface{}{
			"description": "Simple class inheritance",
			"text_dsl": `diagram "Class Inheritance" {
  node base = generic.server "BaseService"
  node web = aws.ec2 "WebService"
  node api = aws.lambda "APIService"
  
  base -> web
  base -> api
}`,
		},
	}
}

// getKubernetesExamples returns Kubernetes diagram examples
func (t *AWSDiagramTool) getKubernetesExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "Kubernetes architecture diagrams",
		"simple_deployment": map[string]interface{}{
			"description": "Basic Kubernetes deployment",
			"text_dsl": `diagram "K8s Deployment" direction=TB {
  cluster k8s_cluster "Kubernetes Cluster" {
    node ingress = k8s.ingress "Ingress"
    node service = k8s.service "Service"
    node deployment = k8s.deployment "Deployment"
    
    cluster pods "Pods" {
      node pod1 = k8s.pod "Pod 1"
      node pod2 = k8s.pod "Pod 2"
      node pod3 = k8s.pod "Pod 3"
    }
  }
  
  ingress -> service -> deployment
  deployment -> [pod1, pod2, pod3]
}`,
		},
	}
}

// getOnPremExamples returns on-premises diagram examples
func (t *AWSDiagramTool) getOnPremExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "On-premises infrastructure diagrams",
		"traditional_architecture": map[string]interface{}{
			"description": "Traditional 3-tier on-premises architecture",
			"text_dsl": `diagram "On-Premises Architecture" direction=TB {
  cluster datacenter "Data Center" {
    cluster dmz "DMZ" {
      node firewall = generic.firewall "Firewall"
      node lb = generic.server "Load Balancer"
    }
    
    cluster app_tier "Application Tier" {
      node app1 = generic.server "App Server 1"
      node app2 = generic.server "App Server 2"
    }
    
    cluster db_tier "Database Tier" {
      node primary_db = generic.database "Primary DB"
      node backup_db = generic.database "Backup DB"
    }
  }
  
  firewall -> lb
  lb -> [app1, app2]
  [app1, app2] -> primary_db
  primary_db -> backup_db
}`,
		},
	}
}

// getCustomExamples returns custom diagram examples
func (t *AWSDiagramTool) getCustomExamples() map[string]interface{} {
	return map[string]interface{}{
		"description": "Custom diagrams with mixed providers",
		"hybrid_cloud": map[string]interface{}{
			"description": "Hybrid cloud architecture",
			"text_dsl": `diagram "Hybrid Cloud Architecture" direction=TB {
  cluster onprem "On-Premises" {
    node legacy_app = generic.server "Legacy Application"
    node local_db = generic.database "Local Database"
  }
  
  cluster aws_cloud "AWS Cloud" {
    node api_gateway = aws.apigateway "API Gateway"
    node lambda = aws.lambda "Lambda Functions"
    node s3 = aws.s3 "S3 Storage"
  }
  
  legacy_app -> api_gateway
  api_gateway -> lambda
  lambda -> s3
  lambda -> local_db
}`,
		},
		"multi_cloud": map[string]interface{}{
			"description": "Multi-cloud architecture",
			"text_dsl": `diagram "Multi-Cloud Architecture" {
  cluster aws_region "AWS Region" {
    node aws_compute = aws.ec2 "EC2 Instance"
    node aws_storage = aws.s3 "S3 Bucket"
  }
  
  cluster gcp_region "GCP Region" {
    node gcp_compute = gcp.compute "Compute Engine"
    node gcp_storage = gcp.storage "Cloud Storage"
  }
  
  node user = user "User"
  
  user -> aws_compute
  user -> gcp_compute
  aws_compute -> aws_storage
  gcp_compute -> gcp_storage
  aws_compute -> gcp_compute
}`,
		},
	}
}
