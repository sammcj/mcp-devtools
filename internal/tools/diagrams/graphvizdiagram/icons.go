package graphvizdiagram

import (
	"fmt"
)

// listIcons returns available icons filtered by provider and service
func (t *GraphvizDiagramTool) listIcons(provider, service string) (map[string]interface{}, error) {
	iconRegistry := t.getIconRegistry()

	// Apply filters
	if provider != "" {
		if providerIcons, exists := iconRegistry[provider]; exists {
			if service != "" {
				if serviceIcons, exists := providerIcons[service]; exists {
					return map[string]interface{}{
						provider: map[string]interface{}{
							service: serviceIcons,
						},
					}, nil
				} else {
					return map[string]interface{}{}, fmt.Errorf("service '%s' not found in provider '%s'", service, provider)
				}
			} else {
				return map[string]interface{}{
					provider: providerIcons,
				}, nil
			}
		} else {
			return map[string]interface{}{}, fmt.Errorf("provider '%s' not found", provider)
		}
	}

	// Return all icons if no filters
	result := make(map[string]interface{})
	for providerName, providerData := range iconRegistry {
		result[providerName] = providerData
	}

	return result, nil
}

// getIconRegistry returns the complete icon registry
func (t *GraphvizDiagramTool) getIconRegistry() map[string]map[string]map[string]interface{} {
	return map[string]map[string]map[string]interface{}{
		"aws":     t.getAWSIcons(),
		"gcp":     t.getGCPIcons(),
		"k8s":     t.getKubernetesIcons(),
		"generic": t.getGenericIcons(),
	}
}

// getAWSIcons returns AWS service icons organised by category
func (t *GraphvizDiagramTool) getAWSIcons() map[string]map[string]interface{} {
	return map[string]map[string]interface{}{
		"compute": {
			"description": "AWS compute services for processing and execution",
			"icons": map[string]interface{}{
				"aws.ec2": map[string]interface{}{
					"name":        "EC2",
					"full_name":   "Amazon Elastic Compute Cloud",
					"description": "Virtual servers in the cloud",
					"category":    "compute",
					"color":       "#FF9900",
					"shape":       "ellipse",
				},
				"aws.lambda": map[string]interface{}{
					"name":        "Lambda",
					"full_name":   "AWS Lambda",
					"description": "Serverless compute service",
					"category":    "compute",
					"color":       "#FF9900",
					"shape":       "hexagon",
				},
				"aws.ecs": map[string]interface{}{
					"name":        "ECS",
					"full_name":   "Amazon Elastic Container Service",
					"description": "Containerised application management",
					"category":    "compute",
					"color":       "#FF9900",
					"shape":       "box",
				},
				"aws.eks": map[string]interface{}{
					"name":        "EKS",
					"full_name":   "Amazon Elastic Kubernetes Service",
					"description": "Managed Kubernetes service",
					"category":    "compute",
					"color":       "#FF9900",
					"shape":       "hexagon",
				},
				"aws.batch": map[string]interface{}{
					"name":        "Batch",
					"full_name":   "AWS Batch",
					"description": "Batch computing service",
					"category":    "compute",
					"color":       "#FF9900",
					"shape":       "box",
				},
			},
		},
		"database": {
			"description": "AWS database and storage services",
			"icons": map[string]interface{}{
				"aws.rds": map[string]interface{}{
					"name":        "RDS",
					"full_name":   "Amazon Relational Database Service",
					"description": "Managed relational database",
					"category":    "database",
					"color":       "#3F48CC",
					"shape":       "cylinder",
				},
				"aws.dynamodb": map[string]interface{}{
					"name":        "DynamoDB",
					"full_name":   "Amazon DynamoDB",
					"description": "NoSQL database service",
					"category":    "database",
					"color":       "#3F48CC",
					"shape":       "cylinder",
				},
				"aws.redshift": map[string]interface{}{
					"name":        "Redshift",
					"full_name":   "Amazon Redshift",
					"description": "Data warehouse service",
					"category":    "database",
					"color":       "#3F48CC",
					"shape":       "cylinder",
				},
				"aws.aurora": map[string]interface{}{
					"name":        "Aurora",
					"full_name":   "Amazon Aurora",
					"description": "High-performance managed database",
					"category":    "database",
					"color":       "#3F48CC",
					"shape":       "cylinder",
				},
				"aws.elasticache": map[string]interface{}{
					"name":        "ElastiCache",
					"full_name":   "Amazon ElastiCache",
					"description": "In-memory caching service",
					"category":    "database",
					"color":       "#3F48CC",
					"shape":       "ellipse",
				},
			},
		},
		"network": {
			"description": "AWS networking and content delivery services",
			"icons": map[string]interface{}{
				"aws.vpc": map[string]interface{}{
					"name":        "VPC",
					"full_name":   "Amazon Virtual Private Cloud",
					"description": "Isolated cloud resources",
					"category":    "network",
					"color":       "#232F3E",
					"shape":       "box",
				},
				"aws.elb": map[string]interface{}{
					"name":        "ELB",
					"full_name":   "Elastic Load Balancer",
					"description": "Distribute incoming traffic",
					"category":    "network",
					"color":       "#FF9900",
					"shape":       "box",
				},
				"aws.alb": map[string]interface{}{
					"name":        "ALB",
					"full_name":   "Application Load Balancer",
					"description": "Layer 7 load balancer",
					"category":    "network",
					"color":       "#FF9900",
					"shape":       "box",
				},
				"aws.cloudfront": map[string]interface{}{
					"name":        "CloudFront",
					"full_name":   "Amazon CloudFront",
					"description": "Content delivery network",
					"category":    "network",
					"color":       "#FF9900",
					"shape":       "diamond",
				},
				"aws.apigateway": map[string]interface{}{
					"name":        "API Gateway",
					"full_name":   "Amazon API Gateway",
					"description": "Managed API service",
					"category":    "network",
					"color":       "#FF9900",
					"shape":       "hexagon",
				},
				"aws.route53": map[string]interface{}{
					"name":        "Route 53",
					"full_name":   "Amazon Route 53",
					"description": "DNS and domain registration",
					"category":    "network",
					"color":       "#FF9900",
					"shape":       "ellipse",
				},
			},
		},
		"storage": {
			"description": "AWS storage and backup services",
			"icons": map[string]interface{}{
				"aws.s3": map[string]interface{}{
					"name":        "S3",
					"full_name":   "Amazon Simple Storage Service",
					"description": "Object storage service",
					"category":    "storage",
					"color":       "#3F48CC",
					"shape":       "folder",
				},
				"aws.ebs": map[string]interface{}{
					"name":        "EBS",
					"full_name":   "Amazon Elastic Block Store",
					"description": "Block storage for EC2",
					"category":    "storage",
					"color":       "#3F48CC",
					"shape":       "cylinder",
				},
				"aws.efs": map[string]interface{}{
					"name":        "EFS",
					"full_name":   "Amazon Elastic File System",
					"description": "Managed file storage",
					"category":    "storage",
					"color":       "#3F48CC",
					"shape":       "folder",
				},
				"aws.glacier": map[string]interface{}{
					"name":        "Glacier",
					"full_name":   "Amazon S3 Glacier",
					"description": "Archive storage service",
					"category":    "storage",
					"color":       "#3F48CC",
					"shape":       "folder",
				},
			},
		},
		"security": {
			"description": "AWS security and identity services",
			"icons": map[string]interface{}{
				"aws.iam": map[string]interface{}{
					"name":        "IAM",
					"full_name":   "AWS Identity and Access Management",
					"description": "Identity and access control",
					"category":    "security",
					"color":       "#FF4B4B",
					"shape":       "diamond",
				},
				"aws.cognito": map[string]interface{}{
					"name":        "Cognito",
					"full_name":   "Amazon Cognito",
					"description": "User identity and authentication",
					"category":    "security",
					"color":       "#FF4B4B",
					"shape":       "diamond",
				},
				"aws.waf": map[string]interface{}{
					"name":        "WAF",
					"full_name":   "AWS Web Application Firewall",
					"description": "Web application firewall",
					"category":    "security",
					"color":       "#FF4B4B",
					"shape":       "diamond",
				},
				"aws.kms": map[string]interface{}{
					"name":        "KMS",
					"full_name":   "AWS Key Management Service",
					"description": "Encryption key management",
					"category":    "security",
					"color":       "#FF4B4B",
					"shape":       "diamond",
				},
			},
		},
		"analytics": {
			"description": "AWS analytics and machine learning services",
			"icons": map[string]interface{}{
				"aws.kinesis": map[string]interface{}{
					"name":        "Kinesis",
					"full_name":   "Amazon Kinesis",
					"description": "Real-time data streaming",
					"category":    "analytics",
					"color":       "#FF9900",
					"shape":       "ellipse",
				},
				"aws.emr": map[string]interface{}{
					"name":        "EMR",
					"full_name":   "Amazon EMR",
					"description": "Big data processing",
					"category":    "analytics",
					"color":       "#FF9900",
					"shape":       "hexagon",
				},
				"aws.sagemaker": map[string]interface{}{
					"name":        "SageMaker",
					"full_name":   "Amazon SageMaker",
					"description": "Machine learning platform",
					"category":    "analytics",
					"color":       "#FF9900",
					"shape":       "hexagon",
				},
				"aws.bedrock": map[string]interface{}{
					"name":        "Bedrock",
					"full_name":   "Amazon Bedrock",
					"description": "Generative AI service",
					"category":    "analytics",
					"color":       "#FF9900",
					"shape":       "hexagon",
				},
			},
		},
	}
}

// getGCPIcons returns Google Cloud Platform icons
func (t *GraphvizDiagramTool) getGCPIcons() map[string]map[string]interface{} {
	return map[string]map[string]interface{}{
		"compute": {
			"description": "Google Cloud compute services",
			"icons": map[string]interface{}{
				"gcp.compute": map[string]interface{}{
					"name":        "Compute Engine",
					"description": "Virtual machines on Google Cloud",
					"category":    "compute",
					"color":       "#4285F4",
					"shape":       "ellipse",
				},
				"gcp.functions": map[string]interface{}{
					"name":        "Cloud Functions",
					"description": "Serverless compute platform",
					"category":    "compute",
					"color":       "#4285F4",
					"shape":       "hexagon",
				},
				"gcp.run": map[string]interface{}{
					"name":        "Cloud Run",
					"description": "Containerised applications",
					"category":    "compute",
					"color":       "#4285F4",
					"shape":       "box",
				},
			},
		},
		"database": {
			"description": "Google Cloud database services",
			"icons": map[string]interface{}{
				"gcp.sql": map[string]interface{}{
					"name":        "Cloud SQL",
					"description": "Managed relational database",
					"category":    "database",
					"color":       "#EA4335",
					"shape":       "cylinder",
				},
				"gcp.firestore": map[string]interface{}{
					"name":        "Firestore",
					"description": "NoSQL document database",
					"category":    "database",
					"color":       "#EA4335",
					"shape":       "cylinder",
				},
			},
		},
		"storage": {
			"description": "Google Cloud storage services",
			"icons": map[string]interface{}{
				"gcp.storage": map[string]interface{}{
					"name":        "Cloud Storage",
					"description": "Object storage service",
					"category":    "storage",
					"color":       "#34A853",
					"shape":       "folder",
				},
			},
		},
	}
}

// getKubernetesIcons returns Kubernetes icons
func (t *GraphvizDiagramTool) getKubernetesIcons() map[string]map[string]interface{} {
	return map[string]map[string]interface{}{
		"workloads": {
			"description": "Kubernetes workload resources",
			"icons": map[string]interface{}{
				"k8s.pod": map[string]interface{}{
					"name":        "Pod",
					"description": "Smallest deployable unit",
					"category":    "workloads",
					"color":       "#326CE5",
					"shape":       "ellipse",
				},
				"k8s.deployment": map[string]interface{}{
					"name":        "Deployment",
					"description": "Declarative updates for pods",
					"category":    "workloads",
					"color":       "#326CE5",
					"shape":       "hexagon",
				},
				"k8s.service": map[string]interface{}{
					"name":        "Service",
					"description": "Expose pods as network service",
					"category":    "workloads",
					"color":       "#326CE5",
					"shape":       "box",
				},
				"k8s.ingress": map[string]interface{}{
					"name":        "Ingress",
					"description": "External access to services",
					"category":    "workloads",
					"color":       "#326CE5",
					"shape":       "diamond",
				},
			},
		},
		"storage": {
			"description": "Kubernetes storage resources",
			"icons": map[string]interface{}{
				"k8s.pv": map[string]interface{}{
					"name":        "Persistent Volume",
					"description": "Storage resource in cluster",
					"category":    "storage",
					"color":       "#326CE5",
					"shape":       "cylinder",
				},
				"k8s.pvc": map[string]interface{}{
					"name":        "Persistent Volume Claim",
					"description": "Request for storage",
					"category":    "storage",
					"color":       "#326CE5",
					"shape":       "folder",
				},
			},
		},
	}
}

// getGenericIcons returns generic/universal icons
func (t *GraphvizDiagramTool) getGenericIcons() map[string]map[string]interface{} {
	return map[string]map[string]interface{}{
		"infrastructure": {
			"description": "Generic infrastructure components",
			"icons": map[string]interface{}{
				"generic.server": map[string]interface{}{
					"name":        "Server",
					"description": "Generic server/compute resource",
					"category":    "infrastructure",
					"color":       "lightgray",
					"shape":       "box",
				},
				"generic.database": map[string]interface{}{
					"name":        "Database",
					"description": "Generic database",
					"category":    "infrastructure",
					"color":       "lightgray",
					"shape":       "cylinder",
				},
				"generic.firewall": map[string]interface{}{
					"name":        "Firewall",
					"description": "Network firewall",
					"category":    "infrastructure",
					"color":       "orange",
					"shape":       "diamond",
				},
				"generic.storage": map[string]interface{}{
					"name":        "Storage",
					"description": "Generic storage system",
					"category":    "infrastructure",
					"color":       "lightgray",
					"shape":       "folder",
				},
			},
		},
		"users": {
			"description": "Users and actors",
			"icons": map[string]interface{}{
				"user": map[string]interface{}{
					"name":        "User",
					"description": "End user or person",
					"category":    "users",
					"color":       "lightyellow",
					"shape":       "ellipse",
				},
				"admin": map[string]interface{}{
					"name":        "Administrator",
					"description": "System administrator",
					"category":    "users",
					"color":       "lightblue",
					"shape":       "ellipse",
				},
			},
		},
		"flow": {
			"description": "Flowchart elements",
			"icons": map[string]interface{}{
				"generic.start": map[string]interface{}{
					"name":        "Start",
					"description": "Flow start point",
					"category":    "flow",
					"color":       "lightgreen",
					"shape":       "ellipse",
				},
				"generic.stop": map[string]interface{}{
					"name":        "Stop",
					"description": "Flow end point",
					"category":    "flow",
					"color":       "lightcoral",
					"shape":       "ellipse",
				},
				"generic.decision": map[string]interface{}{
					"name":        "Decision",
					"description": "Decision point",
					"category":    "flow",
					"color":       "lightyellow",
					"shape":       "diamond",
				},
				"generic.process": map[string]interface{}{
					"name":        "Process",
					"description": "Process step",
					"category":    "flow",
					"color":       "lightblue",
					"shape":       "box",
				},
			},
		},
	}
}
